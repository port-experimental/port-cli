package auth

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cli/browser"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func ReadTokenFromStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("Failed reading token from stdin", err)
	}

	if stat.Mode()&os.ModeNamedPipe == 0 && stat.Size() == 0 {
		return "", fmt.Errorf("No token provided")
	}
	reader := bufio.NewReader(os.Stdin)
	var b strings.Builder
	for {
		r, _, rErr := reader.ReadRune()
		if rErr != nil && rErr == io.EOF {
			break
		}
		_, rErr = b.WriteRune(r)
		if rErr != nil {
			return "", fmt.Errorf("Error getting input:", rErr)
		}
	}

	return b.String(), nil
}

type LoginOpts struct {
	BaseURL string
	APIURL  string
}

func TokenFromOAuth(ctx context.Context, opts LoginOpts) (*jwt.Token, error) {
	obtainedToken := make(chan *oauth2.Token)
	conf := &oauth2.Config{
		ClientID:    "bY90kSHEuHEmQy6vtABmoQITeH4N6SFA",
		RedirectURL: "http://localhost:4321/oauth/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/authorize?audience=%s", opts.BaseURL, opts.APIURL),
			TokenURL: fmt.Sprintf("%s/oauth/token", opts.BaseURL),
		},
	}

	verifier := oauth2.GenerateVerifier()

	handler := func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			w.Write(bytes.NewBufferString("There was an error authenticating, please try again.").Bytes())
			w.WriteHeader(400)
			obtainedToken <- nil
			return
		}

		token, err := conf.Exchange(
			ctx,
			code,
			oauth2.VerifierOption(verifier),
		)
		if err != nil {
			w.Write(bytes.NewBufferString(fmt.Sprintf("Internal error. %v\n", err)).Bytes())
			w.WriteHeader(500)
			obtainedToken <- nil
			return
		}

		obtainedToken <- token

		w.Header().Set("Content-Type", "text/html")
		w.Write(bytes.NewBufferString("Logged in successfuly. You can now close this window.").Bytes())
	}
	server := &http.Server{Addr: ":4321"}
	http.HandleFunc("/oauth/callback", handler)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	fmt.Println("Opening a browser to log you in...")

	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
	err := browser.OpenURL(url)
	if err != nil {
		return nil, fmt.Errorf("Failed opening a browser")
	}

	token := <-obtainedToken
	if err := server.Shutdown(ctx); err != nil {
		return nil, fmt.Errorf("Unexpected error", err)
	}

	if token == nil {
		return nil, fmt.Errorf("Failed logging in.")
	}

	parsed, err := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name})).
		Parse(token.AccessToken, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(""), nil
		})
	return parsed, nil
}

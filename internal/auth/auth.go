package auth

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/cli/browser"
	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/styles"
	"golang.org/x/oauth2"
)

func ReadTokenFromStdin() (string, error) {
	return ReadToken(os.Stdin)
}

func ReadToken(f fs.File) (string, error) {
	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("failed reading token (%w)", err)
	}

	if stat.Mode()&os.ModeNamedPipe == 0 && stat.Size() == 0 {
		return "", fmt.Errorf("no token provided")
	}
	reader := bufio.NewReader(f)
	var b strings.Builder
	for {
		r, _, rErr := reader.ReadRune()
		if rErr != nil && rErr == io.EOF {
			break
		}
		_, rErr = b.WriteRune(r)
		if rErr != nil {
			return "", fmt.Errorf("error getting input (%w)", rErr)
		}
	}

	return b.String(), nil
}

type LoginOpts struct {
	BaseURL string
	APIURL  string
	Org     string
}

var clientIds = map[string]string{
	"https://auth.getport.io":         "DEcppuFTwCgBDGxgD2sOyJ0xOQx3p2OP",
	"https://auth.us.getport.io":      "OWZg1272IgNmjz7PPYP9bk7K3pzZkIeM",
	"https://auth.staging.getport.io": "bY90kSHEuHEmQy6vtABmoQITeH4N6SFA",
}

func TokenFromOAuth(ctx context.Context, opts LoginOpts) (*Token, error) {
	obtainedToken := make(chan *oauth2.Token)

	clientId, ok := clientIds[opts.BaseURL]
	if !ok {
		return nil, fmt.Errorf("base url %s is not supported", opts.BaseURL)
	}

	conf := &oauth2.Config{
		ClientID:    clientId,
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
			w.WriteHeader(http.StatusBadRequest)
			w.Write(bytes.NewBufferString("There was an error authenticating, please try again.").Bytes())
			obtainedToken <- nil
			return
		}

		token, err := conf.Exchange(
			ctx,
			code,
			oauth2.VerifierOption(verifier),
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(bytes.NewBufferString(fmt.Sprintf("Internal error. %v\n", err)).Bytes())
			obtainedToken <- nil
			return
		}

		obtainedToken <- token

		w.Header().Set("Content-Type", "text/html")
		w.Write(bytes.NewBufferString("Logged in successfully. You can now close this window.").Bytes())
	}
	server := &http.Server{Addr: ":4321"}
	http.HandleFunc("/oauth/callback", handler)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	lipgloss.Printf("Opening a browser to log you into %s...\n", styles.Bold.Render(opts.Org))

	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
	err := browser.OpenURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed opening a browser")
	}

	token := <-obtainedToken
	if err := server.Shutdown(ctx); err != nil {
		return nil, fmt.Errorf("unexpected error (%w)", err)
	}

	if token == nil {
		return nil, fmt.Errorf("failed logging in")
	}

	return ParseToken(token.AccessToken)
}

type Claims struct {
	Audience string    `json:"aud"`
	OrgName  string    `json:"orgName"`
	OrgId    string    `json:"orgId"`
	Email    string    `json:"email"`
	Expiry   time.Time `json:"exp"`
}
type Token struct {
	Token  string
	Claims Claims
}

func ParseToken(token string) (*Token, error) {
	claims := jwt.MapClaims{}
	t, _, err := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()})).ParseUnverified(token, &claims)
	if err != nil {
		return nil, err
	}
	aud, err := t.Claims.GetAudience()
	if err != nil {
		return nil, err
	}
	if len(aud) == 0 {
		return nil, fmt.Errorf("missing audience in token")
	}

	emailKey := fmt.Sprintf("%s/email", aud[0])
	email, found := claims[emailKey]
	if !found {
		return nil, fmt.Errorf("failed finding email claim")
	}
	if _, ok := email.(string); !ok {
		return nil, fmt.Errorf("email claim is not a string")
	}

	orgIdKey := fmt.Sprintf("%s/orgId", aud[0])
	orgId, found := claims[orgIdKey]
	if !found {
		return nil, fmt.Errorf("failed finding orgId claim")
	}
	if _, ok := orgId.(string); !ok {
		return nil, fmt.Errorf("orgId claim is not a string")
	}

	orgNameKey := fmt.Sprintf("%s/orgName", aud[0])
	orgName, found := claims[orgNameKey]
	if !found {
		return nil, fmt.Errorf("failed finding orgName claim")
	}
	if _, ok := orgName.(string); !ok {
		return nil, fmt.Errorf("orgName claim is not a string")
	}

	exp, found := claims["exp"]
	if !found {
		return nil, fmt.Errorf("failed finding exp claim")
	}
	if _, ok := exp.(float64); !ok {
		return nil, fmt.Errorf("exp claim is not a float64")
	}
	expiry := int64(exp.(float64))

	return &Token{
		Token: t.Raw,
		Claims: Claims{
			Audience: aud[0],
			Email:    email.(string),
			OrgId:    orgId.(string),
			OrgName:  orgName.(string),
			Expiry:   time.Unix(expiry, 0),
		},
	}, err
}

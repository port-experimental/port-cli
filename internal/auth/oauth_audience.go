package auth

import (
	"net/url"
	"strings"
)

// OAuthAudienceForAPIURL returns the API base URL used as the OAuth `audience` when logging in.
// Local port-api validates JWTs against PUBLIC_MAIN_API_URL (typically http://api.localhost:9080 via Traefik),
// which may differ from the direct port-api URL (http://localhost:3000/v1) used in config.
func OAuthAudienceForAPIURL(apiURL string) string {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(apiURL, "/")
	u, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimSuffix(trimmed, "/v1")
	}
	host := u.Hostname()
	port := u.Port()
	if host == "localhost" || host == "127.0.0.1" {
		if port == "3000" || port == "" {
			return "http://api.localhost:9080"
		}
	}
	return strings.TrimSuffix(trimmed, "/v1")
}

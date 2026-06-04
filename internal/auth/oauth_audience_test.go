package auth

import "testing"

func TestOAuthAudienceForAPIURL(t *testing.T) {
	tests := []struct {
		api  string
		want string
	}{
		{"https://api.getport.io/v1", "https://api.getport.io"},
		{"http://localhost:3000/v1", "http://api.localhost:9080"},
		{"http://localhost:3000", "http://api.localhost:9080"},
		{"http://api.localhost:9080/v1", "http://api.localhost:9080"},
	}
	for _, tt := range tests {
		if got := OAuthAudienceForAPIURL(tt.api); got != tt.want {
			t.Errorf("OAuthAudienceForAPIURL(%q) = %q, want %q", tt.api, got, tt.want)
		}
	}
}

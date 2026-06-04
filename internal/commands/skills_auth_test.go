package commands

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
)

func TestResolveSkillsAuthPrefersOAuthOverClientCredentials(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	manager := config.NewConfigManager(configPath)

	if err := manager.Write(&config.Config{
		DefaultOrg: "local",
		Organizations: map[string]config.OrganizationConfig{
			"local": {
				ClientID:     "machine-client-id",
				ClientSecret: "machine-client-secret",
				APIURL:       "https://api.getport.io/v1",
			},
		},
	}); err != nil {
		t.Fatalf("Write config: %v", err)
	}

	oauthJWT := testOAuthJWT(t, "https://api.getport.io", time.Now().Add(time.Hour))
	oauthToken, err := auth.ParseToken(oauthJWT)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if err := manager.StoreToken("local", oauthToken); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	token, orgConfig, _, err := resolveSkillsAuth(context.Background(), GlobalFlags{}, manager, "local")
	if err != nil {
		t.Fatalf("resolveSkillsAuth: %v", err)
	}
	if token.Token != oauthJWT {
		t.Fatal("expected stored OAuth token, got client-credentials token")
	}
	if token.Claims.IsMachine {
		t.Fatal("expected user OAuth token, not machine")
	}
	if orgConfig.APIURL == "" {
		t.Fatal("expected org api_url from config")
	}
	_ = orgConfig
}

func testOAuthJWT(t *testing.T, audience string, expiry time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                       audience,
		"exp":                       float64(expiry.Unix()),
		audience + "/email":         "user@test.com",
		audience + "/orgId":         "org_test",
		audience + "/orgName":       "Test",
		audience + "/port_user_id":  "auth0|user",
	})
	ss, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	return ss
}

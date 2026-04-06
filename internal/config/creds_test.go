package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/auth"
)

func configTestJWT(t *testing.T, audience string, expiry time.Time) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                 audience,
		"exp":                 float64(expiry.Unix()),
		audience + "/email":   "user@test.com",
		audience + "/orgId":   "someOrgId",
		audience + "/orgName": "Org Name",
	})
	ss, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	return ss
}

func TestGetOrRefreshTokenReturnsValidToken(t *testing.T) {
	dir := t.TempDir()
	manager := NewConfigManager(filepath.Join(dir, "config.yaml"))

	token, err := auth.ParseToken(configTestJWT(t, "https://api.example.com", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if err := manager.StoreToken("test-org", token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	got, err := manager.GetOrRefreshToken(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("GetOrRefreshToken failed: %v", err)
	}
	if got == nil || got.Token != token.Token {
		t.Fatalf("expected unchanged valid token")
	}
}

func TestGetOrRefreshTokenReturnsExpiredTokenWithoutRefreshMetadata(t *testing.T) {
	dir := t.TempDir()
	manager := NewConfigManager(filepath.Join(dir, "config.yaml"))

	token, err := auth.ParseToken(configTestJWT(t, "https://api.example.com", time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if err := manager.StoreToken("test-org", token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	got, err := manager.GetOrRefreshToken(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("GetOrRefreshToken failed: %v", err)
	}
	if got == nil || got.Token != token.Token {
		t.Fatalf("expected original expired token without refresh metadata")
	}
}

func TestGetOrRefreshTokenRefreshesAndPersists(t *testing.T) {
	dir := t.TempDir()
	manager := NewConfigManager(filepath.Join(dir, "config.yaml"))

	audience := "https://api.example.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  configTestJWT(t, audience, time.Now().Add(time.Hour)),
			"refresh_token": "rotated-refresh-token",
		})
	}))
	defer server.Close()

	auth.RegisterClientID(server.URL, "test-client-id")
	defer auth.UnregisterClientID(server.URL)

	token, err := auth.ParseToken(configTestJWT(t, audience, time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	token.RefreshToken = "old-refresh-token"
	token.AuthBaseURL = server.URL
	if err := manager.StoreToken("test-org", token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	got, err := manager.GetOrRefreshToken(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("GetOrRefreshToken failed: %v", err)
	}
	if got == nil || got.Token == token.Token {
		t.Fatalf("expected refreshed access token")
	}
	if got.RefreshToken != "rotated-refresh-token" {
		t.Fatalf("expected rotated refresh token, got %q", got.RefreshToken)
	}

	stored, err := manager.GetToken("test-org")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if stored.Token != got.Token {
		t.Fatalf("expected refreshed token to be persisted")
	}
}

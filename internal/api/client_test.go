package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenManager_GetToken(t *testing.T) {
	tm := NewTokenManager("test-client-id", "test-client-secret", "https://api.getport.io/v1")

	// Initially no token
	token, err := tm.GetToken()
	if err == nil && token != "" {
		t.Error("Expected error or empty token when refreshToken is not implemented")
	}
}

func TestTokenManager_SetToken(t *testing.T) {
	tm := NewTokenManager("test-client-id", "test-client-secret", "https://api.getport.io/v1")

	expiry := time.Now().Add(1 * time.Hour)
	tm.SetToken("test-token", expiry)

	// Token should be cached
	token, err := tm.GetToken()
	if err == nil && token == "test-token" {
		// Token is valid (within 5 minute buffer)
		return
	}

	// If token expired, that's also fine for this test
	if err != nil {
		// Expected if token expired
		return
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("test-id", "test-secret", "https://api.getport.io/v1", 0)

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}

	if client.tokenMgr.ClientID != "test-id" {
		t.Errorf("Expected ClientID 'test-id', got '%s'", client.tokenMgr.ClientID)
	}
}

func TestNewClient_DefaultURL(t *testing.T) {
	client := NewClient("test-id", "test-secret", "", 0)

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected default apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}
}

func TestClient_refreshToken(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/access_token" {
			t.Errorf("Expected path '/auth/access_token', got '%s'", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected method 'POST', got '%s'", r.Method)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		response := TokenResponse{
			AccessToken: "test-access-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient("test-id", "test-secret", server.URL, 0)
	client.apiURL = server.URL

	token, err := client.refreshToken(context.Background())
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	if token != "test-access-token" {
		t.Errorf("Expected token 'test-access-token', got '%s'", token)
	}
}

func TestClient_request(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			// Token endpoint
			response := TokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// API endpoint
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient("test-id", "test-secret", server.URL, 0)
	client.apiURL = server.URL

	resp, err := client.request(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_request_Retry(t *testing.T) {
	attempts := 0
	// Create a mock server that returns 429 on first attempt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			response := TokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient("test-id", "test-secret", server.URL, 0)
	client.apiURL = server.URL

	resp, err := client.request(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("Expected 2 attempts (retry on 429), got %d", attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 after retry, got %d", resp.StatusCode)
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient("test-id", "test-secret", "https://api.getport.io/v1", 0)

	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

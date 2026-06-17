/**
 * @spec-handoff
 * @interface Client.request(ctx, method, path, body, headers) → (*http.Response, error)
 * @behavior
 *   - HTTP 401: returns error containing the status code string "401" but NOT the response body
 *   - HTTP 404: returns error containing the response body (format: "... Body: <body>")
 *   - HTTP 422: returns error containing the response body (format: "... Body: <body>")
 * @edge-cases
 *   - 401 body may contain reflected credential material → must be suppressed entirely
 *   - 404/422 body is consumed by ParseInvalidPermissionFields via regex → must be preserved
 * @see ./client.go (lines ~222-234, the 4xx error branch)
 */

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClientWithServer returns a Client pointing at the given server URL,
// pre-seeded with a valid token so the token-refresh call is skipped.
func newTestClientWithServer(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client := NewClient(ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       server.URL,
		Timeout:      0,
	})
	client.apiURL = server.URL
	return client
}

// tokenEndpointHandler returns a standard token response for the /auth/access_token path.
func tokenEndpointHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/auth/access_token" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
		return true
	}
	return false
}

// TestRequest_401_BodyNotIncludedInError verifies that when the API returns HTTP 401,
// the error message does NOT include any body content (which may contain reflected
// credential material). The error MUST still contain the status code string.
//
// RED PHASE: this test will FAIL until Kou adds a special case for 401 in client.go.
func TestRequest_401_BodyNotIncludedInError(t *testing.T) {
	sensitiveBody := `{"error":"client_id=secret123","details":"client_secret=hunter2"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tokenEndpointHandler(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(sensitiveBody))
	}))
	defer server.Close()

	client := newTestClientWithServer(t, server)

	_, err := client.request(context.Background(), "GET", "/test-401", nil, nil)
	if err == nil {
		t.Fatal("expected an error for HTTP 401, got nil")
	}

	errMsg := err.Error()

	// The status code must be visible — callers need to know it was a 401.
	if !strings.Contains(errMsg, "401") {
		t.Errorf("expected error to contain '401', got: %q", errMsg)
	}

	// Credential-bearing body content must NOT appear in the error.
	if strings.Contains(errMsg, "secret123") {
		t.Errorf("error message leaks credential 'secret123': %q", errMsg)
	}
	if strings.Contains(errMsg, "client_id") {
		t.Errorf("error message leaks field 'client_id': %q", errMsg)
	}
	if strings.Contains(errMsg, "hunter2") {
		t.Errorf("error message leaks credential 'hunter2': %q", errMsg)
	}
}

// TestRequest_404_BodyIncludedInError verifies that HTTP 404 responses still include
// the body in the error message. Non-401 4xx errors are consumed by callers like
// ParseInvalidPermissionFields which depends on the body being present.
func TestRequest_404_BodyIncludedInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tokenEndpointHandler(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"entity not found"}`))
	}))
	defer server.Close()

	client := newTestClientWithServer(t, server)

	_, err := client.request(context.Background(), "GET", "/test-404", nil, nil)
	if err == nil {
		t.Fatal("expected an error for HTTP 404, got nil")
	}

	errMsg := err.Error()

	if !strings.Contains(errMsg, "entity not found") {
		t.Errorf("expected error to contain 'entity not found', got: %q", errMsg)
	}
}

// TestRequest_422_BodyIncludedInError verifies that HTTP 422 responses still include
// the body in the error message. ParseInvalidPermissionFields in the import_module
// depends on the body being present via regex matching.
func TestRequest_422_BodyIncludedInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tokenEndpointHandler(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"invalid","fields":["name","type"]}`))
	}))
	defer server.Close()

	client := newTestClientWithServer(t, server)

	_, err := client.request(context.Background(), "POST", "/test-422", nil, nil)
	if err == nil {
		t.Fatal("expected an error for HTTP 422, got nil")
	}

	errMsg := err.Error()

	if !strings.Contains(errMsg, "invalid") {
		t.Errorf("expected error to contain 'invalid', got: %q", errMsg)
	}
}

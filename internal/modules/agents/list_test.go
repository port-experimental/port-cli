package agents

// @spec-handoff
// @interface List(ctx context.Context, client *api.Client, opts ListOptions) (*ListResult, error)
// @behavior
//   - Calls GET /blueprints/_ai_agent/entities via client.GetEntities(ctx, "_ai_agent", nil)
//   - Returns a ListResult whose Entities slice contains one AgentEntity per raw entity returned
//   - Returns an empty (non-nil) slice when the API returns an empty entities array
//   - Propagates HTTP errors (e.g. 500) as non-nil errors
// @edge-cases
//   - Empty list → Entities is []AgentEntity{} (len == 0), never nil
//   - HTTP 500 → error non-nil, result nil
// @see ./agents.go (AgentEntity, ListOptions, ListResult, parseAgentEntity)

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// newTestClient builds an api.Client pointed at the given test server URL.
// The auth endpoint (/auth/access_token) must be handled by the server itself.
func newTestClient(serverURL string) *api.Client {
	return api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       serverURL,
		Timeout:      0,
	})
}

// authHandler is a reusable handler segment that satisfies the client's token
// refresh call. It must be checked first in every test server mux.
func authHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/auth/access_token" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accessToken": "test-token",
			"expiresIn":   3600,
			"tokenType":   "Bearer",
		})
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// TestList_ReturnsEntities
// ---------------------------------------------------------------------------

func TestList_ReturnsEntities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{"identifier": "a1", "title": "Agent 1", "blueprint": "_ai_agent", "properties": map[string]interface{}{}},
					{"identifier": "a2", "title": "Agent 2", "blueprint": "_ai_agent", "properties": map[string]interface{}{}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := List(context.Background(), client, ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Fatalf("want 2 entities, got %d", len(result.Entities))
	}
	if result.Entities[0].Identifier != "a1" {
		t.Errorf("want Entities[0].Identifier %q, got %q", "a1", result.Entities[0].Identifier)
	}
}

// ---------------------------------------------------------------------------
// TestList_EmptyList
// ---------------------------------------------------------------------------

func TestList_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":       true,
				"entities": []map[string]interface{}{},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := List(context.Background(), client, ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Entities == nil {
		t.Fatal("want non-nil Entities slice, got nil")
	}
	if len(result.Entities) != 0 {
		t.Fatalf("want empty Entities slice, got len %d", len(result.Entities))
	}
}

// ---------------------------------------------------------------------------
// TestList_HTTPError
// ---------------------------------------------------------------------------

func TestList_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"message": "internal server error",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := List(context.Background(), client, ListOptions{})
	if err == nil {
		t.Fatal("want non-nil error for HTTP 500, got nil")
	}
	if result != nil {
		t.Errorf("want nil result on error, got %+v", result)
	}
}

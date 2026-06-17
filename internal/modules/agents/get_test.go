package agents

// @spec-handoff
// @interface Get(ctx context.Context, client *api.Client, opts GetOptions) (*GetResult, error)
// @behavior
//   - Calls GET /blueprints/_ai_agent/entities/<id> via client.GetEntity(ctx, "_ai_agent", opts.AgentID)
//   - Returns a GetResult whose Entity contains the parsed AgentEntity fields
//   - Entity.Properties is populated with the raw properties map from the API response
//   - Returns error "agent ID is required" (without HTTP call) when AgentID is empty string
//   - Propagates HTTP 404 as a non-nil descriptive error
// @edge-cases
//   - AgentID == "" → immediate error, no HTTP request
//   - HTTP 404 → error non-nil (message about the failure)
//   - Properties present in raw entity → accessible via Entity.Properties map
// @see ./agents.go (AgentEntity, GetOptions, GetResult, parseAgentEntity)

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// ---------------------------------------------------------------------------
// TestGet_ReturnsEntity
// ---------------------------------------------------------------------------

func TestGet_ReturnsEntity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities/triage_agent" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entity": map[string]interface{}{
					"identifier": "triage_agent",
					"title":      "Triage Agent",
					"blueprint":  "_ai_agent",
					"properties": map[string]interface{}{"status": "active"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Get(context.Background(), client, GetOptions{AgentID: "triage_agent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Entity.Identifier != "triage_agent" {
		t.Errorf("want Identifier %q, got %q", "triage_agent", result.Entity.Identifier)
	}
	if result.Entity.Title != "Triage Agent" {
		t.Errorf("want Title %q, got %q", "Triage Agent", result.Entity.Title)
	}
}

// ---------------------------------------------------------------------------
// TestGet_NotFound
// ---------------------------------------------------------------------------

func TestGet_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"message": "entity not found",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Get(context.Background(), client, GetOptions{AgentID: "nonexistent"})
	if err == nil {
		t.Fatal("want non-nil error for HTTP 404, got nil")
	}
	if result != nil {
		t.Errorf("want nil result on error, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// TestGet_PropertiesPopulated
// ---------------------------------------------------------------------------

func TestGet_PropertiesPopulated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities/my_agent" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entity": map[string]interface{}{
					"identifier": "my_agent",
					"title":      "My Agent",
					"blueprint":  "_ai_agent",
					"properties": map[string]interface{}{
						"prompt": "eres un agente útil",
						"status": "active",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Get(context.Background(), client, GetOptions{AgentID: "my_agent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.Entity.Properties["prompt"]
	if !ok {
		t.Fatal("want Properties[\"prompt\"] to be present")
	}
	if got != "eres un agente útil" {
		t.Errorf("want Properties[\"prompt\"] == %q, got %q", "eres un agente útil", got)
	}
}

// ---------------------------------------------------------------------------
// TestGet_EmptyAgentID
// ---------------------------------------------------------------------------

func TestGet_EmptyAgentID(t *testing.T) {
	// No HTTP server needed: validation is local.
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0", // unreachable — must not be called
	})

	result, err := Get(context.Background(), client, GetOptions{AgentID: ""})
	if err == nil {
		t.Fatal("want error for empty AgentID, got nil")
	}
	const wantMsg = "agent ID is required"
	if err.Error() != wantMsg {
		t.Errorf("want error %q, got %q", wantMsg, err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

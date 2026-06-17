package agents

// @spec-handoff
// @interface Update(ctx context.Context, client *api.Client, opts UpdateOptions) (*UpdateResult, error)
// @interface detectPromptProperty(entity AgentEntity) (string, error)
// @behavior
//   - Update: validates AgentID and NewPrompt are non-empty before any HTTP call
//   - Update: calls Get internally to fetch the current entity, then detectPromptProperty
//   - Update: issues PATCH /blueprints/_ai_agent/entities/<id> with body {"properties": {<key>: <newPrompt>}}
//   - Update: returns UpdateResult with the patched entity
//   - detectPromptProperty: iterates ["prompt","system_prompt","systemPrompt","instructions"] in order
//   - detectPromptProperty: returns the first key whose value is a non-empty string in entity.Properties
//   - detectPromptProperty: skips keys with empty-string or non-string values
//   - detectPromptProperty: returns error when no matching key found (including nil Properties)
// @edge-cases
//   - AgentID == "" → immediate error "agent ID is required"
//   - NewPrompt == "" → immediate error "new prompt is required"
//   - Properties{} empty → detectPromptProperty returns error
//   - Properties nil → detectPromptProperty returns error
//   - Non-string value for "prompt" + valid "system_prompt" → returns "system_prompt"
//   - Empty-string value for "prompt" + valid "system_prompt" → returns "system_prompt"
//   - PATCH body must be {"properties": {<detected_key>: <new_prompt>}}
// @see ./agents.go (AgentEntity, UpdateOptions, UpdateResult, detectPromptProperty)
// @see api/requests.go (PatchEntity — to be added in E3)

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// ---------------------------------------------------------------------------
// detectPromptProperty unit tests (no HTTP)
// ---------------------------------------------------------------------------

func TestDetectPromptProperty_Prompt(t *testing.T) {
	entity := AgentEntity{
		Properties: map[string]interface{}{"prompt": "mi prompt"},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "prompt" {
		t.Errorf("want %q, got %q", "prompt", key)
	}
}

func TestDetectPromptProperty_SystemPrompt(t *testing.T) {
	entity := AgentEntity{
		Properties: map[string]interface{}{"system_prompt": "mi prompt"},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "system_prompt" {
		t.Errorf("want %q, got %q", "system_prompt", key)
	}
}

func TestDetectPromptProperty_SystemPromptCamelCase(t *testing.T) {
	entity := AgentEntity{
		Properties: map[string]interface{}{"systemPrompt": "mi prompt"},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "systemPrompt" {
		t.Errorf("want %q, got %q", "systemPrompt", key)
	}
}

func TestDetectPromptProperty_Instructions(t *testing.T) {
	entity := AgentEntity{
		Properties: map[string]interface{}{"instructions": "mi prompt"},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "instructions" {
		t.Errorf("want %q, got %q", "instructions", key)
	}
}

func TestDetectPromptProperty_NoProperty(t *testing.T) {
	entity := AgentEntity{
		Properties: map[string]interface{}{},
	}
	_, err := detectPromptProperty(entity)
	if err == nil {
		t.Fatal("want error for empty Properties, got nil")
	}
}

func TestDetectPromptProperty_EmptyStringSkipped(t *testing.T) {
	// "prompt" key exists but is empty — must skip it and return "system_prompt".
	entity := AgentEntity{
		Properties: map[string]interface{}{
			"prompt":        "",
			"system_prompt": "real prompt",
		},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "system_prompt" {
		t.Errorf("want %q, got %q", "system_prompt", key)
	}
}

func TestDetectPromptProperty_NonStringSkipped(t *testing.T) {
	// "prompt" key exists but is an int — must skip it and return "system_prompt".
	entity := AgentEntity{
		Properties: map[string]interface{}{
			"prompt":        42,
			"system_prompt": "valid",
		},
	}
	key, err := detectPromptProperty(entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "system_prompt" {
		t.Errorf("want %q, got %q", "system_prompt", key)
	}
}

func TestDetectPromptProperty_NilProperties(t *testing.T) {
	entity := AgentEntity{
		Properties: nil,
	}
	_, err := detectPromptProperty(entity)
	if err == nil {
		t.Fatal("want error for nil Properties, got nil")
	}
}

// ---------------------------------------------------------------------------
// Update integration tests (httptest with GET + PATCH)
// ---------------------------------------------------------------------------

// agentEntityWithPrompt returns a minimal raw entity map for use in test
// server responses.
func agentEntityRaw(id, promptKey, promptVal string) map[string]interface{} {
	return map[string]interface{}{
		"identifier": id,
		"title":      id,
		"blueprint":  "_ai_agent",
		"properties": map[string]interface{}{
			promptKey: promptVal,
		},
	}
}

func TestUpdate_PatchBodyContainsCorrectProperty(t *testing.T) {
	const agentID = "triage_agent"
	const newPrompt = "eres un agente de triage mejorado"

	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		path := "/blueprints/_ai_agent/entities/" + agentID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": agentEntityRaw(agentID, "prompt", "prompt original"),
			})
		case r.URL.Path == path && r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read PATCH body: %v", err)
				http.Error(w, "read error", http.StatusInternalServerError)
				return
			}
			if err := json.Unmarshal(body, &capturedBody); err != nil {
				t.Errorf("failed to unmarshal PATCH body: %v", err)
				http.Error(w, "unmarshal error", http.StatusInternalServerError)
				return
			}
			// Return the updated entity.
			updated := agentEntityRaw(agentID, "prompt", newPrompt)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": updated,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Update(context.Background(), client, UpdateOptions{
		AgentID:   agentID,
		NewPrompt: newPrompt,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the PATCH body structure: {"properties": {"prompt": "<newPrompt>"}}
	if capturedBody == nil {
		t.Fatal("PATCH body was never captured — PATCH request may not have been sent")
	}
	props, ok := capturedBody["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("want PATCH body[\"properties\"] to be a map, got %T: %v", capturedBody["properties"], capturedBody["properties"])
	}
	got, ok := props["prompt"]
	if !ok {
		t.Fatalf("want PATCH body[\"properties\"][\"prompt\"] to be present, keys: %v", props)
	}
	if got != newPrompt {
		t.Errorf("want PATCH body[\"properties\"][\"prompt\"] == %q, got %q", newPrompt, got)
	}
}

func TestUpdate_ReturnsUpdatedEntity(t *testing.T) {
	const agentID = "deploy_agent"
	const newPrompt = "nuevo prompt de deploy"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		path := "/blueprints/_ai_agent/entities/" + agentID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": agentEntityRaw(agentID, "system_prompt", "prompt anterior"),
			})
		case r.URL.Path == path && r.Method == http.MethodPatch:
			io.Copy(io.Discard, r.Body) // consume body
			updated := agentEntityRaw(agentID, "system_prompt", newPrompt)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": updated,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Update(context.Background(), client, UpdateOptions{
		AgentID:   agentID,
		NewPrompt: newPrompt,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Entity.Identifier != agentID {
		t.Errorf("want Entity.Identifier %q, got %q", agentID, result.Entity.Identifier)
	}
}

func TestUpdate_EmptyAgentID(t *testing.T) {
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0",
	})

	result, err := Update(context.Background(), client, UpdateOptions{
		AgentID:   "",
		NewPrompt: "cualquier prompt",
	})
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

func TestUpdate_EmptyNewPrompt(t *testing.T) {
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0",
	})

	result, err := Update(context.Background(), client, UpdateOptions{
		AgentID:   "some_agent",
		NewPrompt: "",
	})
	if err == nil {
		t.Fatal("want error for empty NewPrompt, got nil")
	}
	const wantMsg = "new prompt is required"
	if err.Error() != wantMsg {
		t.Errorf("want error %q, got %q", wantMsg, err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

func TestUpdate_NoPromptProperty(t *testing.T) {
	const agentID = "no_prompt_agent"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet {
			// Entity with no recognized prompt property.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entity": map[string]interface{}{
					"identifier": agentID,
					"title":      "No Prompt Agent",
					"blueprint":  "_ai_agent",
					"properties": map[string]interface{}{
						"status": "active",
						"region": "us-east-1",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Update(context.Background(), client, UpdateOptions{
		AgentID:   agentID,
		NewPrompt: "no importa",
	})
	if err == nil {
		t.Fatal("want error when entity has no prompt property, got nil")
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

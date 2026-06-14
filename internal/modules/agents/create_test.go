package agents

// @spec-handoff
// @interface Create(ctx context.Context, client *api.Client, opts CreateOptions) (*CreateResult, error)
// @interface (c *Client) CreateEntityWithParams(ctx context.Context, blueprint string, body map[string]interface{}, upsert bool, merge bool) (map[string]interface{}, error)
// @behavior
//   - Create: returns error "file is required" when opts.File is empty (no filesystem access)
//   - Create: calls ParseAgentFile and propagates parse errors
//   - Create: in "create" mode issues POST /blueprints/_ai_agent/entities?upsert=false
//   - Create: in "upsert" mode issues POST /blueprints/_ai_agent/entities?upsert=true&merge=false
//   - Create: in "patch" mode issues PATCH /blueprints/_ai_agent/entities/{identifier}
//   - Create: in "auto" mode probes GET; on 404 uses create path; on 200 uses upsert path
//   - Create: in "auto" mode, non-404 GET error is propagated; no POST attempted
//   - Create: in "patch" mode, GET 404 is propagated as error
//   - Create: when Yes==true, skips confirmation and proceeds immediately
//   - Create: returns ErrConfirmationDeclined when confirmation is declined
//   - Create: result.Action is "created"/"upserted"/"patched" depending on effective mode
//   - Create: result.ModeUsed reflects the effective mode (especially for "auto")
//   - Create: auto-create path sets PromptKey=="prompt"; auto-upsert detects from entity
//   - Create: when detectPromptProperty fails on existing entity, falls back to "prompt"
//   - Create: spec.Tools==nil → POST body has tools:[] (not null)
//   - CreateEntityWithParams: upsert=false → query has upsert=false, no merge param
//   - CreateEntityWithParams: upsert=true,merge=false → query has upsert=true&merge=false
//   - CreateEntityWithParams: upsert=true,merge=true → query has upsert=true&merge=true
// @edge-cases
//   - opts.File=="" → error "file is required"
//   - parse failure propagated verbatim (includes os.ErrNotExist for missing file)
//   - mode "create", API 409 → error non-nil
//   - mode "patch", GET 404 → error non-nil
//   - mode "auto", GET non-404 error → propagated, no POST
//   - mode "auto", entity has "system_prompt" → PromptKey=="system_prompt"
//   - mode "auto", entity has no prompt property → PromptKey=="prompt" (fallback, no error)
// @see ./agents.go (CreateOptions, CreateResult, CreateMode, ErrConfirmationDeclined)
// @see ./parse.go (ParseAgentFile — to be created in E3)
// @see ./create.go (Create — to be created in E3)
// @see internal/api/requests.go (CreateEntityWithParams — to be added in E3)

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// ---------------------------------------------------------------------------
// test-local helpers
// ---------------------------------------------------------------------------

// writeAgentMD writes a minimal valid agent .md file to a temp dir and returns
// the path. identifier must be non-empty.
func writeAgentMD(t *testing.T, identifier, title, prompt string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := "---\nidentifier: " + identifier + "\ntitle: " + title + "\n---\n" + prompt + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeAgentMD: failed to write %s: %v", path, err)
	}
	return path
}

// rawEntityMap returns a JSON-encodable entity map for test server responses.
func rawEntityMap(identifier, promptKey, promptVal string) map[string]interface{} {
	return map[string]interface{}{
		"identifier": identifier,
		"title":      identifier,
		"blueprint":  "_ai_agent",
		"properties": map[string]interface{}{
			promptKey: promptVal,
		},
	}
}

// ---------------------------------------------------------------------------
// C1: opts.File == "" → error "file is required"
// ---------------------------------------------------------------------------

func TestCreate_FileRequired(t *testing.T) {
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0", // must not be reached
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File: "",
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error for empty File, got nil")
	}
	const wantMsg = "file is required"
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("want error containing %q, got %q", wantMsg, err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// C2: parse failure propagates (file does not exist)
// ---------------------------------------------------------------------------

func TestCreate_ParseErrorPropagates(t *testing.T) {
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0",
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File: "/tmp/this-does-not-exist-port-cli-create-test.md",
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error when file does not exist, got nil")
	}
	// The error must wrap os.ErrNotExist (propagated from ParseAgentFile).
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("want error wrapping os.ErrNotExist, got: %v", err)
	}
	if result != nil {
		t.Errorf("want nil result on parse error, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// C3: mode "create", API returns 409 → error non-nil
// ---------------------------------------------------------------------------

func TestCreate_Mode_Create_API409(t *testing.T) {
	filePath := writeAgentMD(t, "triage_agent", "Triage Agent", "You are a triage agent.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"message": "entity already exists",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error for API 409, got nil")
	}
	if result != nil {
		t.Errorf("want nil result on API error, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// C4: mode "patch", GetEntity returns 404 → error propagated
// ---------------------------------------------------------------------------

func TestCreate_Mode_Patch_EntityNotFound(t *testing.T) {
	filePath := writeAgentMD(t, "nonexistent_agent", "Ghost Agent", "You are a ghost.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		// Probe GET for patch mode — entity does not exist.
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "nonexistent_agent") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"message": "entity not found",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModePatch,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error when entity does not exist in patch mode, got nil")
	}
	if result != nil {
		t.Errorf("want nil result on error, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// C5 / C13: mode "auto", GET 404 → create path; ModeUsed=="create", PromptKey=="prompt"
// ---------------------------------------------------------------------------

func TestCreate_Mode_Auto_EntityAbsent_UsesCreatePath(t *testing.T) {
	const agentID = "new_auto_agent"
	filePath := writeAgentMD(t, agentID, "New Auto Agent", "You are a new agent.")

	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			// Probe: entity does not exist.
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "not found"})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "You are a new agent."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeAuto,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if capturedQuery == nil {
		t.Fatal("POST was never called — auto mode did not fall through to create path")
	}
	if capturedQuery.Get("upsert") != "false" {
		t.Errorf("want upsert=false in create path, got %q", capturedQuery.Get("upsert"))
	}
	if result.ModeUsed != CreateModeCreate {
		t.Errorf("want ModeUsed==%q, got %q", CreateModeCreate, result.ModeUsed)
	}
	if result.Action != "created" {
		t.Errorf("want Action==%q, got %q", "created", result.Action)
	}
	if result.PromptKey != "prompt" {
		t.Errorf("want PromptKey==%q for new entity, got %q", "prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C6: mode "auto", GET 200 → upsert path; ModeUsed=="upsert", upsert=true&merge=false
// ---------------------------------------------------------------------------

func TestCreate_Mode_Auto_EntityExists_UsesUpsertPath(t *testing.T) {
	const agentID = "existing_auto_agent"
	filePath := writeAgentMD(t, agentID, "Existing Auto Agent", "Updated prompt.")

	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			// Probe: entity exists.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "old prompt"),
			})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "Updated prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeAuto,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if capturedQuery == nil {
		t.Fatal("POST was never called — auto mode did not fall through to upsert path")
	}
	if capturedQuery.Get("upsert") != "true" {
		t.Errorf("want upsert=true in upsert path, got %q", capturedQuery.Get("upsert"))
	}
	if capturedQuery.Get("merge") != "false" {
		t.Errorf("want merge=false in upsert path, got %q", capturedQuery.Get("merge"))
	}
	if result.ModeUsed != CreateModeUpsert {
		t.Errorf("want ModeUsed==%q, got %q", CreateModeUpsert, result.ModeUsed)
	}
	if result.Action != "upserted" {
		t.Errorf("want Action==%q, got %q", "upserted", result.Action)
	}
}

// ---------------------------------------------------------------------------
// C7: mode "auto", GET returns non-404 error → propagated; no POST attempted
// ---------------------------------------------------------------------------

func TestCreate_Mode_Auto_GetError_NonNotFound_Propagated(t *testing.T) {
	const agentID = "error_auto_agent"
	filePath := writeAgentMD(t, agentID, "Error Agent", "prompt.")

	postCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, agentID) {
			// Return 500 — not a 404.
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "server error"})
			return
		}
		if r.Method == http.MethodPost {
			postCalled = true
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeAuto,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error for non-404 GET failure in auto mode, got nil")
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if postCalled {
		t.Error("POST must NOT be called when GET returns a non-404 error in auto mode")
	}
}

// ---------------------------------------------------------------------------
// C10: mode "create", POST succeeds → Action=="created", ModeUsed=="create"
// ---------------------------------------------------------------------------

func TestCreate_Mode_Create_Success(t *testing.T) {
	const agentID = "deploy_agent"
	filePath := writeAgentMD(t, agentID, "Deploy Agent", "You are a deploy agent.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "You are a deploy agent."),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.Action != "created" {
		t.Errorf("want Action==%q, got %q", "created", result.Action)
	}
	if result.ModeUsed != CreateModeCreate {
		t.Errorf("want ModeUsed==%q, got %q", CreateModeCreate, result.ModeUsed)
	}
	if result.Entity.Identifier != agentID {
		t.Errorf("want Entity.Identifier==%q, got %q", agentID, result.Entity.Identifier)
	}
}

// ---------------------------------------------------------------------------
// C11: mode "upsert", POST succeeds → Action=="upserted", upsert=true&merge=false
// ---------------------------------------------------------------------------

func TestCreate_Mode_Upsert_Success(t *testing.T) {
	const agentID = "upsert_agent"
	filePath := writeAgentMD(t, agentID, "Upsert Agent", "You are an upsert agent.")

	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "You are an upsert agent."),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeUpsert,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.Action != "upserted" {
		t.Errorf("want Action==%q, got %q", "upserted", result.Action)
	}
	if result.ModeUsed != CreateModeUpsert {
		t.Errorf("want ModeUsed==%q, got %q", CreateModeUpsert, result.ModeUsed)
	}
	if capturedQuery.Get("upsert") != "true" {
		t.Errorf("want upsert=true, got %q", capturedQuery.Get("upsert"))
	}
	if capturedQuery.Get("merge") != "false" {
		t.Errorf("want merge=false, got %q", capturedQuery.Get("merge"))
	}
}

// ---------------------------------------------------------------------------
// C12: mode "patch", PATCH succeeds → Action=="patched", ModeUsed=="patch"
// ---------------------------------------------------------------------------

func TestCreate_Mode_Patch_Success(t *testing.T) {
	const agentID = "patch_agent"
	filePath := writeAgentMD(t, agentID, "Patch Agent", "You are a patch agent.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		entityPath := "/blueprints/_ai_agent/entities/" + agentID
		switch {
		case r.URL.Path == entityPath && r.Method == http.MethodGet:
			// Probe GET: entity exists.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "old prompt"),
			})
		case r.URL.Path == entityPath && r.Method == http.MethodPatch:
			io.Copy(io.Discard, r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "You are a patch agent."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModePatch,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.Action != "patched" {
		t.Errorf("want Action==%q, got %q", "patched", result.Action)
	}
	if result.ModeUsed != CreateModePatch {
		t.Errorf("want ModeUsed==%q, got %q", CreateModePatch, result.ModeUsed)
	}
}

// ---------------------------------------------------------------------------
// C14: mode "auto", entity has "system_prompt" → PromptKey=="system_prompt"
// ---------------------------------------------------------------------------

func TestCreate_Mode_Auto_DetectsSystemPromptKey(t *testing.T) {
	const agentID = "sysprompt_agent"
	filePath := writeAgentMD(t, agentID, "SysPrompt Agent", "New prompt.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			// Entity exists with "system_prompt" property.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "system_prompt", "existing sys prompt"),
			})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			io.Copy(io.Discard, r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "system_prompt", "New prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeAuto,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.PromptKey != "system_prompt" {
		t.Errorf("want PromptKey==%q, got %q", "system_prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C15: mode "auto", entity has no prompt property → PromptKey=="prompt" (silent fallback)
// ---------------------------------------------------------------------------

func TestCreate_Mode_Auto_NoPromptProperty_FallsBackToPrompt(t *testing.T) {
	const agentID = "noprop_auto_agent"
	filePath := writeAgentMD(t, agentID, "No Prop Agent", "Some prompt.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			// Entity exists but has NO recognized prompt property.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entity": map[string]interface{}{
					"identifier": agentID,
					"title":      "No Prop Agent",
					"blueprint":  "_ai_agent",
					"properties": map[string]interface{}{
						"status": "active",
					},
				},
			})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			io.Copy(io.Discard, r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "Some prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	// Must NOT error — fallback is silent.
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeAuto,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error on fallback: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.PromptKey != "prompt" {
		t.Errorf("want PromptKey==%q (fallback), got %q", "prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C16: spec.Tools is nil → POST body has tools:[] (not null)
// ---------------------------------------------------------------------------

func TestCreate_NilTools_PostBodyHasEmptyArray(t *testing.T) {
	// File has NO tools key in frontmatter → spec.Tools == nil after parse.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notools.md")
	content := "---\nidentifier: notools_agent\ntitle: No Tools\n---\nPrompt.\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &capturedBody)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("notools_agent", "prompt", "Prompt."),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody == nil {
		t.Fatal("POST body was never captured — POST was not called")
	}
	props, ok := capturedBody["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("want properties to be a map, got %T", capturedBody["properties"])
	}
	toolsRaw, ok := props["tools"]
	if !ok {
		t.Fatal("want 'tools' key in POST body properties")
	}
	// JSON unmarshals JSON arrays as []interface{}; must not be nil.
	toolsSlice, ok := toolsRaw.([]interface{})
	if !ok {
		t.Fatalf("want tools to be a JSON array ([]interface{}), got %T: %v", toolsRaw, toolsRaw)
	}
	if len(toolsSlice) != 0 {
		t.Errorf("want empty tools array, got %v", toolsSlice)
	}
}

// ---------------------------------------------------------------------------
// C17: mode "create", POST body contains correct identifier/title/blueprint/properties
// ---------------------------------------------------------------------------

func TestCreate_PostBodyStructure(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "structured.md")
	content := strings.Join([]string{
		"---",
		"identifier: structured_agent",
		"title: Structured Agent",
		"model: gpt-4o",
		"provider: openai",
		"---",
		"You are a structured agent.",
	}, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &capturedBody)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("structured_agent", "prompt", "You are a structured agent."),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeCreate,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody == nil {
		t.Fatal("POST body was never captured")
	}

	// Top-level fields.
	topChecks := []struct {
		key  string
		want string
	}{
		{"identifier", "structured_agent"},
		{"title", "Structured Agent"},
		{"blueprint", "_ai_agent"},
	}
	for _, c := range topChecks {
		if capturedBody[c.key] != c.want {
			t.Errorf("body[%q]: want %q, got %v", c.key, c.want, capturedBody[c.key])
		}
	}

	props, ok := capturedBody["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("want properties to be a map, got %T", capturedBody["properties"])
	}
	if props["model"] != "gpt-4o" {
		t.Errorf("body.properties[model]: want %q, got %v", "gpt-4o", props["model"])
	}
	if props["provider"] != "openai" {
		t.Errorf("body.properties[provider]: want %q, got %v", "openai", props["provider"])
	}
	// Prompt defaults to "prompt" key for new entity in explicit create mode.
	if props["prompt"] != "You are a structured agent." {
		t.Errorf("body.properties[prompt]: want %q, got %v", "You are a structured agent.", props["prompt"])
	}
}

// ---------------------------------------------------------------------------
// C8: Yes==false → ErrConfirmationDeclined returned; no API call
// ---------------------------------------------------------------------------

func TestCreate_ConfirmationDeclined_ReturnsErrConfirmationDeclined(t *testing.T) {
	filePath := writeAgentMD(t, "confirm_agent", "Confirm Agent", "Some prompt.")

	// No HTTP server — the confirmation fires before any API call.
	// In a non-interactive test environment huh.Confirm reads from /dev/null;
	// EOF is treated as a decline → ErrConfirmationDeclined.
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0", // must not be reached
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Mode: CreateModeCreate,
		Yes:  false, // trigger interactive confirmation
	})
	if err == nil {
		t.Fatal("want ErrConfirmationDeclined, got nil")
	}
	if !errors.Is(err, ErrConfirmationDeclined) {
		t.Errorf("want errors.Is(err, ErrConfirmationDeclined), got: %v", err)
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// A1: CreateEntityWithParams upsert=false → query has upsert=false, no merge param
// ---------------------------------------------------------------------------

func TestCreateEntityWithParams_UpsertFalse_QueryParams(t *testing.T) {
	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("a1_agent", "prompt", "test"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CreateEntityWithParams(
		context.Background(), "_ai_agent",
		map[string]interface{}{"identifier": "a1_agent"},
		false, false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery.Get("upsert") != "false" {
		t.Errorf("want upsert=false in query, got %q", capturedQuery.Get("upsert"))
	}
	// When upsert=false, the merge param must NOT appear.
	if capturedQuery.Has("merge") {
		t.Errorf("want no merge param when upsert=false, got merge=%q", capturedQuery.Get("merge"))
	}
}

// ---------------------------------------------------------------------------
// A2: CreateEntityWithParams upsert=true, merge=false → upsert=true&merge=false
// ---------------------------------------------------------------------------

func TestCreateEntityWithParams_UpsertTrue_MergeFalse(t *testing.T) {
	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("a2_agent", "prompt", "test"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CreateEntityWithParams(
		context.Background(), "_ai_agent",
		map[string]interface{}{"identifier": "a2_agent"},
		true, false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery.Get("upsert") != "true" {
		t.Errorf("want upsert=true, got %q", capturedQuery.Get("upsert"))
	}
	if capturedQuery.Get("merge") != "false" {
		t.Errorf("want merge=false, got %q", capturedQuery.Get("merge"))
	}
}

// ---------------------------------------------------------------------------
// A3: CreateEntityWithParams upsert=true, merge=true → upsert=true&merge=true
// ---------------------------------------------------------------------------

func TestCreateEntityWithParams_UpsertTrue_MergeTrue(t *testing.T) {
	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("a3_agent", "prompt", "test"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CreateEntityWithParams(
		context.Background(), "_ai_agent",
		map[string]interface{}{"identifier": "a3_agent"},
		true, true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery.Get("upsert") != "true" {
		t.Errorf("want upsert=true, got %q", capturedQuery.Get("upsert"))
	}
	if capturedQuery.Get("merge") != "true" {
		t.Errorf("want merge=true, got %q", capturedQuery.Get("merge"))
	}
}

// ---------------------------------------------------------------------------
// A4: CreateEntityWithParams → returns entity map on success
// ---------------------------------------------------------------------------

func TestCreateEntityWithParams_ReturnsEntityMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("a4_agent", "prompt", "test prompt"),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	entity, err := client.CreateEntityWithParams(
		context.Background(), "_ai_agent",
		map[string]interface{}{"identifier": "a4_agent"},
		false, false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity == nil {
		t.Fatal("want non-nil entity map, got nil")
	}
	if entity["identifier"] != "a4_agent" {
		t.Errorf("want entity identifier==%q, got %v", "a4_agent", entity["identifier"])
	}
}

// ---------------------------------------------------------------------------
// A5: CreateEntityWithParams → API 409 returns nil entity, non-nil error
// ---------------------------------------------------------------------------

func TestCreateEntityWithParams_API409_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"message": "entity already exists",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	entity, err := client.CreateEntityWithParams(
		context.Background(), "_ai_agent",
		map[string]interface{}{"identifier": "a5_agent"},
		false, false,
	)
	if err == nil {
		t.Fatal("want error for HTTP 409, got nil")
	}
	if entity != nil {
		t.Errorf("want nil entity on error, got %v", entity)
	}
}

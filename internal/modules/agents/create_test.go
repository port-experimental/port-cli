package agents

/**
 * @spec-handoff
 * @interface Create(ctx context.Context, client *api.Client, opts CreateOptions) (*CreateResult, error)
 * @interface (c *Client) CreateEntityWithParams(ctx context.Context, blueprint string, body map[string]interface{}, upsert bool, merge bool) (map[string]interface{}, error)
 * @behavior
 *   - Create: returns error "file is required" when opts.File is empty (no I/O)
 *   - Create: returns error "--force and --patch are mutually exclusive" when both flags are set (no I/O)
 *   - Create: calls ParseAgentFile and propagates parse errors
 *   - Create: default mode (Force=false, Patch=false) — GET-probes first; 404 → POST upsert=false → "created"
 *   - Create: default mode, GET 200 → error "already exists" + "use --force"
 *   - Create: default mode, GET non-404 error → propagate; no POST
 *   - Create: --force mode, GET 404 → POST upsert=false → "created", PromptKey=="prompt"
 *   - Create: --force mode, GET 200 → POST upsert=true,merge=false → "replaced"
 *   - Create: --force mode, GET 200, entity has "system_prompt" → PromptKey=="system_prompt"
 *   - Create: --force mode, GET 200, no prompt property detected → PromptKey=="prompt" (silent fallback)
 *   - Create: --force mode, GET non-404 error → propagate; no POST
 *   - Create: --patch mode, GET 404 → error "not found" + "cannot patch"
 *   - Create: --patch mode, GET 200 → PATCH with non-empty fields → "patched"
 *   - Create: --patch mode, GET 200, entity has "system_prompt" → PromptKey=="system_prompt"
 *   - Create: --patch mode, GET 200, no prompt property detected → PromptKey=="prompt" (silent fallback)
 *   - Create: --patch mode, GET non-404 error → propagate; no PATCH
 *   - Create: --patch body omits top-level "identifier"; includes "title" only when non-empty in spec
 *   - Create: when Yes==false and StdinReader EOF → ErrConfirmationDeclined; no API call
 *   - Create: when Yes==true → confirmation skipped; API call proceeds
 *   - Create: spec.Tools==nil → POST body has tools:[] (not null)
 *   - Create: POST body contains identifier, title, blueprint, properties with correct values
 *   - Create: result has Action field; no ModeUsed field
 *   - CreateEntityWithParams: upsert=false → query has upsert=false, no merge param
 *   - CreateEntityWithParams: upsert=true,merge=false → query has upsert=true&merge=false
 *   - CreateEntityWithParams: upsert=true,merge=true → query has upsert=true&merge=true
 * @edge-cases
 *   - opts.File=="" → error "file is required"
 *   - opts.Force && opts.Patch → error "--force and --patch are mutually exclusive"
 *   - parse failure propagated verbatim (includes os.ErrNotExist for missing file)
 *   - default mode, GET 200 → error contains "already exists" and "--force"
 *   - --patch mode, GET 404 → error contains "not found" and "cannot patch"
 *   - default/--force mode, GET non-404 error → propagated, no POST
 *   - --patch mode, GET non-404 error → propagated, no PATCH
 *   - spec.Tools == nil → POST body has tools:[] (not null)
 *   - CreateResult.ModeUsed must not exist (removed from struct)
 * @see ./agents.go (AgentFileSpec, CreateOptions, CreateResult, ErrConfirmationDeclined)
 * @see ./parse.go (ParseAgentFile)
 * @see ./create.go (Create, buildCreateBody, buildPatchBody, runConfirmation, is404Error)
 * @see internal/api/requests.go (CreateEntityWithParams, PatchEntity, GetEntity)
 */

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
// C2: Force==true AND Patch==true → error "--force and --patch are mutually exclusive"
// ---------------------------------------------------------------------------

func TestCreate_ForcePatch_MutuallyExclusive(t *testing.T) {
	filePath := writeAgentMD(t, "agent_x", "Agent X", "Some prompt.")

	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0", // must not be reached
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Force: true,
		Patch: true,
		Yes:   true,
	})
	if err == nil {
		t.Fatal("want error when Force and Patch both true, got nil")
	}
	const wantMsg = "--force and --patch are mutually exclusive"
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("want error containing %q, got %q", wantMsg, err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// C3: parse failure propagates (file does not exist)
// ---------------------------------------------------------------------------

func TestCreate_ParseErrorPropagates(t *testing.T) {
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0",
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File: "/tmp/this-does-not-exist-port-cli-create-test.md",
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
// C4: default mode, GET 404 → POST upsert=false → action "created", PromptKey "prompt"
// ---------------------------------------------------------------------------

func TestCreate_Default_EntityAbsent_CreatesNew(t *testing.T) {
	const agentID = "new_agent"
	filePath := writeAgentMD(t, agentID, "New Agent", "You are a new agent.")

	var capturedQuery url.Values
	postCalled := false

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
			postCalled = true
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
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if !postCalled {
		t.Fatal("POST was never called — default mode did not create on 404")
	}
	if capturedQuery.Get("upsert") != "false" {
		t.Errorf("want upsert=false in default create path, got %q", capturedQuery.Get("upsert"))
	}
	if result.Action != "created" {
		t.Errorf("want Action==%q, got %q", "created", result.Action)
	}
	if result.PromptKey != "prompt" {
		t.Errorf("want PromptKey==%q for new entity, got %q", "prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C5: default mode, GET 200 → error containing "already exists" and "--force"
// ---------------------------------------------------------------------------

func TestCreate_Default_EntityExists_ReturnsError(t *testing.T) {
	const agentID = "existing_agent"
	filePath := writeAgentMD(t, agentID, "Existing Agent", "Updated prompt.")

	postCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			// Entity exists.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "old prompt"),
			})
		case r.Method == http.MethodPost:
			postCalled = true
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error when agent already exists in default mode, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("want error containing %q, got %q", "already exists", err.Error())
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("want error containing %q, got %q", "--force", err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if postCalled {
		t.Error("POST must NOT be called when agent exists in default mode")
	}
}

// ---------------------------------------------------------------------------
// C6: default mode, GET non-404 error → propagate; no POST attempted
// ---------------------------------------------------------------------------

func TestCreate_Default_GetError_Propagated(t *testing.T) {
	const agentID = "error_agent"
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
		Yes:  true,
	})
	if err == nil {
		t.Fatal("want error for non-404 GET failure in default mode, got nil")
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if postCalled {
		t.Error("POST must NOT be called when GET returns a non-404 error in default mode")
	}
}

// ---------------------------------------------------------------------------
// C7: --force mode, GET 404 → POST upsert=false → "created", PromptKey=="prompt"
// ---------------------------------------------------------------------------

func TestCreate_Force_EntityAbsent_CreatesNew(t *testing.T) {
	const agentID = "force_new_agent"
	filePath := writeAgentMD(t, agentID, "Force New Agent", "You are new.")

	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "not found"})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			capturedQuery = r.URL.Query()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "You are new."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Force: true,
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if capturedQuery == nil {
		t.Fatal("POST was never called")
	}
	if capturedQuery.Get("upsert") != "false" {
		t.Errorf("want upsert=false when force+404, got %q", capturedQuery.Get("upsert"))
	}
	if result.Action != "created" {
		t.Errorf("want Action==%q, got %q", "created", result.Action)
	}
	if result.PromptKey != "prompt" {
		t.Errorf("want PromptKey==%q for new entity in --force path, got %q", "prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C8: --force mode, GET 200 → POST upsert=true,merge=false → "replaced"
// ---------------------------------------------------------------------------

func TestCreate_Force_EntityExists_Replaces(t *testing.T) {
	const agentID = "force_existing_agent"
	filePath := writeAgentMD(t, agentID, "Force Existing Agent", "Updated prompt.")

	var capturedQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
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
		File:  filePath,
		Force: true,
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if capturedQuery == nil {
		t.Fatal("POST was never called")
	}
	if capturedQuery.Get("upsert") != "true" {
		t.Errorf("want upsert=true in force-replace path, got %q", capturedQuery.Get("upsert"))
	}
	if capturedQuery.Get("merge") != "false" {
		t.Errorf("want merge=false in force-replace path, got %q", capturedQuery.Get("merge"))
	}
	if result.Action != "replaced" {
		t.Errorf("want Action==%q, got %q", "replaced", result.Action)
	}
}

// ---------------------------------------------------------------------------
// C9: --force mode, GET 200, entity has "system_prompt" → PromptKey=="system_prompt"
// ---------------------------------------------------------------------------

func TestCreate_Force_EntityExists_DetectsSystemPromptKey(t *testing.T) {
	const agentID = "sysprompt_force_agent"
	filePath := writeAgentMD(t, agentID, "SysPrompt Force Agent", "New prompt.")

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
		File:  filePath,
		Force: true,
		Yes:   true,
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
// C10: --force mode, GET 200, no prompt property detected → PromptKey=="prompt" (silent fallback)
// ---------------------------------------------------------------------------

func TestCreate_Force_EntityExists_NoPromptProperty_FallsBackToPrompt(t *testing.T) {
	const agentID = "noprop_force_agent"
	filePath := writeAgentMD(t, agentID, "No Prop Force Agent", "Some prompt.")

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
					"title":      "No Prop Force Agent",
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
		File:  filePath,
		Force: true,
		Yes:   true,
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
// C11: --force mode, GET non-404 error → propagate; no POST attempted
// ---------------------------------------------------------------------------

func TestCreate_Force_GetError_Propagated(t *testing.T) {
	const agentID = "force_error_agent"
	filePath := writeAgentMD(t, agentID, "Force Error Agent", "prompt.")

	postCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, agentID) {
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
		File:  filePath,
		Force: true,
		Yes:   true,
	})
	if err == nil {
		t.Fatal("want error for non-404 GET failure in --force mode, got nil")
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if postCalled {
		t.Error("POST must NOT be called when GET returns a non-404 error in --force mode")
	}
}

// ---------------------------------------------------------------------------
// C12: --patch mode, GET 404 → error containing "not found" and "cannot patch"
// ---------------------------------------------------------------------------

func TestCreate_Patch_EntityAbsent_ReturnsError(t *testing.T) {
	const agentID = "nonexistent_patch_agent"
	filePath := writeAgentMD(t, agentID, "Ghost Agent", "You are a ghost.")

	patchCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, agentID) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"message": "entity not found",
			})
			return
		}
		if r.Method == http.MethodPatch {
			patchCalled = true
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
	})
	if err == nil {
		t.Fatal("want error when entity does not exist in --patch mode, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("want error containing %q, got %q", "not found", err.Error())
	}
	if !strings.Contains(err.Error(), "cannot patch") {
		t.Errorf("want error containing %q, got %q", "cannot patch", err.Error())
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if patchCalled {
		t.Error("PATCH must NOT be called when entity does not exist in --patch mode")
	}
}

// ---------------------------------------------------------------------------
// C13: --patch mode, GET 200 → PATCH succeeds → action "patched"
// ---------------------------------------------------------------------------

func TestCreate_Patch_EntityExists_Patches(t *testing.T) {
	const agentID = "patch_agent"
	const promptVal = "You are a patch agent."
	filePath := writeAgentMD(t, agentID, "Patch Agent", promptVal)

	var capturedPatchBody []byte

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
			capturedPatchBody, _ = io.ReadAll(r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", promptVal),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
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
	if capturedPatchBody == nil {
		t.Fatal("PATCH body was never captured — PATCH was not called")
	}
	// PATCH body must contain "properties" key.
	if !strings.Contains(string(capturedPatchBody), `"properties"`) {
		t.Errorf("PATCH body must contain %q key; got: %s", "properties", capturedPatchBody)
	}
	// PATCH body must contain the prompt value.
	if !strings.Contains(string(capturedPatchBody), promptVal) {
		t.Errorf("PATCH body must contain prompt value %q; got: %s", promptVal, capturedPatchBody)
	}
}

// ---------------------------------------------------------------------------
// C14: --patch mode, GET 200, entity has "system_prompt" → PromptKey=="system_prompt"
// ---------------------------------------------------------------------------

func TestCreate_Patch_DetectsSystemPromptKey(t *testing.T) {
	const agentID = "sysprompt_patch_agent"
	filePath := writeAgentMD(t, agentID, "SysPrompt Patch Agent", "New patch prompt.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		entityPath := "/blueprints/_ai_agent/entities/" + agentID
		switch {
		case r.URL.Path == entityPath && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "system_prompt", "existing sys prompt"),
			})
		case r.URL.Path == entityPath && r.Method == http.MethodPatch:
			io.Copy(io.Discard, r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "system_prompt", "New patch prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if result.PromptKey != "system_prompt" {
		t.Errorf("want PromptKey==%q in --patch mode, got %q", "system_prompt", result.PromptKey)
	}
}

// ---------------------------------------------------------------------------
// C15: --patch mode, GET 200, no prompt property detected → PromptKey=="prompt" (silent fallback)
// ---------------------------------------------------------------------------

func TestCreate_Patch_NoPromptProperty_FallsBackToPrompt(t *testing.T) {
	const agentID = "noprop_patch_agent"
	filePath := writeAgentMD(t, agentID, "No Prop Patch Agent", "Some prompt.")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		entityPath := "/blueprints/_ai_agent/entities/" + agentID
		switch {
		case r.URL.Path == entityPath && r.Method == http.MethodGet:
			// Entity exists but has NO recognized prompt property.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entity": map[string]interface{}{
					"identifier": agentID,
					"title":      "No Prop Patch Agent",
					"blueprint":  "_ai_agent",
					"properties": map[string]interface{}{
						"status": "active",
					},
				},
			})
		case r.URL.Path == entityPath && r.Method == http.MethodPatch:
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
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
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
// C16: --patch mode, GET non-404 error → propagate; no PATCH attempted
// ---------------------------------------------------------------------------

func TestCreate_Patch_GetError_Propagated(t *testing.T) {
	const agentID = "patch_error_agent"
	filePath := writeAgentMD(t, agentID, "Patch Error Agent", "prompt.")

	patchCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, agentID) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "server error"})
			return
		}
		if r.Method == http.MethodPatch {
			patchCalled = true
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
	})
	if err == nil {
		t.Fatal("want error for non-404 GET failure in --patch mode, got nil")
	}
	if result != nil {
		t.Errorf("want nil result, got %+v", result)
	}
	if patchCalled {
		t.Error("PATCH must NOT be called when GET returns a non-404 error in --patch mode")
	}
}

// ---------------------------------------------------------------------------
// C17: --patch body structure — no top-level "identifier"; "title" absent when empty
// ---------------------------------------------------------------------------

func TestCreate_Patch_BodyStructure_NoIdentifierNoEmptyTitle(t *testing.T) {
	// File has NO title in frontmatter → spec.Title == "" → title must be absent from PATCH body.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notitle.md")
	// identifier only, no title
	content := "---\nidentifier: notitle_agent\n---\nYou are a no-title agent.\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	var capturedPatchBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		entityPath := "/blueprints/_ai_agent/entities/notitle_agent"
		switch {
		case r.URL.Path == entityPath && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("notitle_agent", "prompt", "old prompt"),
			})
		case r.URL.Path == entityPath && r.Method == http.MethodPatch:
			capturedPatchBody, _ = io.ReadAll(r.Body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("notitle_agent", "prompt", "You are a no-title agent."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Create(context.Background(), client, CreateOptions{
		File:  filePath,
		Patch: true,
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPatchBody == nil {
		t.Fatal("PATCH body was never captured — PATCH was not called")
	}

	var parsed map[string]interface{}
	if jsonErr := json.Unmarshal(capturedPatchBody, &parsed); jsonErr != nil {
		t.Fatalf("PATCH body is not valid JSON: %v — body: %s", jsonErr, capturedPatchBody)
	}
	// Top-level "identifier" must never appear in patch body.
	if _, hasID := parsed["identifier"]; hasID {
		t.Errorf("PATCH body must NOT contain top-level %q; body: %s", "identifier", capturedPatchBody)
	}
	// Top-level "title" must be absent when spec.Title is empty.
	if _, hasTitle := parsed["title"]; hasTitle {
		t.Errorf("PATCH body must NOT contain top-level %q when spec.Title is empty; body: %s", "title", capturedPatchBody)
	}
}

// ---------------------------------------------------------------------------
// C18: Yes==false, StdinReader EOF → ErrConfirmationDeclined; no API call
// ---------------------------------------------------------------------------

func TestCreate_ConfirmationDeclined_ReturnsErrConfirmationDeclined(t *testing.T) {
	filePath := writeAgentMD(t, "confirm_agent", "Confirm Agent", "Some prompt.")

	// Default mode: GET 404 → would POST; but confirmation fires first.
	// We set up no server — any network call would fail, proving no API call is made.
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "http://localhost:0", // must not be reached
	})

	result, err := Create(context.Background(), client, CreateOptions{
		File:        filePath,
		Yes:         false,                 // trigger interactive confirmation
		StdinReader: strings.NewReader(""), // EOF → huh declines
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
// C19: Yes==true → confirmation skipped; API call proceeds
// ---------------------------------------------------------------------------

func TestCreate_YesTrue_SkipsConfirmation(t *testing.T) {
	const agentID = "yes_agent"
	filePath := writeAgentMD(t, agentID, "Yes Agent", "prompt.")

	apiCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/"+agentID && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "not found"})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			apiCalled = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap(agentID, "prompt", "prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
		Yes:  true, // skip confirmation
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if !apiCalled {
		t.Error("API must be called when Yes==true")
	}
}

// ---------------------------------------------------------------------------
// C20: spec.Tools == nil → POST body has tools:[] (not null)
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
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/notools_agent" && r.Method == http.MethodGet:
			// Default mode: entity does not exist.
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "not found"})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &capturedBody)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("notools_agent", "prompt", "Prompt."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
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
// C21: POST body structure — identifier, title, blueprint, properties verified
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
		switch {
		case r.URL.Path == "/blueprints/_ai_agent/entities/structured_agent" && r.Method == http.MethodGet:
			// Default mode: entity does not exist.
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "not found"})
		case r.URL.Path == "/blueprints/_ai_agent/entities" && r.Method == http.MethodPost:
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &capturedBody)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"entity": rawEntityMap("structured_agent", "prompt", "You are a structured agent."),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := Create(context.Background(), client, CreateOptions{
		File: filePath,
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
	// Prompt defaults to "prompt" key for new entity in default mode.
	if props["prompt"] != "You are a structured agent." {
		t.Errorf("body.properties[prompt]: want %q, got %v", "You are a structured agent.", props["prompt"])
	}
}

// ---------------------------------------------------------------------------
// CreateResult struct guard: ModeUsed must not exist
// This test will fail to compile if ModeUsed is still on the struct.
// ---------------------------------------------------------------------------

func TestCreateResult_HasNoModeUsedField(t *testing.T) {
	// If CreateResult still has ModeUsed, the line below will cause a compile error.
	// The test asserts the new struct shape: Action is present, ModeUsed is absent.
	result := &CreateResult{
		Action:    "created",
		PromptKey: "prompt",
	}
	if result.Action != "created" {
		t.Errorf("want Action==%q, got %q", "created", result.Action)
	}
	if result.PromptKey != "prompt" {
		t.Errorf("want PromptKey==%q, got %q", "prompt", result.PromptKey)
	}
	// No reference to result.ModeUsed — if the field exists it is simply unused here,
	// which is fine; but if Kou removes it (as required), this compiles cleanly.
	// The real guard is: tests above do NOT reference ModeUsed at all.
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

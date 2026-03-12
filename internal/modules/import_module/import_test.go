package import_module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestApplyDataExclusion_Deep(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service"},
			{"identifier": "_rule_result"},
		},
		Entities: []api.Entity{
			{"identifier": "e1", "blueprint": "service"},
			{"identifier": "e2", "blueprint": "_rule_result"},
		},
		Scorecards: []api.Scorecard{
			{"identifier": "sc1", "blueprintIdentifier": "_rule_result"},
		},
		Actions: []api.Action{
			{"identifier": "a1", "blueprint": "_rule_result"},
			{"identifier": "a2", "blueprint": "service"},
		},
		BlueprintPermissions: map[string]api.Permissions{
			"_rule_result": {"read": []string{"everyone"}},
			"service":      {"read": []string{"everyone"}},
		},
		ActionPermissions: map[string]api.Permissions{
			"a1": {"execute": []string{"everyone"}},
			"a2": {"execute": []string{"everyone"}},
		},
	}

	applyDataExclusion(data, []string{"_rule_result"}, nil)

	if len(data.Blueprints) != 1 {
		t.Errorf("expected 1 blueprint, got %d", len(data.Blueprints))
	}
	if len(data.Entities) != 1 {
		t.Errorf("expected 1 entity (deep removes resources too), got %d", len(data.Entities))
	}
	if len(data.Scorecards) != 0 {
		t.Errorf("expected 0 scorecards, got %d", len(data.Scorecards))
	}
	if len(data.Actions) != 1 {
		t.Errorf("expected 1 action (only non-excluded blueprint action kept), got %d", len(data.Actions))
	}
	if _, ok := data.BlueprintPermissions["_rule_result"]; ok {
		t.Error("expected BlueprintPermissions entry for excluded blueprint '_rule_result' to be removed")
	}
	if _, ok := data.BlueprintPermissions["service"]; !ok {
		t.Error("expected BlueprintPermissions entry for non-excluded blueprint 'service' to be present")
	}
	if _, ok := data.ActionPermissions["a1"]; ok {
		t.Error("expected ActionPermissions entry for excluded action 'a1' to be removed")
	}
	if _, ok := data.ActionPermissions["a2"]; !ok {
		t.Error("expected ActionPermissions entry for non-excluded action 'a2' to be present")
	}
}

func TestApplyDataExclusion_SchemaOnly(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service"},
			{"identifier": "_rule_result"},
		},
		Entities: []api.Entity{
			{"identifier": "e1", "blueprint": "service"},
			{"identifier": "e2", "blueprint": "_rule_result"},
		},
		Scorecards: []api.Scorecard{
			{"identifier": "sc1", "blueprintIdentifier": "_rule_result"},
		},
		Actions: []api.Action{
			{"identifier": "a1", "blueprint": "_rule_result"},
		},
	}

	applyDataExclusion(data, nil, []string{"_rule_result"})

	if len(data.Blueprints) != 1 {
		t.Errorf("expected 1 blueprint (schema removed), got %d", len(data.Blueprints))
	}
	// Schema-only: entities/scorecards/actions for _rule_result are KEPT
	if len(data.Entities) != 2 {
		t.Errorf("expected 2 entities (schema-only keeps resources), got %d", len(data.Entities))
	}
	if len(data.Scorecards) != 1 {
		t.Errorf("expected 1 scorecard (schema-only keeps resources), got %d", len(data.Scorecards))
	}
	if len(data.Actions) != 1 {
		t.Errorf("expected 1 action (schema-only keeps resources), got %d", len(data.Actions))
	}
}

func TestApplyDataExclusion_NoExclude(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{{"identifier": "service"}},
		Entities:   []api.Entity{{"identifier": "e1", "blueprint": "service"}},
	}
	applyDataExclusion(data, nil, nil)
	if len(data.Blueprints) != 1 || len(data.Entities) != 1 {
		t.Error("empty exclusion lists should leave data unchanged")
	}
}

func TestIsSidebarParentNotFound(t *testing.T) {
	cases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some other error"), false},
		{errors.New(`{"error":"not_found","message":"Sidebar item with parent \"initiatives\" was not found"}`), true},
		{errors.New("Sidebar item not found"), true},
	}
	for _, c := range cases {
		got := isSidebarParentNotFound(c.err)
		if got != c.expected {
			t.Errorf("isSidebarParentNotFound(%v) = %v, want %v", c.err, got, c.expected)
		}
	}
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := api.NewClient("id", "secret", srv.URL, 0)
	return srv, client
}

func authHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/auth/access_token" {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		return true
	}
	return false
}

// TestImportPages_PreservesTypeOnCreate verifies that `type` and navigation fields are
// sent to Port when creating a new page.
func TestImportPages_PreservesTypeOnCreate(t *testing.T) {
	var receivedPage map[string]interface{}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/pages" {
			json.NewDecoder(r.Body).Decode(&receivedPage)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": receivedPage})
			return
		}
		http.NotFound(w, r)
	})

	page := api.Page{
		"identifier":          "aws_cost_overview",
		"type":                "dashboard",
		"parent":              "initiatives",
		"sidebar":             "catalog",
		"after":               "mastering_the_estate",
		"requiredQueryParams": []interface{}{},
		"title":               "AWS Cost Overview",
		"widgets":             []interface{}{},
		"createdBy":           "user_abc",
		"createdAt":           "2026-01-01",
		"id":                  "internal-id",
	}

	importer := NewImporter(client)
	pool := NewWorkerPool(1)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result, pool)
	pool.Wait()

	if result.PagesCreated != 1 {
		t.Fatalf("expected 1 page created, got %d", result.PagesCreated)
	}
	if receivedPage["type"] != "dashboard" {
		t.Errorf("expected type=dashboard to be sent on create, got %v", receivedPage["type"])
	}
	if receivedPage["parent"] != "initiatives" {
		t.Errorf("expected parent=initiatives to be sent on create, got %v", receivedPage["parent"])
	}
	if receivedPage["sidebar"] != "catalog" {
		t.Errorf("expected sidebar=catalog to be sent on create, got %v", receivedPage["sidebar"])
	}
	// System/audit fields must be stripped
	if receivedPage["createdBy"] != nil {
		t.Errorf("expected createdBy to be stripped, got %v", receivedPage["createdBy"])
	}
}

// TestImportPages_UpdateSendsNavFields verifies that page updates include navigation
// fields (after, parent, sidebar) so Port moves the page to the correct sidebar position,
// and that `type` is stripped because the PATCH endpoint rejects it.
func TestImportPages_UpdateSendsNavFields(t *testing.T) {
	postCalls := 0
	var patchBody map[string]interface{}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/pages" {
			postCalls++
			// Always return conflict so the importer falls through to update.
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": false, "error": "conflict",
			})
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/pages/aws_cost_overview" {
			json.NewDecoder(r.Body).Decode(&patchBody)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": patchBody})
			return
		}
		// GetPage — return empty existing page so agentIdentifier merge is a no-op.
		if r.Method == http.MethodGet && r.URL.Path == "/pages/aws_cost_overview" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{"identifier": "aws_cost_overview"}})
			return
		}
		http.NotFound(w, r)
	})

	page := api.Page{
		"identifier": "aws_cost_overview",
		"type":       "dashboard",
		"parent":     "initiatives",
		"sidebar":    "catalog",
		"after":      "mastering_the_estate",
		"title":      "AWS Cost Overview",
		"widgets":    []interface{}{},
		"createdBy":  "user_abc",
	}

	importer := NewImporter(client)
	pool := NewWorkerPool(1)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result, pool)
	pool.Wait()

	if result.PagesUpdated != 1 {
		t.Fatalf("expected 1 page updated, got %d (created=%d)", result.PagesUpdated, result.PagesCreated)
	}
	// Navigation fields must be present in the PATCH body.
	if patchBody["parent"] != "initiatives" {
		t.Errorf("expected parent=initiatives in update, got %v", patchBody["parent"])
	}
	if patchBody["sidebar"] != "catalog" {
		t.Errorf("expected sidebar=catalog in update, got %v", patchBody["sidebar"])
	}
	// `after` is withheld from the concurrent PATCH and applied by applyPageOrdering.
	if _, exists := patchBody["after"]; exists {
		t.Errorf("expected after to be withheld from concurrent update (applied by ordering pass), got %v", patchBody["after"])
	}
	// type must be stripped from PATCH.
	if patchBody["type"] != nil {
		t.Errorf("expected type to be stripped from update, got %v", patchBody["type"])
	}
	// Audit fields must be stripped.
	if patchBody["createdBy"] != nil {
		t.Errorf("expected createdBy to be stripped from update, got %v", patchBody["createdBy"])
	}
}

// TestImportPages_UpdateFallsBackWithoutNavWhenParentMissing verifies that when Port
// rejects a page update because the parent doesn't exist, we retry without nav fields.
func TestImportPages_UpdateFallsBackWithoutNavWhenParentMissing(t *testing.T) {
	patchCalls := 0
	var secondPatchBody map[string]interface{}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/pages" {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "conflict"})
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/pages/aws_cost_overview" {
			patchCalls++
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if patchCalls == 1 {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":      false,
					"error":   "not_found",
					"message": `Sidebar item with parent "initiatives" was not found`,
				})
				return
			}
			secondPatchBody = body
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": body})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/pages/aws_cost_overview" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{"identifier": "aws_cost_overview"}})
			return
		}
		http.NotFound(w, r)
	})

	page := api.Page{
		"identifier": "aws_cost_overview",
		"type":       "dashboard",
		"parent":     "initiatives",
		"sidebar":    "catalog",
		"title":      "AWS Cost Overview",
		"widgets":    []interface{}{},
	}

	importer := NewImporter(client)
	pool := NewWorkerPool(1)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result, pool)
	pool.Wait()

	if patchCalls != 2 {
		t.Fatalf("expected 2 PATCH attempts, got %d", patchCalls)
	}
	if result.PagesUpdated != 1 {
		t.Fatalf("expected 1 page updated, got %d", result.PagesUpdated)
	}
	// Navigation fields must be stripped on the fallback PATCH.
	if secondPatchBody["parent"] != nil {
		t.Errorf("expected parent to be stripped on fallback update, got %v", secondPatchBody["parent"])
	}
	if secondPatchBody["sidebar"] != nil {
		t.Errorf("expected sidebar to be stripped on fallback update, got %v", secondPatchBody["sidebar"])
	}
}

// TestImportPages_FallsBackWithoutNavWhenParentMissing verifies that when Port rejects
// a page creation because the parent page doesn't exist, we retry without navigation
// fields but still send `type` so the page renders correctly.
func TestImportPages_FallsBackWithoutNavWhenParentMissing(t *testing.T) {
	calls := 0
	var secondCallPage map[string]interface{}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/pages" {
			calls++
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if calls == 1 {
				// First attempt: reject because parent doesn't exist
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":      false,
					"error":   "not_found",
					"message": `Sidebar item with parent "initiatives" was not found`,
				})
				return
			}
			// Second attempt: accept
			secondCallPage = body
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": body})
			return
		}
		http.NotFound(w, r)
	})

	page := api.Page{
		"identifier": "aws_cost_overview",
		"type":       "dashboard",
		"parent":     "initiatives",
		"sidebar":    "catalog",
		"title":      "AWS Cost Overview",
		"widgets":    []interface{}{},
	}

	importer := NewImporter(client)
	pool := NewWorkerPool(1)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result, pool)
	pool.Wait()

	if calls != 2 {
		t.Fatalf("expected 2 create attempts, got %d", calls)
	}
	if result.PagesCreated != 1 {
		t.Fatalf("expected 1 page created, got %d", result.PagesCreated)
	}
	// type must still be present on retry
	if secondCallPage["type"] != "dashboard" {
		t.Errorf("expected type=dashboard on fallback create, got %v", secondCallPage["type"])
	}
	// navigation fields must be stripped on retry
	if secondCallPage["parent"] != nil {
		t.Errorf("expected parent to be stripped on fallback create, got %v", secondCallPage["parent"])
	}
	if secondCallPage["sidebar"] != nil {
		t.Errorf("expected sidebar to be stripped on fallback create, got %v", secondCallPage["sidebar"])
	}
}

// TestImportPages_NullNavFieldsNotSentOnUpdate verifies that when the source page has
// null nav fields (e.g. exported from an org where those fields weren't captured),
// the PATCH request does NOT include those null fields — sending null would clear the
// page's existing navigation context in Port.
func TestImportPages_NullNavFieldsNotSentOnUpdate(t *testing.T) {
	var patchBody map[string]interface{}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/pages" {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "conflict"})
			return
		}
		if r.Method == http.MethodPatch && r.URL.Path == "/pages/aws_cost_overview" {
			json.NewDecoder(r.Body).Decode(&patchBody)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": patchBody})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/pages/aws_cost_overview" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{"identifier": "aws_cost_overview"}})
			return
		}
		http.NotFound(w, r)
	})

	// Source page has null for all nav fields (common in exports from orgs that don't capture them)
	page := api.Page{
		"identifier":          "aws_cost_overview",
		"type":                nil, // null
		"parent":              nil, // null
		"sidebar":             nil, // null
		"after":               nil, // null
		"requiredQueryParams": nil, // null
		"title":               "AWS Cost Overview",
		"widgets":             []interface{}{},
	}

	importer := NewImporter(client)
	pool := NewWorkerPool(1)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result, pool)
	pool.Wait()

	if result.PagesUpdated != 1 {
		t.Fatalf("expected 1 page updated, got %d", result.PagesUpdated)
	}
	// Null string nav fields must NOT be sent in PATCH (would clear existing values)
	if _, exists := patchBody["parent"]; exists {
		t.Errorf("expected null parent to be stripped from PATCH, got %v", patchBody["parent"])
	}
	if _, exists := patchBody["sidebar"]; exists {
		t.Errorf("expected null sidebar to be stripped from PATCH, got %v", patchBody["sidebar"])
	}
	if _, exists := patchBody["after"]; exists {
		t.Errorf("expected null after to be stripped from PATCH, got %v", patchBody["after"])
	}
	// requiredQueryParams: null must be normalized to [] (not stripped, not sent as null)
	if v, exists := patchBody["requiredQueryParams"]; !exists {
		t.Errorf("expected requiredQueryParams to be sent as [] in PATCH, but it was missing")
	} else {
		arr, ok := v.([]interface{})
		if !ok || len(arr) != 0 {
			t.Errorf("expected requiredQueryParams=[] in PATCH, got %v", v)
		}
	}
	// type is always stripped from PATCH regardless
	if _, exists := patchBody["type"]; exists {
		t.Errorf("expected type to be stripped from PATCH, got %v", patchBody["type"])
	}
}

// TestSortPagesByAfterDeps verifies topological sort respects after-dependencies.
func TestSortPagesByAfterDeps(t *testing.T) {
	// Chain: alpha <- beta <- gamma (beta after alpha, gamma after beta)
	pages := []api.Page{
		{"identifier": "gamma", "after": "beta"},
		{"identifier": "alpha"},
		{"identifier": "beta", "after": "alpha"},
	}
	sorted := sortPagesByAfterDeps(pages)

	// Build position map
	pos := make(map[string]int)
	for i, p := range sorted {
		pos[p["identifier"].(string)] = i
	}

	if pos["alpha"] >= pos["beta"] {
		t.Errorf("expected alpha before beta, got alpha=%d beta=%d", pos["alpha"], pos["beta"])
	}
	if pos["beta"] >= pos["gamma"] {
		t.Errorf("expected beta before gamma, got beta=%d gamma=%d", pos["beta"], pos["gamma"])
	}
}

// TestApplyPageOrdering verifies that after the concurrent update pass, applyPageOrdering
// sends sequential PATCH requests with only the `after` field in topological order.
func TestApplyPageOrdering(t *testing.T) {
	var patchOrder []string
	var patchBodies []map[string]interface{}
	var mu sync.Mutex

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		if r.Method == http.MethodPatch {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			patchOrder = append(patchOrder, r.URL.Path)
			patchBodies = append(patchBodies, body)
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": body})
			return
		}
		http.NotFound(w, r)
	})

	// gamma depends on beta, beta depends on alpha
	pages := []api.Page{
		{"identifier": "gamma", "after": "beta", "title": "Gamma"},
		{"identifier": "alpha", "title": "Alpha"},
		{"identifier": "beta", "after": "alpha", "title": "Beta"},
	}

	importer := NewImporter(client)
	result := &Result{}
	importer.applyPageOrdering(context.Background(), pages, result)

	// alpha has no after so it's skipped; beta and gamma must be patched in order
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Find positions of beta and gamma in patch order
	betaIdx, gammaIdx := -1, -1
	for i, path := range patchOrder {
		switch path {
		case "/pages/beta":
			betaIdx = i
		case "/pages/gamma":
			gammaIdx = i
		}
	}
	if betaIdx == -1 || gammaIdx == -1 {
		t.Fatalf("expected patches for beta and gamma, got order: %v", patchOrder)
	}
	if betaIdx >= gammaIdx {
		t.Errorf("expected beta patched before gamma (topo order), got beta=%d gamma=%d", betaIdx, gammaIdx)
	}

	// Each patch body must contain only the after field (plus identifier)
	for i, body := range patchBodies {
		path := patchOrder[i]
		if _, hasAfter := body["after"]; !hasAfter {
			t.Errorf("patch to %s missing after field", path)
		}
		if _, hasTitle := body["title"]; hasTitle {
			t.Errorf("patch to %s should not include title (content fields should be in main pass)", path)
		}
	}
}

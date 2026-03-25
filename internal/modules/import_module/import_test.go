package import_module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
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
	client := api.NewClient(nil, "id", "secret", srv.URL, 0)
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
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result)

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
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result)

	if result.PagesUpdated != 1 {
		t.Fatalf("expected 1 page updated, got %d (created=%d)", result.PagesUpdated, result.PagesCreated)
	}
	// Relevant placement fields must be present in the PATCH body.
	if patchBody["parent"] != "initiatives" {
		t.Errorf("expected parent=initiatives in update, got %v", patchBody["parent"])
	}
	if patchBody["sidebar"] != nil {
		t.Errorf("expected sidebar to be stripped from update, got %v", patchBody["sidebar"])
	}
	// `after` must be present in the PATCH body (ordering is applied inline, not in a second pass).
	if patchBody["after"] != "mastering_the_estate" {
		t.Errorf("expected after=mastering_the_estate in update, got %v", patchBody["after"])
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

// TestImportPages_UpdateFallsBackWithoutAfterWhenSiblingMissing verifies that when
// Port rejects a page update because the `after` sibling is missing, we retry
// without only the `after` field while preserving parent placement.
func TestImportPages_UpdateFallsBackWithoutAfterWhenSiblingMissing(t *testing.T) {
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
					"message": `Sidebar item with after "mastering_the_estate" was not found`,
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
		"after":      "mastering_the_estate",
		"title":      "AWS Cost Overview",
		"widgets":    []interface{}{},
	}

	importer := NewImporter(client)
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result)

	if patchCalls != 2 {
		t.Fatalf("expected 2 PATCH attempts, got %d", patchCalls)
	}
	if result.PagesUpdated != 1 {
		t.Fatalf("expected 1 page updated, got %d", result.PagesUpdated)
	}
	// Only the invalid `after` field should be stripped on the fallback PATCH.
	if secondPatchBody["after"] != nil {
		t.Errorf("expected after to be stripped on fallback update, got %v", secondPatchBody["after"])
	}
	if secondPatchBody["parent"] != "initiatives" {
		t.Errorf("expected parent to be preserved on fallback update, got %v", secondPatchBody["parent"])
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
	result := &Result{}
	importer.importPages(context.Background(), []api.Page{page}, result)

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
	// requiredQueryParams: null must be stripped from PATCH (not sent as null or []).
	if _, exists := patchBody["requiredQueryParams"]; exists {
		t.Errorf("expected null requiredQueryParams to be stripped from PATCH, got %v", patchBody["requiredQueryParams"])
	}
	// type is always stripped from PATCH regardless
	if _, exists := patchBody["type"]; exists {
		t.Errorf("expected type to be stripped from PATCH, got %v", patchBody["type"])
	}
}

func TestImportBlueprints_RestoresOwnershipAfterCreate(t *testing.T) {
	var createBody map[string]interface{}
	var ownershipPatchBody map[string]interface{}
	var mu sync.Mutex

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/blueprints":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprint": createBody})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "service"},
				},
			})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints/service":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprint": map[string]interface{}{
					"identifier": "service",
					"title":      "Service",
					"relations": map[string]interface{}{
						"system": map[string]interface{}{"target": "system"},
					},
				},
			})
			return
		case r.Method == http.MethodPut && r.URL.Path == "/blueprints/service":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}

			mu.Lock()
			if ownership, ok := body["ownership"].(map[string]interface{}); ok && ownership["type"] == "Inherited" {
				ownershipPatchBody = body
			}
			mu.Unlock()

			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprint": body})
			return
		default:
			http.NotFound(w, r)
		}
	})

	importer := NewImporter(client)
	result := &Result{}
	blueprints := []api.Blueprint{
		{
			"identifier": "service",
			"title":      "Service",
			"relations": map[string]interface{}{
				"system": map[string]interface{}{"target": "system"},
			},
			"ownership": map[string]interface{}{
				"type": "Inherited",
				"path": "system.$identifier",
			},
		},
	}

	if err := importer.importBlueprints(context.Background(), blueprints, result); err != nil {
		t.Fatalf("importBlueprints returned error: %v", err)
	}

	if createBody["ownership"] != nil {
		t.Fatalf("expected ownership to be deferred during create, got %v", createBody["ownership"])
	}
	if ownershipPatchBody == nil {
		t.Fatal("expected ownership to be restored in a later blueprint update")
	}

	ownership, ok := ownershipPatchBody["ownership"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ownership in update body, got %T", ownershipPatchBody["ownership"])
	}
	if ownership["type"] != "Inherited" {
		t.Fatalf("expected ownership type Inherited, got %v", ownership["type"])
	}
	if ownership["path"] != "system.$identifier" {
		t.Fatalf("expected ownership path system.$identifier, got %v", ownership["path"])
	}
}

func TestImportBlueprints_AppliesOwnershipInDependencyOrder(t *testing.T) {
	var mu sync.Mutex
	var ownershipUpdateOrder []string

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/blueprints":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprint": body})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "service"},
					{"identifier": "deployment"},
					{"identifier": "pod"},
				},
			})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints/service":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":        true,
				"blueprint": map[string]interface{}{"identifier": "service", "title": "Service"},
			})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints/deployment":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprint": map[string]interface{}{
					"identifier": "deployment",
					"title":      "Deployment",
					"relations": map[string]interface{}{
						"service": map[string]interface{}{"target": "service"},
					},
				},
			})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints/pod":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprint": map[string]interface{}{
					"identifier": "pod",
					"title":      "Pod",
					"relations": map[string]interface{}{
						"deployment": map[string]interface{}{"target": "deployment"},
					},
				},
			})
			return
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/blueprints/"):
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			if _, ok := body["ownership"].(map[string]interface{}); ok {
				mu.Lock()
				ownershipUpdateOrder = append(ownershipUpdateOrder, body["identifier"].(string))
				mu.Unlock()
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprint": body})
			return
		default:
			http.NotFound(w, r)
		}
	})

	importer := NewImporter(client)
	result := &Result{}
	blueprints := []api.Blueprint{
		{
			"identifier": "service",
			"title":      "Service",
			"ownership":  map[string]interface{}{"type": "Direct"},
		},
		{
			"identifier": "deployment",
			"title":      "Deployment",
			"relations": map[string]interface{}{
				"service": map[string]interface{}{"target": "service"},
			},
			"ownership": map[string]interface{}{
				"type": "Inherited",
				"path": "service.$identifier",
			},
		},
		{
			"identifier": "pod",
			"title":      "Pod",
			"relations": map[string]interface{}{
				"deployment": map[string]interface{}{"target": "deployment"},
			},
			"ownership": map[string]interface{}{
				"type": "Inherited",
				"path": "deployment.$identifier",
			},
		},
	}

	if err := importer.importBlueprints(context.Background(), blueprints, result); err != nil {
		t.Fatalf("importBlueprints returned error: %v", err)
	}

	if len(ownershipUpdateOrder) != 3 {
		t.Fatalf("expected 3 ownership updates, got %d (%v)", len(ownershipUpdateOrder), ownershipUpdateOrder)
	}

	expectedOrder := []string{"service", "deployment", "pod"}
	for i, id := range expectedOrder {
		if ownershipUpdateOrder[i] != id {
			t.Fatalf("expected ownership update %d to be %s, got %s", i, id, ownershipUpdateOrder[i])
		}
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

// TestImportPages_OrderingRespectedInline verifies that importPages processes pages
// in topological `after` order so that `after` targets exist before dependents.
func TestImportPages_OrderingRespectedInline(t *testing.T) {
	var mu sync.Mutex
	var patchOrder []string

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}
		// All pages "already exist" — POST returns 409 conflict → update path
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "conflict"})
			return
		}
		if r.Method == http.MethodGet {
			pageID := r.URL.Path[len("/pages/"):]
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{"identifier": pageID}})
			return
		}
		if r.Method == http.MethodPatch {
			mu.Lock()
			patchOrder = append(patchOrder, r.URL.Path)
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{}})
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
	importer.importPages(context.Background(), pages, result)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// alpha has no dependency — it must come before beta, beta before gamma
	pos := make(map[string]int)
	for i, path := range patchOrder {
		switch path {
		case "/pages/alpha":
			pos["alpha"] = i
		case "/pages/beta":
			pos["beta"] = i
		case "/pages/gamma":
			pos["gamma"] = i
		}
	}
	if pos["alpha"] >= pos["beta"] {
		t.Errorf("expected alpha before beta, got alpha=%d beta=%d", pos["alpha"], pos["beta"])
	}
	if pos["beta"] >= pos["gamma"] {
		t.Errorf("expected beta before gamma, got beta=%d gamma=%d", pos["beta"], pos["gamma"])
	}
}

func TestImportFolders_CreatedBeforePages(t *testing.T) {
	var calls []string
	var folderPayloads []map[string]interface{}
	var mu sync.Mutex

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHandler(w, r) {
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sidebars/catalog/folders":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode folder body: %v", err)
			}
			mu.Lock()
			calls = append(calls, "folder")
			folderPayloads = append(folderPayloads, body)
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
			return
		case r.Method == http.MethodPost && r.URL.Path == "/pages":
			mu.Lock()
			calls = append(calls, "page")
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "page": map[string]interface{}{"identifier": "service_overview"}})
			return
		default:
			http.NotFound(w, r)
		}
	})

	importer := NewImporter(client)
	result := &Result{}
	data := &export.Data{
		Blueprints: []api.Blueprint{{"identifier": "service", "title": "Service"}},
		Folders: []api.Folder{
			{"identifier": "root", "title": "Root"},
			{"identifier": "child", "title": "Child", "parent": "root"},
		},
		Pages: []api.Page{{"identifier": "service_overview", "title": "Service Overview", "parent": "root"}},
	}

	result = &Result{}
	if err := importer.importOtherResources(context.Background(), data, Options{IncludeResources: []string{"pages"}}, result); err != nil {
		t.Fatalf("importOtherResources error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d (%v)", len(calls), calls)
	}
	if calls[0] != "folder" {
		t.Fatalf("expected the root folder to be created before any page, got %v", calls)
	}
	if len(folderPayloads) != 2 {
		t.Fatalf("expected 2 folder payloads, got %d", len(folderPayloads))
	}
	if folderPayloads[0]["identifier"] != "root" {
		t.Fatalf("expected root folder first, got %v", folderPayloads[0]["identifier"])
	}
	if folderPayloads[1]["identifier"] != "child" {
		t.Fatalf("expected child folder to be created, got %v", folderPayloads[1]["identifier"])
	}
	if folderPayloads[1]["parent"] != "root" {
		t.Fatalf("expected child folder payload to preserve parent=root, got %v", folderPayloads[1]["parent"])
	}
	if result.PagesCreated != 1 {
		t.Fatalf("expected 1 page created, got %d", result.PagesCreated)
	}
}

func createTempConfig(t *testing.T) *config.ConfigManager {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	return config.NewConfigManager(configPath)
}

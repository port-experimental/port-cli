package migrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
)

func TestExportFromSource_SkipEntities_SkipsTeamsAndUsers(t *testing.T) {
	teamsHit := false
	usersHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/teams":
			teamsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "teams": []interface{}{}})
		case "/users":
			usersHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	opts := Options{SkipEntities: true}
	_, err := m.exportFromSource(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsHit {
		t.Error("teams endpoint should not be called when SkipEntities=true")
	}
	if usersHit {
		t.Error("users endpoint should not be called when SkipEntities=true")
	}
}

func TestExportFromSource_SkipSystemBlueprints_ExcludesSchemaAndEntities(t *testing.T) {
	entitiesHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user"},
					{"identifier": "service"},
				},
			})
		case "/blueprints/_user/entities":
			entitiesHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	opts := Options{SkipSystemBlueprints: true}
	data, err := m.exportFromSource(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, bp := range data.Blueprints {
		id, _ := bp["identifier"].(string)
		if id == "_user" {
			t.Error("_user blueprint schema should be excluded from migrate data")
		}
	}
	if entitiesHit {
		t.Error("entities endpoint for _user should not be called in migrate when SkipSystemBlueprints=true")
	}
}

func TestMigrateOptionsHasExcludeFields(t *testing.T) {
	opts := Options{
		ExcludeBlueprints:      []string{"service"},
		ExcludeBlueprintSchema: []string{"region"},
	}
	if len(opts.ExcludeBlueprints) != 1 {
		t.Error("ExcludeBlueprints not set")
	}
	if len(opts.ExcludeBlueprintSchema) != 1 {
		t.Error("ExcludeBlueprintSchema not set")
	}
}

func TestExportFromSource_ActionPermissionsNotCollectedWhenExcluded(t *testing.T) {
	actionPermsHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":         true,
				"blueprints": []map[string]interface{}{{"identifier": "svc"}},
			})
		case "/blueprints/svc/actions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      true,
				"actions": []map[string]interface{}{{"identifier": "act1"}},
			})
		case "/actions/act1/permissions":
			actionPermsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "permissions": map[string]interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	opts := Options{IncludeResources: []string{"actions"}}
	_, err := m.exportFromSource(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actionPermsHit {
		t.Error("action permissions endpoint should not be called when action-permissions not in IncludeResources")
	}
}

func TestExportFromSource_ActionPermissionsFetchFailureRecordsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":         true,
				"blueprints": []map[string]interface{}{{"identifier": "svc"}},
			})
		case "/blueprints/svc/actions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      true,
				"actions": []map[string]interface{}{{"identifier": "act1"}},
			})
		case "/actions/act1/permissions":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	data, err := m.exportFromSource(context.Background(), Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Warnings) == 0 {
		t.Error("expected a warning when action permissions fetch fails")
	}
}

func TestExportFromSource_PagePermissionsFetchFailureRecordsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/pages":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    true,
				"pages": []map[string]interface{}{{"identifier": "home", "title": "Home"}},
			})
		case "/pages/home/permissions":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	data, err := m.exportFromSource(context.Background(), Options{SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Warnings) == 0 {
		t.Error("expected a warning when page permissions fetch fails")
	}
}

func TestExportFromSource_PagePermissionsNotCollectedWhenExcluded(t *testing.T) {
	pagePermsHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/pages":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    true,
				"pages": []map[string]interface{}{{"identifier": "home"}},
			})
		case "/pages/home/permissions":
			pagePermsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "permissions": map[string]interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}
	_, err := m.exportFromSource(context.Background(), Options{
		SkipEntities:     true,
		IncludeResources: []string{"pages"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pagePermsHit {
		t.Error("page permissions endpoint should not be called when page-permissions not in IncludeResources")
	}
}

func TestImportToTarget_PagePermissions_RetriesOnOrphanedFields(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/pages/home/permissions":
			if r.Method != "PATCH" {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
				return
			}
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":      false,
					"error":   "invalid_permissions",
					"message": "You cannot update permissions on unknown fields",
					"details": map[string]interface{}{
						"invalidProperties": []string{},
						"invalidRelations":  []string{"staleRel"},
					},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "permissions": map[string]interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	m := &Module{
		sourceClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
		targetClient: api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL}),
	}

	data := &export.Data{
		Blueprints:           []api.Blueprint{},
		Entities:             []api.Entity{},
		Scorecards:           []api.Scorecard{},
		Actions:              []api.Action{},
		Teams:                []api.Team{},
		Users:                []api.User{},
		Folders:              []api.Folder{},
		Pages:                []api.Page{},
		Integrations:         []api.Integration{},
		BlueprintPermissions: map[string]api.Permissions{},
		ActionPermissions:    map[string]api.Permissions{},
		PagePermissions:      map[string]api.Permissions{},
	}
	diff := &import_module.DiffResult{
		PagePermissions: []import_module.PermissionsChange{
			{
				Identifier: "home",
				Permissions: api.Permissions{
					"read":     map[string]interface{}{"roles": []string{"Admin"}},
					"staleRel": map[string]interface{}{"roles": []string{"Admin"}},
				},
			},
		},
	}

	result, err := m.importToTarget(context.Background(), data, diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PagePermissionsUpdated != 1 {
		t.Errorf("expected 1 page permission updated after retry, got %d", result.PagePermissionsUpdated)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (original + retry), got %d", callCount)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning about stripped fields, got %d: %v", len(result.Warnings), result.Warnings)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors after successful retry, got: %v", result.Errors)
	}
}

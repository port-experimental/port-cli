package migrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/plan"
)

func applyEquivalenceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case r.Method == http.MethodPost && r.URL.Path == "/blueprints":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprint": map[string]interface{}{}})
		case r.Method == http.MethodGet && r.URL.Path == "/blueprints":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/blueprints/"):
			id := strings.TrimPrefix(r.URL.Path, "/blueprints/")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":        true,
				"blueprint": map[string]interface{}{"identifier": id},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/teams":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "team": map[string]interface{}{}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}
}

type applyOutcome struct {
	BlueprintsCreated int
	BlueprintsUpdated int
	TeamsCreated      int
	TeamsUpdated      int
	Errors            []string
}

func outcomeFromImport(r *import_module.Result) applyOutcome {
	return applyOutcome{
		BlueprintsCreated: r.BlueprintsCreated,
		BlueprintsUpdated: r.BlueprintsUpdated,
		TeamsCreated:      r.TeamsCreated,
		TeamsUpdated:      r.TeamsUpdated,
		Errors:            append([]string(nil), r.Errors...),
	}
}

func outcomeFromMigrate(r *Result) applyOutcome {
	return applyOutcome{
		BlueprintsCreated: r.BlueprintsCreated,
		BlueprintsUpdated: r.BlueprintsUpdated,
		TeamsCreated:      r.TeamsCreated,
		TeamsUpdated:      r.TeamsUpdated,
		Errors:            append([]string(nil), r.Errors...),
	}
}

func TestMigrateApplyMatchesImportApplyFiltered(t *testing.T) {
	fixtureData := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service", "title": "Service"},
		},
		Teams: []api.Team{
			{"name": "platform"},
		},
		Entities:             []api.Entity{},
		Scorecards:           []api.Scorecard{},
		Actions:              []api.Action{},
		Users:                []api.User{},
		Folders:              []api.Folder{},
		Pages:                []api.Page{},
		Integrations:         []api.Integration{},
		BlueprintPermissions: map[string]api.Permissions{},
		ActionPermissions:    map[string]api.Permissions{},
		PagePermissions:      map[string]api.Permissions{},
	}
	diff := &import_module.DiffResult{
		BlueprintsToCreate: fixtureData.Blueprints,
		TeamsToCreate:      fixtureData.Teams,
	}
	executionPlan := plan.BuildFromDiffResult(diff)
	importOpts := import_module.Options{
		SkipEntities:     true,
		IncludeResources: []string{"blueprints", "teams"},
	}

	runImport := func(t *testing.T) applyOutcome {
		t.Helper()
		server := httptest.NewServer(applyEquivalenceHandler())
		t.Cleanup(server.Close)
		client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})
		t.Cleanup(func() { _ = client.Close() })

		importer := import_module.NewImporter(client)
		result, err := importer.ApplyFiltered(context.Background(), fixtureData, diff, importOpts)
		if err != nil {
			t.Fatalf("ApplyFiltered failed: %v", err)
		}
		return outcomeFromImport(result)
	}

	runMigrate := func(t *testing.T) applyOutcome {
		t.Helper()
		server := httptest.NewServer(applyEquivalenceHandler())
		t.Cleanup(server.Close)
		client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})
		t.Cleanup(func() { _ = client.Close() })

		m := &Module{targetClient: client}
		result, err := m.importToTarget(context.Background(), fixtureData, executionPlan, diff, Options{}, true)
		if err != nil {
			t.Fatalf("importToTarget failed: %v", err)
		}
		return outcomeFromMigrate(result)
	}

	importOutcome := runImport(t)
	migrateOutcome := runMigrate(t)

	if !reflect.DeepEqual(importOutcome, migrateOutcome) {
		t.Fatalf("apply outcomes differ:\n  import:  %#v\n  migrate: %#v", importOutcome, migrateOutcome)
	}
	if importOutcome.BlueprintsCreated != 1 {
		t.Fatalf("expected 1 blueprint created, got %#v", importOutcome)
	}
	if importOutcome.TeamsCreated != 1 {
		t.Fatalf("expected 1 team created, got %#v", importOutcome)
	}
}

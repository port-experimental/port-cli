package render

import (
	"strings"
	"testing"

	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
)

func TestApplyCountsFromImportMapsCounters(t *testing.T) {
	counts := ApplyCountsFromImport(&importmodule.Result{
		BlueprintsCreated:           2,
		ActionsUpdated:              1,
		IntegrationsUpdated:         3,
		BlueprintPermissionsUpdated: 4,
	})
	if counts.Blueprints.Created != 2 || counts.Actions.Updated != 1 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
	if counts.Integration.Updated != 3 || counts.BlueprintPermissionsUpdated != 4 {
		t.Fatalf("unexpected integration/permission counts: %#v", counts)
	}
}

func TestApplyCountsFromMigrateIncludesSkipped(t *testing.T) {
	counts := ApplyCountsFromMigrate(&migrate.Result{
		TeamsCreated:    1,
		TeamsSkipped:    2,
		PagesUpdated:    3,
		EntitiesSkipped: 4,
	})
	if counts.Teams.Created != 1 || counts.Teams.Skipped != 2 {
		t.Fatalf("unexpected team counts: %#v", counts.Teams)
	}
	if counts.Entities.Skipped != 4 {
		t.Fatalf("unexpected entity skip count: %#v", counts.Entities)
	}
}

func TestPopulateApplyCountsJSONStableKeys(t *testing.T) {
	data := map[string]interface{}{}
	PopulateApplyCountsJSON(data, ApplyCounts{
		Blueprints:                  ResourceCounts{Created: 1, Updated: 2, Skipped: 3},
		Integration:                 ResourceCounts{Updated: 4, Skipped: 5},
		BlueprintPermissionsUpdated: 6,
	}, true)

	for _, key := range []string{
		"blueprints_created", "blueprints_updated", "blueprints_skipped",
		"integrations_updated", "integrations_skipped",
		"blueprint_permissions_updated",
	} {
		if _, ok := data[key]; !ok {
			t.Fatalf("missing key %s in %#v", key, data)
		}
	}
	if data["blueprints_created"] != 1 || data["integrations_skipped"] != 5 {
		t.Fatalf("unexpected values: %#v", data)
	}
}

func TestPrintApplyCountsTextImportAndMigrateParity(t *testing.T) {
	importOut := captureOutput(t, func() {
		PrintApplyCountsText(ApplyCountsFromImport(&importmodule.Result{
			BlueprintsCreated:           1,
			BlueprintPermissionsUpdated: 2,
			ActionPermissionsUpdated:    3,
		}), false)
	})
	for _, want := range []string{
		"Blueprints created: 1, updated: 0",
		"Blueprint permissions updated: 2",
		"Action permissions updated: 3",
	} {
		if !strings.Contains(importOut, want) {
			t.Fatalf("import output missing %q:\n%s", want, importOut)
		}
	}

	migrateOut := captureOutput(t, func() {
		PrintApplyCountsText(ApplyCountsFromMigrate(&migrate.Result{
			PagesCreated:                1,
			PagesSkipped:                2,
			BlueprintPermissionsUpdated: 3,
		}), true)
	})
	for _, want := range []string{
		"Pages created: 1, updated: 0, skipped: 2",
		"Blueprint permissions updated: 3",
	} {
		if !strings.Contains(migrateOut, want) {
			t.Fatalf("migrate output missing %q:\n%s", want, migrateOut)
		}
	}
}

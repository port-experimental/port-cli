package commands

import (
	"testing"

	exportmodule "github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/render"
)

func TestExportJSONSummaryShapeContract(t *testing.T) {
	result := &exportmodule.Result{Success: true, Message: "ok", OutputPath: "backup.tar.gz", Format: "tar"}
	data := render.ExportJSONSummary(result, false, nil, nil, nil)
	for _, key := range []string{
		"output_path", "format", "blueprints_count", "entities_count", "actions_count",
		"users_count", "teams_count", "folders_count", "pages_count", "integrations_count",
		"skipped_entities", "included_resources", "excluded_blueprints", "schema_only_excluded",
	} {
		if _, ok := data[key]; !ok {
			t.Fatalf("export JSON summary missing key %q", key)
		}
	}
}

func TestMigrateJSONDetailShapeContract(t *testing.T) {
	data := map[string]interface{}{}
	render.AddMigrationDetailJSON(data, &migrate.Result{
		BlueprintsToCreate:           []string{"service"},
		BlueprintsToUpdate:           []string{"repo"},
		BlueprintsToSkip:             []string{"team"},
		BlueprintPermissionsToUpdate: []string{"service"},
		ActionPermissionsToUpdate:    []string{"deploy"},
		PagePermissionsToUpdate:      []string{"home"},
	})
	for _, key := range []string{
		"blueprints_to_create",
		"blueprints_to_update",
		"blueprints_skipped_ids",
		"blueprint_permissions_to_update",
		"action_permissions_to_update",
		"page_permissions_to_update",
	} {
		if _, ok := data[key]; !ok {
			t.Fatalf("migrate JSON details missing key %q", key)
		}
	}
}

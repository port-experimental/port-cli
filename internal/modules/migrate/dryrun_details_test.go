package migrate

import (
	"reflect"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
)

func TestGenerateDryRunResultIncludesIdentifiers(t *testing.T) {
	m := &Module{}
	result := m.generateDryRunResult(&import_module.DiffResult{
		BlueprintsToCreate: []api.Blueprint{{"identifier": "service"}, {"identifier": "repo"}},
		BlueprintsToUpdate: []api.Blueprint{{"identifier": "team"}},
		BlueprintPermissions: []import_module.PermissionsChange{
			{Identifier: "service"},
			{Identifier: "repo"},
		},
	})

	if !reflect.DeepEqual(result.BlueprintsToCreate, []string{"repo", "service"}) {
		t.Fatalf("unexpected blueprints to create: %#v", result.BlueprintsToCreate)
	}
	if !reflect.DeepEqual(result.BlueprintsToUpdate, []string{"team"}) {
		t.Fatalf("unexpected blueprints to update: %#v", result.BlueprintsToUpdate)
	}
	if !reflect.DeepEqual(result.BlueprintPermissionsToUpdate, []string{"repo", "service"}) {
		t.Fatalf("unexpected permissions to update: %#v", result.BlueprintPermissionsToUpdate)
	}
}

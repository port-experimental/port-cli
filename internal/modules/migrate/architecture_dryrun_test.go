package migrate

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
)

func TestMigrateDryRunResultIdenticalDiffContract(t *testing.T) {
	m := &Module{}
	result := m.generateDryRunResult(&import_module.DiffResult{
		BlueprintsToSkip: []api.Blueprint{{"identifier": "service"}},
		EntitiesToSkip:   []api.Entity{{"identifier": "svc", "blueprint": "service"}},
	})

	if !result.Success {
		t.Fatal("expected successful dry run")
	}
	if result.BlueprintsCreated != 0 || result.BlueprintsUpdated != 0 || result.EntitiesCreated != 0 || result.EntitiesUpdated != 0 {
		t.Fatalf("identical diff should not create/update resources: %#v", result)
	}
	if len(result.BlueprintsToSkip) != 1 || result.BlueprintsToSkip[0] != "service" {
		t.Fatalf("expected skipped blueprint identifier, got %#v", result.BlueprintsToSkip)
	}
}

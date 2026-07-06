package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestImportDryRunResultIdenticalDiffContract(t *testing.T) {
	m := &Module{}
	result := m.generateDryRunResult(&export.Data{}, &DiffResult{
		BlueprintsToSkip: []api.Blueprint{{"identifier": "service"}},
		EntitiesToSkip:   []api.Entity{{"identifier": "svc", "blueprint": "service"}},
	}, Options{})

	if !result.Success {
		t.Fatal("expected successful dry run")
	}
	if result.BlueprintsCreated != 0 || result.BlueprintsUpdated != 0 || result.EntitiesCreated != 0 || result.EntitiesUpdated != 0 {
		t.Fatalf("identical diff should not create/update resources: %#v", result)
	}
}

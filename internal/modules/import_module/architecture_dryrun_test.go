package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/plan"
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

func TestImportDryRunResultMatchesPlanCounts(t *testing.T) {
	diffResult := &DiffResult{
		ActionsToCreate: []api.Action{{"identifier": "new"}},
		ActionsToUpdate: []api.Action{{"identifier": "changed"}},
		TeamsToSkip:     []api.Team{{"name": "platform"}},
		BlueprintPermissions: map[string]api.Permissions{
			"service": {"read": []interface{}{"Everyone"}},
		},
	}
	result := (&Module{}).generateDryRunResult(&export.Data{}, diffResult, Options{})

	if result.ActionsCreated != 1 || result.ActionsUpdated != 1 {
		t.Fatalf("unexpected action counts: created=%d updated=%d", result.ActionsCreated, result.ActionsUpdated)
	}
	if result.TeamsCreated != 0 || result.TeamsUpdated != 0 {
		t.Fatalf("skipped teams should not count as create/update: %#v", result)
	}
	if result.BlueprintPermissionsUpdated != 1 {
		t.Fatalf("expected 1 blueprint permission update, got %d", result.BlueprintPermissionsUpdated)
	}

	summary := plan.Summarize(plan.BuildFromDiffResult(diffResult))
	counters := plan.ApplyCountersFromSummary(summary)
	if result.ActionsCreated != counters.Actions.Created || result.BlueprintPermissionsUpdated != counters.BlueprintPermissionsUpdated {
		t.Fatalf("import dry-run counters diverged from plan summary: result=%#v counters=%#v", result, counters)
	}
}

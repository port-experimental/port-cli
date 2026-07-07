package migrate

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/plan"
)

func TestMigrateDryRunResultIdenticalDiffContract(t *testing.T) {
	diffResult := &import_module.DiffResult{
		BlueprintsToSkip: []api.Blueprint{{"identifier": "service"}},
		EntitiesToSkip:   []api.Entity{{"identifier": "svc", "blueprint": "service"}},
	}
	result := (&Module{}).generateDryRunResult(import_module.BuildFromDiffResult(diffResult), diffResult)
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

func TestMigrateDryRunResultMatchesDiffResultCounts(t *testing.T) {
	diffResult := &import_module.DiffResult{
		ActionsToCreate: []api.Action{{"identifier": "new"}},
		ActionsToUpdate: []api.Action{{"identifier": "changed"}},
		TeamsToSkip:     []api.Team{{"name": "platform"}},
	}
	executionPlan := import_module.BuildFromDiffResult(diffResult)
	result := (&Module{}).generateDryRunResult(executionPlan, diffResult)

	if result.ActionsCreated != 1 || result.ActionsUpdated != 1 {
		t.Fatalf("unexpected dry-run counts: created=%d updated=%d", result.ActionsCreated, result.ActionsUpdated)
	}
	if result.TeamsSkipped != 1 {
		t.Fatalf("expected 1 skipped team, got %d", result.TeamsSkipped)
	}
}

package diff_test

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestExecutionPlanSummaryMatchesImportDiff(t *testing.T) {
	diff := &importmodule.DiffResult{
		BlueprintsToCreate: []api.Blueprint{{"identifier": "svc"}},
		ActionsToUpdate:    []api.Action{{"identifier": "deploy"}},
		PagesToSkip:        []api.Page{{"identifier": "home"}},
	}
	summary := plan.Summarize(plan.BuildFromDiffResult(diff))

	if summary.Created[resources.KindBlueprints] != 1 {
		t.Fatalf("blueprints created: got %d", summary.Created[resources.KindBlueprints])
	}
	if summary.Updated[resources.KindActions] != 1 {
		t.Fatalf("actions updated: got %d", summary.Updated[resources.KindActions])
	}
	if summary.Skipped[resources.KindPages] != 1 {
		t.Fatalf("pages skipped: got %d", summary.Skipped[resources.KindPages])
	}
}

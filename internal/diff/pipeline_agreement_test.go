package diff_test

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/diff"
	"github.com/port-experimental/port-cli/internal/modules/compare"
	"github.com/port-experimental/port-cli/internal/modules/export"
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestPipelineSummaryAgreement_FullFixture(t *testing.T) {
	// current = target org state, desired = source org export (migrate/import direction)
	current := &export.Data{
		Actions: []api.Action{
			{"identifier": "deploy", "title": "Deploy"},
			{"identifier": "rollback", "title": "Rollback"},
		},
		Teams: []api.Team{
			{"name": "platform", "description": "Platform"},
		},
		Pages: []api.Page{
			{"identifier": "overview", "title": "Overview", "parent": "root"},
		},
	}
	desired := &export.Data{
		Actions: []api.Action{
			{"identifier": "deploy", "title": "Deploy v2"},
			{"identifier": "scale", "title": "Scale"},
		},
		Teams: []api.Team{
			{"name": "platform", "description": "Platform team"},
		},
		Pages: []api.Page{
			{"identifier": "overview", "title": "Overview", "parent": nil},
		},
	}

	include := []string{"actions", "teams", "pages"}
	opts := importmodule.Options{IncludeResources: include}

	compareResult := compare.NewDiffer().Diff(desired, current, include)
	diffResult := importmodule.BuildDiffResult(desired, current, opts)

	assertResourceAgreement(t, "actions",
		compareResult.Actions.Summary,
		len(diffResult.ActionsToCreate), len(diffResult.ActionsToUpdate),
	)

	assertResourceAgreement(t, "teams",
		compareResult.Teams.Summary,
		len(diffResult.TeamsToCreate), len(diffResult.TeamsToUpdate),
	)

	// Pages: null parent in desired should not count as modified (shared PagesEqual).
	if compareResult.Pages.Summary.Modified != 0 {
		t.Fatalf("pages compare: expected 0 modified (null nav ignored), got %d", compareResult.Pages.Summary.Modified)
	}
	if len(diffResult.PagesToUpdate) != 0 {
		t.Fatalf("pages import: expected 0 updates, got %d", len(diffResult.PagesToUpdate))
	}
}

func TestPipelineSummaryAgreement_PagesNavSemantics(t *testing.T) {
	current := []api.Page{{"identifier": "p1", "title": "Page", "parent": "root"}}
	desired := []api.Page{{"identifier": "p1", "title": "Page", "parent": nil}}

	comparePages := compare.NewDiffer().Diff(
		&export.Data{Pages: desired},
		&export.Data{Pages: current},
		[]string{"pages"},
	)
	importOutcome := diff.DiffForImport(current, desired, diff.ImportConfig{
		Kind:  resources.KindPages,
		Equal: resources.PagesEqual,
	})

	if comparePages.Pages.Summary.Modified != 0 {
		t.Fatalf("compare: expected 0 modified, got %d", comparePages.Pages.Summary.Modified)
	}
	if len(importOutcome.ToUpdate) != 0 {
		t.Fatalf("import: expected 0 updates, got %d", len(importOutcome.ToUpdate))
	}
}

func assertResourceAgreement(
	t *testing.T,
	name string,
	summary compare.DiffSummary,
	importCreates, importUpdates int,
) {
	t.Helper()
	if summary.Added != importCreates || summary.Modified != importUpdates {
		t.Fatalf("%s compare vs import: compare added=%d modified=%d, import create=%d update=%d",
			name, summary.Added, summary.Modified, importCreates, importUpdates)
	}
}

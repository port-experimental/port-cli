package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestBuildFromDiffResult_AllOperations(t *testing.T) {
	diff := &DiffResult{
		BlueprintsToCreate: []api.Blueprint{{"identifier": "service"}},
		BlueprintsToUpdate: []api.Blueprint{{"identifier": "team"}},
		BlueprintsToSkip:   []api.Blueprint{{"identifier": "_user"}},
		ActionsToCreate:    []api.Action{{"identifier": "deploy"}},
		ActionsToUpdate:    []api.Action{{"identifier": "scale"}},
		ScorecardsToCreate: []api.Scorecard{{"blueprintIdentifier": "service", "identifier": "quality"}},
		PagesToUpdate:      []api.Page{{"identifier": "overview"}},
		IntegrationsToUpdate: []api.Integration{
			{"installationId": "github", "config": map[string]interface{}{"org": "acme"}},
		},
		BlueprintPermissions: []PermissionsChange{
			{Identifier: "service", Permissions: api.Permissions{"read": map[string]interface{}{}}},
		},
	}

	p := BuildFromDiffResult(diff)
	summary := plan.Summarize(p)

	if summary.Created[resources.KindBlueprints] != 1 {
		t.Fatalf("blueprints create: got %d", summary.Created[resources.KindBlueprints])
	}
	if summary.Updated[resources.KindBlueprints] != 1 {
		t.Fatalf("blueprints update: got %d", summary.Updated[resources.KindBlueprints])
	}
	if summary.Skipped[resources.KindBlueprints] != 1 {
		t.Fatalf("blueprints skip: got %d", summary.Skipped[resources.KindBlueprints])
	}
	if summary.Created[resources.KindActions] != 1 || summary.Updated[resources.KindActions] != 1 {
		t.Fatalf("actions: create=%d update=%d", summary.Created[resources.KindActions], summary.Updated[resources.KindActions])
	}
	if summary.Created[resources.KindScorecards] != 1 {
		t.Fatalf("scorecards create: got %d", summary.Created[resources.KindScorecards])
	}
	if summary.Updated[resources.KindPages] != 1 {
		t.Fatalf("pages update: got %d", summary.Updated[resources.KindPages])
	}
	if summary.Updated[resources.KindIntegrations] != 1 {
		t.Fatalf("integrations update: got %d", summary.Updated[resources.KindIntegrations])
	}
	if summary.PermissionUpdates[resources.KindBlueprintPermissions] != 1 {
		t.Fatalf("blueprint permissions: got %d", summary.PermissionUpdates[resources.KindBlueprintPermissions])
	}
}

func TestBuildFromDiffResult_NilDiff(t *testing.T) {
	p := BuildFromDiffResult(nil)
	if len(p.Steps) != 0 {
		t.Fatalf("expected empty plan, got %d steps", len(p.Steps))
	}
}

func TestCreateUpdateSets_CompositeIdentity(t *testing.T) {
	diff := &DiffResult{
		ScorecardsToCreate: []api.Scorecard{{"blueprintIdentifier": "svc", "identifier": "sc1"}},
		ScorecardsToUpdate: []api.Scorecard{{"blueprintIdentifier": "svc", "identifier": "sc2"}},
	}
	p := BuildFromDiffResult(diff)
	create, update := plan.CreateUpdateSets(p, resources.KindScorecards)

	if !create["svc:sc1"] {
		t.Fatal("expected create key svc:sc1")
	}
	if !update["svc:sc2"] {
		t.Fatal("expected update key svc:sc2")
	}
}

func TestApplyCountersFromSummaryMatchesBuildFromDiffResult(t *testing.T) {
	diffResult := &DiffResult{
		ActionsToCreate: []api.Action{{"identifier": "new"}},
		ActionsToUpdate: []api.Action{{"identifier": "changed"}},
		TeamsToSkip:     []api.Team{{"name": "platform"}},
	}
	counters := plan.ApplyCountersFromSummary(plan.Summarize(BuildFromDiffResult(diffResult)))
	if counters.Actions.Created != 1 || counters.Actions.Updated != 1 || counters.Teams.Skipped != 1 {
		t.Fatalf("unexpected counters from diff: %#v", counters)
	}
}

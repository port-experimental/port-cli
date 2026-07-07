package plan

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestApplyCountersFromSummary(t *testing.T) {
	summary := Summarize(&ExecutionPlan{Steps: []Step{
		{Kind: resources.KindActions, Operation: OpCreate, Identifier: "new"},
		{Kind: resources.KindActions, Operation: OpUpdate, Identifier: "changed"},
		{Kind: resources.KindTeams, Operation: OpSkip, Identifier: "platform"},
		{Kind: resources.KindBlueprintPermissions, Operation: OpPermissionUpdate, Identifier: "service"},
	}})

	counters := ApplyCountersFromSummary(summary)
	if counters.Actions.Created != 1 || counters.Actions.Updated != 1 {
		t.Fatalf("unexpected action counters: %#v", counters.Actions)
	}
	if counters.Teams.Skipped != 1 {
		t.Fatalf("expected 1 skipped team, got %#v", counters.Teams)
	}
	if counters.BlueprintPermissionsUpdated != 1 {
		t.Fatalf("expected 1 blueprint permission update, got %d", counters.BlueprintPermissionsUpdated)
	}
}

func TestApplyCountersFromImportMergesSkipped(t *testing.T) {
	importResult := &importmodule.Result{
		BlueprintsCreated: 2,
		TeamsCreated:      1,
	}
	summary := Summarize(&ExecutionPlan{Steps: []Step{
		{Kind: resources.KindBlueprints, Operation: OpSkip, Identifier: "unchanged"},
		{Kind: resources.KindTeams, Operation: OpSkip, Identifier: "legacy"},
	}})

	counters := ApplyCountersFromImport(importResult, summary)
	if counters.Blueprints.Created != 2 || counters.Blueprints.Skipped != 1 {
		t.Fatalf("unexpected blueprint counters: %#v", counters.Blueprints)
	}
	if counters.Teams.Created != 1 || counters.Teams.Skipped != 1 {
		t.Fatalf("unexpected team counters: %#v", counters.Teams)
	}
}

func TestApplyCountersFromSummaryIntegrationOnlyUpdated(t *testing.T) {
	summary := Summarize(&ExecutionPlan{Steps: []Step{
		{Kind: resources.KindIntegrations, Operation: OpUpdate, Identifier: "github"},
		{Kind: resources.KindIntegrations, Operation: OpSkip, Identifier: "jira"},
	}})
	counters := ApplyCountersFromSummary(summary)
	if counters.Integration.Updated != 1 || counters.Integration.Skipped != 1 {
		t.Fatalf("unexpected integration counters: %#v", counters.Integration)
	}
}

func TestApplyCountersFromImportNilUsesSummaryOnly(t *testing.T) {
	summary := Summarize(&ExecutionPlan{Steps: []Step{
		{Kind: resources.KindEntities, Operation: OpCreate, Identifier: "service:svc"},
	}})
	counters := ApplyCountersFromImport(nil, summary)
	if counters.Entities.Created != 1 {
		t.Fatalf("expected entity create count from summary, got %#v", counters.Entities)
	}
}

func TestApplyCountersFromImportPreservesPermissionUpdates(t *testing.T) {
	importResult := &importmodule.Result{
		PagePermissionsUpdated: 3,
	}
	counters := ApplyCountersFromImport(importResult, Summary{})
	if counters.PagePermissionsUpdated != 3 {
		t.Fatalf("expected page permission updates to carry over, got %d", counters.PagePermissionsUpdated)
	}
}

func TestApplyCountersFromSummaryMatchesBuildFromDiffResult(t *testing.T) {
	diffResult := &importmodule.DiffResult{
		ActionsToCreate: []api.Action{{"identifier": "new"}},
		ActionsToUpdate: []api.Action{{"identifier": "changed"}},
		TeamsToSkip:     []api.Team{{"name": "platform"}},
	}
	counters := ApplyCountersFromSummary(Summarize(BuildFromDiffResult(diffResult)))
	if counters.Actions.Created != 1 || counters.Actions.Updated != 1 || counters.Teams.Skipped != 1 {
		t.Fatalf("unexpected counters from diff: %#v", counters)
	}
}

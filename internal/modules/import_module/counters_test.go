package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestApplyCountersFromImportMergesSkipped(t *testing.T) {
	importResult := &Result{
		BlueprintsCreated: 2,
		TeamsCreated:      1,
	}
	summary := plan.Summarize(&plan.ExecutionPlan{Steps: []plan.Step{
		{Kind: resources.KindBlueprints, Operation: plan.OpSkip, Identifier: "unchanged"},
		{Kind: resources.KindTeams, Operation: plan.OpSkip, Identifier: "legacy"},
	}})

	counters := ApplyCountersFromImport(importResult, summary)
	if counters.Blueprints.Created != 2 || counters.Blueprints.Skipped != 1 {
		t.Fatalf("unexpected blueprint counters: %#v", counters.Blueprints)
	}
	if counters.Teams.Created != 1 || counters.Teams.Skipped != 1 {
		t.Fatalf("unexpected team counters: %#v", counters.Teams)
	}
}

func TestApplyCountersFromImportNilUsesSummaryOnly(t *testing.T) {
	summary := plan.Summarize(&plan.ExecutionPlan{Steps: []plan.Step{
		{Kind: resources.KindEntities, Operation: plan.OpCreate, Identifier: "service:svc"},
	}})
	counters := ApplyCountersFromImport(nil, summary)
	if counters.Entities.Created != 1 {
		t.Fatalf("expected entity create count from summary, got %#v", counters.Entities)
	}
}

func TestApplyCountersFromImportPreservesPermissionUpdates(t *testing.T) {
	importResult := &Result{
		PagePermissionsUpdated: 3,
	}
	counters := ApplyCountersFromImport(importResult, plan.Summary{})
	if counters.PagePermissionsUpdated != 3 {
		t.Fatalf("expected page permission updates to carry over, got %d", counters.PagePermissionsUpdated)
	}
}

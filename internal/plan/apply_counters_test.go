package plan

import (
	"testing"

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

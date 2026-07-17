package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestApplyContextFromPlan_PermissionsAndUserUpdates(t *testing.T) {
	diff := &DiffResult{
		UsersToCreate: []api.User{{"email": "new@example.com"}},
		UsersToUpdate: []api.User{{"email": "existing@example.com"}},
		UsersToSkip:   []api.User{{"email": "same@example.com"}},
		BlueprintPermissions: []PermissionsChange{
			{Identifier: "service", Permissions: api.Permissions{"entities": map[string]interface{}{"read": []string{"Admin"}}}},
		},
		ActionPermissions: []PermissionsChange{
			{Identifier: "deploy", Permissions: api.Permissions{"execute": []string{"Admin"}}},
		},
		PagePermissions: []PermissionsChange{
			{Identifier: "home", Permissions: api.Permissions{"read": []string{"Admin"}}},
		},
	}

	applyCtx := ApplyContextFromPlan(BuildFromDiffResult(diff))

	if len(applyCtx.BlueprintPermissions) != 1 || applyCtx.BlueprintPermissions[0].Identifier != "service" {
		t.Fatalf("blueprint permissions: %#v", applyCtx.BlueprintPermissions)
	}
	if len(applyCtx.ActionPermissions) != 1 || applyCtx.ActionPermissions[0].Identifier != "deploy" {
		t.Fatalf("action permissions: %#v", applyCtx.ActionPermissions)
	}
	if len(applyCtx.PagePermissions) != 1 || applyCtx.PagePermissions[0].Identifier != "home" {
		t.Fatalf("page permissions: %#v", applyCtx.PagePermissions)
	}
	if !applyCtx.UserUpdateEmails["existing@example.com"] {
		t.Fatalf("expected existing@example.com in UserUpdateEmails, got %#v", applyCtx.UserUpdateEmails)
	}
	if applyCtx.UserUpdateEmails["new@example.com"] || applyCtx.UserUpdateEmails["same@example.com"] {
		t.Fatalf("create/skip users must not be update emails: %#v", applyCtx.UserUpdateEmails)
	}
}

func TestApplyContextFromPlan_NilPlan(t *testing.T) {
	applyCtx := ApplyContextFromPlan(nil)
	if len(applyCtx.BlueprintPermissions) != 0 || applyCtx.UserUpdateEmails != nil {
		t.Fatalf("expected empty context, got %#v", applyCtx)
	}
}

func TestApplyContextAlignedWithDryRunPermissionCounters(t *testing.T) {
	diff := &DiffResult{
		UsersToUpdate: []api.User{{"email": "u@example.com"}},
		BlueprintPermissions: []PermissionsChange{
			{Identifier: "service", Permissions: api.Permissions{"read": []string{"Admin"}}},
		},
		ActionPermissions: []PermissionsChange{
			{Identifier: "deploy", Permissions: api.Permissions{"execute": []string{"Admin"}}},
		},
		PagePermissions: []PermissionsChange{
			{Identifier: "home", Permissions: api.Permissions{"read": []string{"Admin"}}},
		},
	}
	executionPlan := BuildFromDiffResult(diff)
	applyCtx := ApplyContextFromPlan(executionPlan)
	counters := plan.ApplyCountersFromSummary(plan.Summarize(executionPlan))

	if counters.BlueprintPermissionsUpdated != len(applyCtx.BlueprintPermissions) {
		t.Fatalf("blueprint perm counters=%d applyCtx=%d", counters.BlueprintPermissionsUpdated, len(applyCtx.BlueprintPermissions))
	}
	if counters.ActionPermissionsUpdated != len(applyCtx.ActionPermissions) {
		t.Fatalf("action perm counters=%d applyCtx=%d", counters.ActionPermissionsUpdated, len(applyCtx.ActionPermissions))
	}
	if counters.PagePermissionsUpdated != len(applyCtx.PagePermissions) {
		t.Fatalf("page perm counters=%d applyCtx=%d", counters.PagePermissionsUpdated, len(applyCtx.PagePermissions))
	}
	if counters.Users.Updated != len(applyCtx.UserUpdateEmails) {
		t.Fatalf("users updated counters=%d applyCtx emails=%d", counters.Users.Updated, len(applyCtx.UserUpdateEmails))
	}

	// Spot-check plan identifiers match apply context (dry-run detail agreement).
	bpIDs := plan.Identifiers(executionPlan, resources.KindBlueprintPermissions, plan.OpPermissionUpdate)
	if len(bpIDs) != 1 || bpIDs[0] != applyCtx.BlueprintPermissions[0].Identifier {
		t.Fatalf("blueprint permission identifiers mismatch: plan=%v apply=%v", bpIDs, applyCtx.BlueprintPermissions)
	}
	userIDs := plan.Identifiers(executionPlan, resources.KindUsers, plan.OpUpdate)
	if len(userIDs) != 1 || !applyCtx.UserUpdateEmails[userIDs[0]] {
		t.Fatalf("user update identifiers mismatch: plan=%v apply=%v", userIDs, applyCtx.UserUpdateEmails)
	}
}

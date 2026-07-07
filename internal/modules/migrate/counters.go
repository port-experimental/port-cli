package migrate

import (
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

func populateMigrateCounters(result *Result, counters plan.ApplyCounters) {
	result.BlueprintsCreated = counters.Blueprints.Created
	result.BlueprintsUpdated = counters.Blueprints.Updated
	result.BlueprintsSkipped = counters.Blueprints.Skipped
	result.EntitiesCreated = counters.Entities.Created
	result.EntitiesUpdated = counters.Entities.Updated
	result.EntitiesSkipped = counters.Entities.Skipped
	result.ScorecardsCreated = counters.Scorecards.Created
	result.ScorecardsUpdated = counters.Scorecards.Updated
	result.ScorecardsSkipped = counters.Scorecards.Skipped
	result.ActionsCreated = counters.Actions.Created
	result.ActionsUpdated = counters.Actions.Updated
	result.ActionsSkipped = counters.Actions.Skipped
	result.TeamsCreated = counters.Teams.Created
	result.TeamsUpdated = counters.Teams.Updated
	result.TeamsSkipped = counters.Teams.Skipped
	result.UsersCreated = counters.Users.Created
	result.UsersUpdated = counters.Users.Updated
	result.UsersSkipped = counters.Users.Skipped
	result.PagesCreated = counters.Pages.Created
	result.PagesUpdated = counters.Pages.Updated
	result.PagesSkipped = counters.Pages.Skipped
	result.IntegrationsUpdated = counters.Integration.Updated
	result.IntegrationsSkipped = counters.Integration.Skipped
	result.BlueprintPermissionsUpdated = counters.BlueprintPermissionsUpdated
	result.ActionPermissionsUpdated = counters.ActionPermissionsUpdated
	result.PagePermissionsUpdated = counters.PagePermissionsUpdated
}

func populateMigrateDryRunDetails(result *Result, executionPlan *plan.ExecutionPlan) {
	result.BlueprintsToCreate = plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpCreate)
	result.BlueprintsToUpdate = plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpUpdate)
	result.BlueprintsToSkip = plan.Identifiers(executionPlan, resources.KindBlueprints, plan.OpSkip)
	result.BlueprintPermissionsToUpdate = plan.Identifiers(executionPlan, resources.KindBlueprintPermissions, plan.OpPermissionUpdate)
	result.ActionPermissionsToUpdate = plan.Identifiers(executionPlan, resources.KindActionPermissions, plan.OpPermissionUpdate)
	result.PagePermissionsToUpdate = plan.Identifiers(executionPlan, resources.KindPagePermissions, plan.OpPermissionUpdate)
}

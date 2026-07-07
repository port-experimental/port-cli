package import_module

import "github.com/port-experimental/port-cli/internal/plan"

func populateImportResultCounters(result *Result, counters plan.ApplyCounters) {
	result.BlueprintsCreated = counters.Blueprints.Created
	result.BlueprintsUpdated = counters.Blueprints.Updated
	result.EntitiesCreated = counters.Entities.Created
	result.EntitiesUpdated = counters.Entities.Updated
	result.ScorecardsCreated = counters.Scorecards.Created
	result.ScorecardsUpdated = counters.Scorecards.Updated
	result.ActionsCreated = counters.Actions.Created
	result.ActionsUpdated = counters.Actions.Updated
	result.TeamsCreated = counters.Teams.Created
	result.TeamsUpdated = counters.Teams.Updated
	result.UsersCreated = counters.Users.Created
	result.UsersUpdated = counters.Users.Updated
	result.PagesCreated = counters.Pages.Created
	result.PagesUpdated = counters.Pages.Updated
	result.IntegrationsUpdated = counters.Integration.Updated
	result.BlueprintPermissionsUpdated = counters.BlueprintPermissionsUpdated
	result.ActionPermissionsUpdated = counters.ActionPermissionsUpdated
	result.PagePermissionsUpdated = counters.PagePermissionsUpdated
}

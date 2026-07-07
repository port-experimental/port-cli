package import_module

import (
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

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

// ApplyCountersFromImport merges actual import apply counts with planned skip counts.
func ApplyCountersFromImport(importResult *Result, summary plan.Summary) plan.ApplyCounters {
	if importResult == nil {
		return plan.ApplyCountersFromSummary(summary)
	}
	return plan.ApplyCounters{
		Blueprints: plan.ResourceCounts{
			Created: importResult.BlueprintsCreated,
			Updated: importResult.BlueprintsUpdated,
			Skipped: summary.Skipped[resources.KindBlueprints],
		},
		Entities: plan.ResourceCounts{
			Created: importResult.EntitiesCreated,
			Updated: importResult.EntitiesUpdated,
			Skipped: summary.Skipped[resources.KindEntities],
		},
		Scorecards: plan.ResourceCounts{
			Created: importResult.ScorecardsCreated,
			Updated: importResult.ScorecardsUpdated,
			Skipped: summary.Skipped[resources.KindScorecards],
		},
		Actions: plan.ResourceCounts{
			Created: importResult.ActionsCreated,
			Updated: importResult.ActionsUpdated,
			Skipped: summary.Skipped[resources.KindActions],
		},
		Teams: plan.ResourceCounts{
			Created: importResult.TeamsCreated,
			Updated: importResult.TeamsUpdated,
			Skipped: summary.Skipped[resources.KindTeams],
		},
		Users: plan.ResourceCounts{
			Created: importResult.UsersCreated,
			Updated: importResult.UsersUpdated,
			Skipped: summary.Skipped[resources.KindUsers],
		},
		Pages: plan.ResourceCounts{
			Created: importResult.PagesCreated,
			Updated: importResult.PagesUpdated,
			Skipped: summary.Skipped[resources.KindPages],
		},
		Integration: plan.ResourceCounts{
			Updated: importResult.IntegrationsUpdated,
			Skipped: summary.Skipped[resources.KindIntegrations],
		},
		BlueprintPermissionsUpdated: importResult.BlueprintPermissionsUpdated,
		ActionPermissionsUpdated:    importResult.ActionPermissionsUpdated,
		PagePermissionsUpdated:      importResult.PagePermissionsUpdated,
	}
}

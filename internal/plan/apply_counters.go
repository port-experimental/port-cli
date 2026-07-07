package plan

import (
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/resources"
)

// ResourceCounts holds create/update/skip tallies for one resource kind.
type ResourceCounts struct {
	Created int
	Updated int
	Skipped int
}

// ApplyCounters is the normalized counter model derived from execution plans and apply results.
type ApplyCounters struct {
	Blueprints  ResourceCounts
	Entities    ResourceCounts
	Scorecards  ResourceCounts
	Actions     ResourceCounts
	Teams       ResourceCounts
	Users       ResourceCounts
	Pages       ResourceCounts
	Integration ResourceCounts

	BlueprintPermissionsUpdated int
	ActionPermissionsUpdated    int
	PagePermissionsUpdated      int
}

// ApplyCountersFromSummary builds counters from a summarized execution plan.
func ApplyCountersFromSummary(summary Summary) ApplyCounters {
	return ApplyCounters{
		Blueprints:  resourceCountsFromSummary(summary, resources.KindBlueprints),
		Entities:    resourceCountsFromSummary(summary, resources.KindEntities),
		Scorecards:  resourceCountsFromSummary(summary, resources.KindScorecards),
		Actions:     resourceCountsFromSummary(summary, resources.KindActions),
		Teams:       resourceCountsFromSummary(summary, resources.KindTeams),
		Users:       resourceCountsFromSummary(summary, resources.KindUsers),
		Pages:       resourceCountsFromSummary(summary, resources.KindPages),
		Integration: integrationCountsFromSummary(summary),
		BlueprintPermissionsUpdated: summary.PermissionUpdates[resources.KindBlueprintPermissions],
		ActionPermissionsUpdated:    summary.PermissionUpdates[resources.KindActionPermissions],
		PagePermissionsUpdated:      summary.PermissionUpdates[resources.KindPagePermissions],
	}
}

// ApplyCountersFromImport merges actual import apply counts with planned skip counts.
func ApplyCountersFromImport(importResult *importmodule.Result, summary Summary) ApplyCounters {
	if importResult == nil {
		return ApplyCountersFromSummary(summary)
	}
	return ApplyCounters{
		Blueprints: ResourceCounts{
			Created: importResult.BlueprintsCreated,
			Updated: importResult.BlueprintsUpdated,
			Skipped: summary.Skipped[resources.KindBlueprints],
		},
		Entities: ResourceCounts{
			Created: importResult.EntitiesCreated,
			Updated: importResult.EntitiesUpdated,
			Skipped: summary.Skipped[resources.KindEntities],
		},
		Scorecards: ResourceCounts{
			Created: importResult.ScorecardsCreated,
			Updated: importResult.ScorecardsUpdated,
			Skipped: summary.Skipped[resources.KindScorecards],
		},
		Actions: ResourceCounts{
			Created: importResult.ActionsCreated,
			Updated: importResult.ActionsUpdated,
			Skipped: summary.Skipped[resources.KindActions],
		},
		Teams: ResourceCounts{
			Created: importResult.TeamsCreated,
			Updated: importResult.TeamsUpdated,
			Skipped: summary.Skipped[resources.KindTeams],
		},
		Users: ResourceCounts{
			Created: importResult.UsersCreated,
			Updated: importResult.UsersUpdated,
			Skipped: summary.Skipped[resources.KindUsers],
		},
		Pages: ResourceCounts{
			Created: importResult.PagesCreated,
			Updated: importResult.PagesUpdated,
			Skipped: summary.Skipped[resources.KindPages],
		},
		Integration: ResourceCounts{
			Updated: importResult.IntegrationsUpdated,
			Skipped: summary.Skipped[resources.KindIntegrations],
		},
		BlueprintPermissionsUpdated: importResult.BlueprintPermissionsUpdated,
		ActionPermissionsUpdated:    importResult.ActionPermissionsUpdated,
		PagePermissionsUpdated:      importResult.PagePermissionsUpdated,
	}
}

func resourceCountsFromSummary(summary Summary, kind resources.ResourceKind) ResourceCounts {
	return ResourceCounts{
		Created: summary.Created[kind],
		Updated: summary.Updated[kind],
		Skipped: summary.Skipped[kind],
	}
}

func integrationCountsFromSummary(summary Summary) ResourceCounts {
	return ResourceCounts{
		Updated: summary.Updated[resources.KindIntegrations],
		Skipped: summary.Skipped[resources.KindIntegrations],
	}
}

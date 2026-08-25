package render

import (
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
)

// ResourceCounts holds create/update/skip tallies for one resource kind.
type ResourceCounts struct {
	Created int
	Updated int
	Skipped int
}

// ApplyCounts is the shared counter model for import and migrate result rendering.
type ApplyCounts struct {
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

// ApplyCountsFromImport maps an import result into the shared counter model.
func ApplyCountsFromImport(result *importmodule.Result) ApplyCounts {
	if result == nil {
		return ApplyCounts{}
	}
	return ApplyCounts{
		Blueprints: ResourceCounts{
			Created: result.BlueprintsCreated,
			Updated: result.BlueprintsUpdated,
		},
		Entities: ResourceCounts{
			Created: result.EntitiesCreated,
			Updated: result.EntitiesUpdated,
		},
		Scorecards: ResourceCounts{
			Created: result.ScorecardsCreated,
			Updated: result.ScorecardsUpdated,
		},
		Actions: ResourceCounts{
			Created: result.ActionsCreated,
			Updated: result.ActionsUpdated,
		},
		Teams: ResourceCounts{
			Created: result.TeamsCreated,
			Updated: result.TeamsUpdated,
		},
		Users: ResourceCounts{
			Created: result.UsersCreated,
			Updated: result.UsersUpdated,
		},
		Pages: ResourceCounts{
			Created: result.PagesCreated,
			Updated: result.PagesUpdated,
		},
		Integration: ResourceCounts{
			Updated: result.IntegrationsUpdated,
		},
		BlueprintPermissionsUpdated: result.BlueprintPermissionsUpdated,
		ActionPermissionsUpdated:    result.ActionPermissionsUpdated,
		PagePermissionsUpdated:      result.PagePermissionsUpdated,
	}
}

// ApplyCountsFromMigrate maps a migrate result into the shared counter model.
func ApplyCountsFromMigrate(result *migrate.Result) ApplyCounts {
	if result == nil {
		return ApplyCounts{}
	}
	return ApplyCounts{
		Blueprints: ResourceCounts{
			Created: result.BlueprintsCreated,
			Updated: result.BlueprintsUpdated,
			Skipped: result.BlueprintsSkipped,
		},
		Entities: ResourceCounts{
			Created: result.EntitiesCreated,
			Updated: result.EntitiesUpdated,
			Skipped: result.EntitiesSkipped,
		},
		Scorecards: ResourceCounts{
			Created: result.ScorecardsCreated,
			Updated: result.ScorecardsUpdated,
			Skipped: result.ScorecardsSkipped,
		},
		Actions: ResourceCounts{
			Created: result.ActionsCreated,
			Updated: result.ActionsUpdated,
			Skipped: result.ActionsSkipped,
		},
		Teams: ResourceCounts{
			Created: result.TeamsCreated,
			Updated: result.TeamsUpdated,
			Skipped: result.TeamsSkipped,
		},
		Users: ResourceCounts{
			Created: result.UsersCreated,
			Updated: result.UsersUpdated,
			Skipped: result.UsersSkipped,
		},
		Pages: ResourceCounts{
			Created: result.PagesCreated,
			Updated: result.PagesUpdated,
			Skipped: result.PagesSkipped,
		},
		Integration: ResourceCounts{
			Updated: result.IntegrationsUpdated,
			Skipped: result.IntegrationsSkipped,
		},
		BlueprintPermissionsUpdated: result.BlueprintPermissionsUpdated,
		ActionPermissionsUpdated:    result.ActionPermissionsUpdated,
		PagePermissionsUpdated:      result.PagePermissionsUpdated,
	}
}

type applyResourceSpec struct {
	label      string
	counts     func(ApplyCounts) ResourceCounts
	createdKey string
	updatedKey string
	skippedKey string
}

var applyResourceSpecs = []applyResourceSpec{
	{label: "Blueprints", counts: func(c ApplyCounts) ResourceCounts { return c.Blueprints }, createdKey: "blueprints_created", updatedKey: "blueprints_updated", skippedKey: "blueprints_skipped"},
	{label: "Entities", counts: func(c ApplyCounts) ResourceCounts { return c.Entities }, createdKey: "entities_created", updatedKey: "entities_updated", skippedKey: "entities_skipped"},
	{label: "Scorecards", counts: func(c ApplyCounts) ResourceCounts { return c.Scorecards }, createdKey: "scorecards_created", updatedKey: "scorecards_updated", skippedKey: "scorecards_skipped"},
	{label: "Actions", counts: func(c ApplyCounts) ResourceCounts { return c.Actions }, createdKey: "actions_created", updatedKey: "actions_updated", skippedKey: "actions_skipped"},
	{label: "Teams", counts: func(c ApplyCounts) ResourceCounts { return c.Teams }, createdKey: "teams_created", updatedKey: "teams_updated", skippedKey: "teams_skipped"},
	{label: "Users", counts: func(c ApplyCounts) ResourceCounts { return c.Users }, createdKey: "users_created", updatedKey: "users_updated", skippedKey: "users_skipped"},
	{label: "Pages", counts: func(c ApplyCounts) ResourceCounts { return c.Pages }, createdKey: "pages_created", updatedKey: "pages_updated", skippedKey: "pages_skipped"},
}

type applyPermissionSpec struct {
	label string
	value func(ApplyCounts) int
	key   string
}

var applyPermissionSpecs = []applyPermissionSpec{
	{label: "Blueprint permissions", value: func(c ApplyCounts) int { return c.BlueprintPermissionsUpdated }, key: "blueprint_permissions_updated"},
	{label: "Action permissions", value: func(c ApplyCounts) int { return c.ActionPermissionsUpdated }, key: "action_permissions_updated"},
	{label: "Page permissions", value: func(c ApplyCounts) int { return c.PagePermissionsUpdated }, key: "page_permissions_updated"},
}

// PopulateApplyCountsJSON writes stable import/migrate counter fields into a JSON map.
func PopulateApplyCountsJSON(data map[string]interface{}, counts ApplyCounts, includeSkipped bool) {
	for _, spec := range applyResourceSpecs {
		rc := spec.counts(counts)
		data[spec.createdKey] = rc.Created
		data[spec.updatedKey] = rc.Updated
		if includeSkipped {
			data[spec.skippedKey] = rc.Skipped
		}
	}
	integration := counts.Integration
	data["integrations_updated"] = integration.Updated
	if includeSkipped {
		data["integrations_skipped"] = integration.Skipped
	}
	for _, spec := range applyPermissionSpecs {
		data[spec.key] = spec.value(counts)
	}
}

// PrintApplyCountsText prints the standard resource counter summary lines.
func PrintApplyCountsText(counts ApplyCounts, includeSkipped bool) {
	for _, spec := range applyResourceSpecs {
		rc := spec.counts(counts)
		if includeSkipped {
			output.Printf("%s created: %d, updated: %d, skipped: %d\n", spec.label, rc.Created, rc.Updated, rc.Skipped)
		} else {
			output.Printf("%s created: %d, updated: %d\n", spec.label, rc.Created, rc.Updated)
		}
	}
	integration := counts.Integration
	if includeSkipped {
		output.Printf("Integrations updated: %d, skipped: %d\n", integration.Updated, integration.Skipped)
	} else {
		output.Printf("Integrations updated: %d\n", integration.Updated)
	}
	if counts.BlueprintPermissionsUpdated > 0 || counts.ActionPermissionsUpdated > 0 || counts.PagePermissionsUpdated > 0 {
		for _, spec := range applyPermissionSpecs {
			if v := spec.value(counts); v > 0 {
				output.Printf("%s updated: %d\n", spec.label, v)
			}
		}
	}
}

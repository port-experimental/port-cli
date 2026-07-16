package import_module

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/diff"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/resources"
	"github.com/port-experimental/port-cli/internal/snapshot"
	systemblueprints "github.com/port-experimental/port-cli/internal/modules/system_blueprints"
)

// PermissionsChange represents a permissions update for a single resource.
type PermissionsChange struct {
	Identifier  string
	Permissions api.Permissions
}

// DiffResult represents the result of comparing import data with current state.
type DiffResult struct {
	BlueprintsToCreate   []api.Blueprint
	BlueprintsToUpdate   []api.Blueprint
	BlueprintsToSkip     []api.Blueprint
	EntitiesToCreate     []api.Entity
	EntitiesToUpdate     []api.Entity
	EntitiesToSkip       []api.Entity
	ScorecardsToCreate   []api.Scorecard
	ScorecardsToUpdate   []api.Scorecard
	ScorecardsToSkip     []api.Scorecard
	ActionsToCreate      []api.Action
	ActionsToUpdate      []api.Action
	ActionsToSkip        []api.Action
	TeamsToCreate        []api.Team
	TeamsToUpdate        []api.Team
	TeamsToSkip          []api.Team
	UsersToCreate        []api.User
	UsersToUpdate        []api.User
	UsersToSkip          []api.User
	PagesToCreate        []api.Page
	PagesToUpdate        []api.Page
	PagesToSkip          []api.Page
	IntegrationsToUpdate []api.Integration
	IntegrationsToSkip   []api.Integration
	BlueprintPermissions []PermissionsChange
	ActionPermissions    []PermissionsChange
	PagePermissions      []PermissionsChange
}

// DiffComparer compares import data with current organization state.
type DiffComparer struct {
	client *api.Client
}

// NewDiffComparer creates a new diff comparer.
func NewDiffComparer(client *api.Client) *DiffComparer {
	return &DiffComparer{
		client: client,
	}
}

// Compare compares import data with current organization state.
func (d *DiffComparer) Compare(ctx context.Context, importData *export.Data, opts Options) (*DiffResult, error) {
	// Export current state from target organization
	currentData, err := d.exportCurrentState(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to export current state: %w", err)
	}

	result := &DiffResult{}

	// Compare each resource type
	result.BlueprintsToCreate, result.BlueprintsToUpdate, result.BlueprintsToSkip = d.compareBlueprints(importData.Blueprints, currentData.Blueprints, opts.IncludeResources)
	result.EntitiesToCreate, result.EntitiesToUpdate, result.EntitiesToSkip = d.compareEntities(importData.Entities, currentData.Entities, opts.IncludeResources)
	result.ScorecardsToCreate, result.ScorecardsToUpdate, result.ScorecardsToSkip = d.compareScorecards(importData.Scorecards, currentData.Scorecards, opts.IncludeResources)
	result.ActionsToCreate, result.ActionsToUpdate, result.ActionsToSkip = d.compareActions(importData.Actions, currentData.Actions, opts.IncludeResources)
	result.TeamsToCreate, result.TeamsToUpdate, result.TeamsToSkip = d.compareTeams(importData.Teams, currentData.Teams, opts.IncludeResources)
	result.UsersToCreate, result.UsersToUpdate, result.UsersToSkip = d.compareUsers(importData.Users, currentData.Users, opts.IncludeResources)
	result.PagesToCreate, result.PagesToUpdate, result.PagesToSkip = d.comparePages(importData.Pages, currentData.Pages, opts.IncludeResources)
	result.IntegrationsToUpdate, result.IntegrationsToSkip = d.compareIntegrations(importData.Integrations, currentData.Integrations, opts.IncludeResources)

	// Compare permissions when included (or when no --include filter is set)
	if shouldImport("blueprint-permissions", opts.IncludeResources) {
		result.BlueprintPermissions = comparePermissions(currentData.BlueprintPermissions, importData.BlueprintPermissions, resources.KindBlueprintPermissions)
	}
	if shouldImport("action-permissions", opts.IncludeResources) {
		result.ActionPermissions = comparePermissions(currentData.ActionPermissions, importData.ActionPermissions, resources.KindActionPermissions)
	}
	if shouldImport("page-permissions", opts.IncludeResources) {
		result.PagePermissions = comparePermissions(currentData.PagePermissions, importData.PagePermissions, resources.KindPagePermissions)
	}

	return result, nil
}

// BuildDiffResult compares desired import data against a current snapshot without API calls.
func BuildDiffResult(importData, currentData *export.Data, opts Options) *DiffResult {
	d := &DiffComparer{}
	result := &DiffResult{}

	result.BlueprintsToCreate, result.BlueprintsToUpdate, result.BlueprintsToSkip = d.compareBlueprints(importData.Blueprints, currentData.Blueprints, opts.IncludeResources)
	result.EntitiesToCreate, result.EntitiesToUpdate, result.EntitiesToSkip = d.compareEntities(importData.Entities, currentData.Entities, opts.IncludeResources)
	result.ScorecardsToCreate, result.ScorecardsToUpdate, result.ScorecardsToSkip = d.compareScorecards(importData.Scorecards, currentData.Scorecards, opts.IncludeResources)
	result.ActionsToCreate, result.ActionsToUpdate, result.ActionsToSkip = d.compareActions(importData.Actions, currentData.Actions, opts.IncludeResources)
	result.TeamsToCreate, result.TeamsToUpdate, result.TeamsToSkip = d.compareTeams(importData.Teams, currentData.Teams, opts.IncludeResources)
	result.UsersToCreate, result.UsersToUpdate, result.UsersToSkip = d.compareUsers(importData.Users, currentData.Users, opts.IncludeResources)
	result.PagesToCreate, result.PagesToUpdate, result.PagesToSkip = d.comparePages(importData.Pages, currentData.Pages, opts.IncludeResources)
	result.IntegrationsToUpdate, result.IntegrationsToSkip = d.compareIntegrations(importData.Integrations, currentData.Integrations, opts.IncludeResources)

	if shouldImport("blueprint-permissions", opts.IncludeResources) {
		result.BlueprintPermissions = comparePermissions(currentData.BlueprintPermissions, importData.BlueprintPermissions, resources.KindBlueprintPermissions)
	}
	if shouldImport("action-permissions", opts.IncludeResources) {
		result.ActionPermissions = comparePermissions(currentData.ActionPermissions, importData.ActionPermissions, resources.KindActionPermissions)
	}
	if shouldImport("page-permissions", opts.IncludeResources) {
		result.PagePermissions = comparePermissions(currentData.PagePermissions, importData.PagePermissions, resources.KindPagePermissions)
	}

	return result
}

// exportCurrentState exports current state from target organization.
func (d *DiffComparer) exportCurrentState(ctx context.Context, opts Options) (*export.Data, error) {
	plan := snapshot.ImportDiffCollectPlan(
		opts.SkipEntities,
		opts.IncludeRuleResults,
		opts.IncludeResources,
		opts.ExcludeBlueprints,
		opts.ExcludeBlueprintSchema,
	)
	snap, err := snapshot.NewCollector(d.client).Collect(ctx, "current", plan)
	if err != nil {
		return nil, err
	}
	return snap.Data, nil
}

// FilterData filters import data to only include resources that need to be created or updated.
func (d *DiffResult) FilterData(original *export.Data) *export.Data {
	return &export.Data{
		Blueprints:   append(d.BlueprintsToCreate, d.BlueprintsToUpdate...),
		Entities:     append(d.EntitiesToCreate, d.EntitiesToUpdate...),
		Scorecards:   append(d.ScorecardsToCreate, d.ScorecardsToUpdate...),
		Actions:      append(d.ActionsToCreate, d.ActionsToUpdate...),
		Teams:        append(d.TeamsToCreate, d.TeamsToUpdate...),
		Users:        append(d.UsersToCreate, d.UsersToUpdate...),
		Folders:      original.Folders,
		Pages:        append(d.PagesToCreate, d.PagesToUpdate...),
		Integrations: d.IntegrationsToUpdate,
	}
}

// portManagedBlueprints are blueprints that are fully managed by Port and cannot be modified.
// These are skipped during import to avoid "protected_blueprint_violation" errors.
var portManagedBlueprints = map[string]bool{
	"_rule": true, // Managed through scorecards, not directly editable
}

// compareBlueprints compares import blueprints with current blueprints.
func (d *DiffComparer) compareBlueprints(importBPs, currentBPs []api.Blueprint, includeResources []string) (create, update, skip []api.Blueprint) {
	if !shouldImport("blueprints", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentBPs, importBPs, diff.ImportConfig{
		Kind: resources.KindBlueprints,
		ShouldSkip: func(desired map[string]interface{}) bool {
			identifier, _ := desired["identifier"].(string)
			return portManagedBlueprints[identifier] && !systemblueprints.IsCustomPatch(desired)
		},
		Equal: func(desired, current map[string]interface{}) bool {
			if systemblueprints.IsCustomPatch(desired) {
				return systemblueprints.CustomPatchEqual(desired, current)
			}
			return resources.ResourcesEqual(desired, current, resources.DefaultServerManagedFields)
		},
	})

	// System blueprint patches (and blueprints that prefer PATCH) cannot be
	// created on the target; treat missing targets as updates instead of creates.
	create = make([]api.Blueprint, 0, len(outcome.ToCreate))
	update = append([]api.Blueprint(nil), outcome.ToUpdate...)
	for _, bp := range outcome.ToCreate {
		id, _ := bp["identifier"].(string)
		if systemblueprints.IsCustomPatch(bp) || systemblueprints.PrefersPatchUpdate(id) {
			update = append(update, bp)
			continue
		}
		create = append(create, bp)
	}
	return create, update, outcome.ToSkip
}

// compareEntities compares import entities with current entities.
func (d *DiffComparer) compareEntities(importEnts, currentEnts []api.Entity, includeResources []string) (create, update, skip []api.Entity) {
	if !shouldImport("entities", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentEnts, importEnts, diff.ImportConfig{Kind: resources.KindEntities})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// compareScorecards compares import scorecards with current scorecards.
func (d *DiffComparer) compareScorecards(importScs, currentScs []api.Scorecard, includeResources []string) (create, update, skip []api.Scorecard) {
	if !shouldImport("scorecards", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentScs, importScs, diff.ImportConfig{Kind: resources.KindScorecards})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// compareActions compares import actions with current actions.
func (d *DiffComparer) compareActions(importActs, currentActs []api.Action, includeResources []string) (create, update, skip []api.Action) {
	if !shouldImport("actions", includeResources) && !shouldImport("automations", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentActs, importActs, diff.ImportConfig{Kind: resources.KindActions})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// compareTeams compares import teams with current teams.
func (d *DiffComparer) compareTeams(importTeams, currentTeams []api.Team, includeResources []string) (create, update, skip []api.Team) {
	if !shouldImport("teams", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentTeams, importTeams, diff.ImportConfig{Kind: resources.KindTeams})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// compareUsers compares import users with current users.
func (d *DiffComparer) compareUsers(importUsers, currentUsers []api.User, includeResources []string) (create, update, skip []api.User) {
	if !shouldImport("users", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentUsers, importUsers, diff.ImportConfig{Kind: resources.KindUsers})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// comparePages compares import pages with current pages.
func (d *DiffComparer) comparePages(importPages, currentPages []api.Page, includeResources []string) (create, update, skip []api.Page) {
	if !shouldImport("pages", includeResources) {
		return nil, nil, nil
	}
	outcome := diff.DiffForImport(currentPages, importPages, diff.ImportConfig{
		Kind: resources.KindPages,
		ShouldSkip: func(desired map[string]interface{}) bool {
			protected, _ := desired["protected"].(bool)
			return protected
		},
		Equal: resources.PagesEqual,
	})
	return outcome.ToCreate, outcome.ToUpdate, outcome.ToSkip
}

// compareIntegrations compares import integrations with current integrations.
func (d *DiffComparer) compareIntegrations(importInts, currentInts []api.Integration, includeResources []string) (update, skip []api.Integration) {
	if !shouldImport("integrations", includeResources) {
		return nil, nil
	}
	outcome := diff.DiffForImport(currentInts, importInts, diff.ImportConfig{
		Kind:          resources.KindIntegrations,
		IgnoreMissing: true,
	})
	return outcome.ToUpdate, outcome.ToSkip
}

// comparePermissions compares desired permissions against current permissions and
// returns a slice of changes for entries that are new or differ from current state.
func comparePermissions(current, desired map[string]api.Permissions, kind resources.ResourceKind) []PermissionsChange {
	changes := diff.DiffPermissions(permissionsToMaps(current), permissionsToMaps(desired), kind)
	result := make([]PermissionsChange, len(changes))
	for i, change := range changes {
		result[i] = PermissionsChange{
			Identifier:  change.Identifier,
			Permissions: api.Permissions(change.Desired),
		}
	}
	return result
}

func permissionsToMaps(perms map[string]api.Permissions) map[string]map[string]interface{} {
	if len(perms) == 0 {
		return nil
	}
	result := make(map[string]map[string]interface{}, len(perms))
	for id, p := range perms {
		result[id] = p
	}
	return result
}

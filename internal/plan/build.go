package plan

import (
	"github.com/port-experimental/port-cli/internal/api"
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/resources"
)

// BuildFromDiffResult converts an import diff into an execution plan.
func BuildFromDiffResult(diff *importmodule.DiffResult) *ExecutionPlan {
	if diff == nil {
		return &ExecutionPlan{}
	}

	var steps []Step
	steps = appendBlueprintSteps(steps, OpCreate, diff.BlueprintsToCreate)
	steps = appendBlueprintSteps(steps, OpUpdate, diff.BlueprintsToUpdate)
	steps = appendBlueprintSteps(steps, OpSkip, diff.BlueprintsToSkip)

	steps = appendEntitySteps(steps, OpCreate, diff.EntitiesToCreate)
	steps = appendEntitySteps(steps, OpUpdate, diff.EntitiesToUpdate)
	steps = appendEntitySteps(steps, OpSkip, diff.EntitiesToSkip)

	steps = appendScorecardSteps(steps, OpCreate, diff.ScorecardsToCreate)
	steps = appendScorecardSteps(steps, OpUpdate, diff.ScorecardsToUpdate)
	steps = appendScorecardSteps(steps, OpSkip, diff.ScorecardsToSkip)

	steps = appendActionSteps(steps, OpCreate, diff.ActionsToCreate)
	steps = appendActionSteps(steps, OpUpdate, diff.ActionsToUpdate)
	steps = appendActionSteps(steps, OpSkip, diff.ActionsToSkip)

	steps = appendTeamSteps(steps, OpCreate, diff.TeamsToCreate)
	steps = appendTeamSteps(steps, OpUpdate, diff.TeamsToUpdate)
	steps = appendTeamSteps(steps, OpSkip, diff.TeamsToSkip)

	steps = appendUserSteps(steps, OpCreate, diff.UsersToCreate)
	steps = appendUserSteps(steps, OpUpdate, diff.UsersToUpdate)
	steps = appendUserSteps(steps, OpSkip, diff.UsersToSkip)

	steps = appendPageSteps(steps, OpCreate, diff.PagesToCreate)
	steps = appendPageSteps(steps, OpUpdate, diff.PagesToUpdate)
	steps = appendPageSteps(steps, OpSkip, diff.PagesToSkip)

	steps = appendIntegrationSteps(steps, OpUpdate, diff.IntegrationsToUpdate)
	steps = appendIntegrationSteps(steps, OpSkip, diff.IntegrationsToSkip)

	steps = appendPermissionSteps(steps, resources.KindBlueprintPermissions, diff.BlueprintPermissions)
	steps = appendPermissionSteps(steps, resources.KindActionPermissions, diff.ActionPermissions)
	steps = appendPermissionSteps(steps, resources.KindPagePermissions, diff.PagePermissions)

	return &ExecutionPlan{Steps: steps}
}

func appendBlueprintSteps(steps []Step, op Operation, items []api.Blueprint) []Step {
	return appendMapSteps(steps, resources.KindBlueprints, op, toMaps(items))
}

func appendEntitySteps(steps []Step, op Operation, items []api.Entity) []Step {
	return appendMapSteps(steps, resources.KindEntities, op, toMaps(items))
}

func appendScorecardSteps(steps []Step, op Operation, items []api.Scorecard) []Step {
	return appendMapSteps(steps, resources.KindScorecards, op, toMaps(items))
}

func appendActionSteps(steps []Step, op Operation, items []api.Action) []Step {
	return appendMapSteps(steps, resources.KindActions, op, toMaps(items))
}

func appendTeamSteps(steps []Step, op Operation, items []api.Team) []Step {
	return appendMapSteps(steps, resources.KindTeams, op, toMaps(items))
}

func appendUserSteps(steps []Step, op Operation, items []api.User) []Step {
	return appendMapSteps(steps, resources.KindUsers, op, toMaps(items))
}

func appendPageSteps(steps []Step, op Operation, items []api.Page) []Step {
	return appendMapSteps(steps, resources.KindPages, op, toMaps(items))
}

func appendIntegrationSteps(steps []Step, op Operation, items []api.Integration) []Step {
	return appendMapSteps(steps, resources.KindIntegrations, op, toMaps(items))
}

func appendPermissionSteps(steps []Step, kind resources.ResourceKind, changes []importmodule.PermissionsChange) []Step {
	for _, change := range changes {
		if change.Identifier == "" {
			continue
		}
		steps = append(steps, Step{
			Kind:       kind,
			Operation:  OpPermissionUpdate,
			Identifier: change.Identifier,
			Payload:    map[string]interface{}(change.Permissions),
		})
	}
	return steps
}

func appendMapSteps(steps []Step, kind resources.ResourceKind, op Operation, items []map[string]interface{}) []Step {
	desc := resources.MustGet(kind)
	for _, item := range items {
		id, ok := desc.Identity(item)
		if !ok {
			continue
		}
		steps = append(steps, Step{
			Kind:       kind,
			Operation:  op,
			Identifier: id,
			Payload:    item,
		})
	}
	return steps
}

func toMaps[T ~map[string]interface{}](items []T) []map[string]interface{} {
	out := make([]map[string]interface{}, len(items))
	for i, item := range items {
		out[i] = map[string]interface{}(item)
	}
	return out
}

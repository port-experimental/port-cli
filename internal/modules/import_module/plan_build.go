package import_module

import (
	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

// BuildFromDiffResult converts an import diff into an execution plan.
func BuildFromDiffResult(diff *DiffResult) *plan.ExecutionPlan {
	if diff == nil {
		return &plan.ExecutionPlan{}
	}

	var steps []plan.Step
	steps = appendBlueprintSteps(steps, plan.OpCreate, diff.BlueprintsToCreate)
	steps = appendBlueprintSteps(steps, plan.OpUpdate, diff.BlueprintsToUpdate)
	steps = appendBlueprintSteps(steps, plan.OpSkip, diff.BlueprintsToSkip)

	steps = appendEntitySteps(steps, plan.OpCreate, diff.EntitiesToCreate)
	steps = appendEntitySteps(steps, plan.OpUpdate, diff.EntitiesToUpdate)
	steps = appendEntitySteps(steps, plan.OpSkip, diff.EntitiesToSkip)

	steps = appendScorecardSteps(steps, plan.OpCreate, diff.ScorecardsToCreate)
	steps = appendScorecardSteps(steps, plan.OpUpdate, diff.ScorecardsToUpdate)
	steps = appendScorecardSteps(steps, plan.OpSkip, diff.ScorecardsToSkip)

	steps = appendActionSteps(steps, plan.OpCreate, diff.ActionsToCreate)
	steps = appendActionSteps(steps, plan.OpUpdate, diff.ActionsToUpdate)
	steps = appendActionSteps(steps, plan.OpSkip, diff.ActionsToSkip)

	steps = appendTeamSteps(steps, plan.OpCreate, diff.TeamsToCreate)
	steps = appendTeamSteps(steps, plan.OpUpdate, diff.TeamsToUpdate)
	steps = appendTeamSteps(steps, plan.OpSkip, diff.TeamsToSkip)

	steps = appendUserSteps(steps, plan.OpCreate, diff.UsersToCreate)
	steps = appendUserSteps(steps, plan.OpUpdate, diff.UsersToUpdate)
	steps = appendUserSteps(steps, plan.OpSkip, diff.UsersToSkip)

	steps = appendPageSteps(steps, plan.OpCreate, diff.PagesToCreate)
	steps = appendPageSteps(steps, plan.OpUpdate, diff.PagesToUpdate)
	steps = appendPageSteps(steps, plan.OpSkip, diff.PagesToSkip)

	steps = appendIntegrationSteps(steps, plan.OpUpdate, diff.IntegrationsToUpdate)
	steps = appendIntegrationSteps(steps, plan.OpSkip, diff.IntegrationsToSkip)

	steps = appendPermissionSteps(steps, resources.KindBlueprintPermissions, diff.BlueprintPermissions)
	steps = appendPermissionSteps(steps, resources.KindActionPermissions, diff.ActionPermissions)
	steps = appendPermissionSteps(steps, resources.KindPagePermissions, diff.PagePermissions)

	return &plan.ExecutionPlan{Steps: steps}
}

func appendBlueprintSteps(steps []plan.Step, op plan.Operation, items []api.Blueprint) []plan.Step {
	return appendMapSteps(steps, resources.KindBlueprints, op, toMaps(items))
}

func appendEntitySteps(steps []plan.Step, op plan.Operation, items []api.Entity) []plan.Step {
	return appendMapSteps(steps, resources.KindEntities, op, toMaps(items))
}

func appendScorecardSteps(steps []plan.Step, op plan.Operation, items []api.Scorecard) []plan.Step {
	return appendMapSteps(steps, resources.KindScorecards, op, toMaps(items))
}

func appendActionSteps(steps []plan.Step, op plan.Operation, items []api.Action) []plan.Step {
	return appendMapSteps(steps, resources.KindActions, op, toMaps(items))
}

func appendTeamSteps(steps []plan.Step, op plan.Operation, items []api.Team) []plan.Step {
	return appendMapSteps(steps, resources.KindTeams, op, toMaps(items))
}

func appendUserSteps(steps []plan.Step, op plan.Operation, items []api.User) []plan.Step {
	return appendMapSteps(steps, resources.KindUsers, op, toMaps(items))
}

func appendPageSteps(steps []plan.Step, op plan.Operation, items []api.Page) []plan.Step {
	return appendMapSteps(steps, resources.KindPages, op, toMaps(items))
}

func appendIntegrationSteps(steps []plan.Step, op plan.Operation, items []api.Integration) []plan.Step {
	return appendMapSteps(steps, resources.KindIntegrations, op, toMaps(items))
}

func appendPermissionSteps(steps []plan.Step, kind resources.ResourceKind, changes []PermissionsChange) []plan.Step {
	for _, change := range changes {
		if change.Identifier == "" {
			continue
		}
		steps = append(steps, plan.Step{
			Kind:       kind,
			Operation:  plan.OpPermissionUpdate,
			Identifier: change.Identifier,
			Payload:    map[string]interface{}(change.Permissions),
		})
	}
	return steps
}

func appendMapSteps(steps []plan.Step, kind resources.ResourceKind, op plan.Operation, items []map[string]interface{}) []plan.Step {
	desc := resources.MustGet(kind)
	for _, item := range items {
		id, ok := desc.Identity(item)
		if !ok {
			continue
		}
		steps = append(steps, plan.Step{
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

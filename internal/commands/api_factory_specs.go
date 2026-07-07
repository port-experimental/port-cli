package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func teamsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "teams",
		Short:    "Team operations",
		Singular: "team",
		Plural:   "teams",
	}

	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all teams",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetTeams(ctx)
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new team",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Team created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreateTeam(ctx, api.Team(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [team-name]",
			Short:    "Update an existing team",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Team updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateTeam(ctx, args[0], api.Team(data))
			},
		},
		{
			Name:          "delete",
			Use:           "delete [team-name]",
			Short:         "Delete a team",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Team '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteTeam(ctx, args[0])
			},
		},
	}

	return spec
}

func usersResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "users",
		Short:    "User operations",
		Singular: "user",
		Plural:   "users",
	}

	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all users",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetUsers(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [email]",
			Short:     "Get a specific user by email",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetUser(ctx, args[0])
			},
		},
	}

	return spec
}

func blueprintsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "blueprints",
		Short:    "Blueprint operations",
		Singular: "blueprint",
		Plural:   "blueprints",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all blueprints",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetBlueprints(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [blueprint-id]",
			Short:     "Get a specific blueprint",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetBlueprint(ctx, args[0])
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new blueprint",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Blueprint created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreateBlueprint(ctx, api.Blueprint(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [blueprint-id]",
			Short:    "Update an existing blueprint",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Blueprint updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateBlueprint(ctx, args[0], api.Blueprint(data))
			},
		},
		{
			Name:          "delete",
			Use:           "delete [blueprint-id]",
			Short:         "Delete a blueprint",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Blueprint '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteBlueprint(ctx, args[0])
			},
		},
	}
	return spec
}

func entitiesResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "entities",
		Short:    "Entity operations",
		Singular: "entity",
		Plural:   "entities",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List entities",
			HasFormat: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(false),
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, extra APIExtraValues) (any, error) {
				blueprint := extra.Strings["blueprint"]
				if blueprint != "" {
					return client.GetEntities(ctx, blueprint, nil)
				}
				blueprints, err := client.GetBlueprints(ctx)
				if err != nil {
					return nil, err
				}
				var result []api.Entity
				for _, bp := range blueprints {
					id, ok := bp["identifier"].(string)
					if !ok {
						continue
					}
					entities, err := client.GetEntities(ctx, id, nil)
					if err != nil {
						continue
					}
					result = append(result, entities...)
				}
				return result, nil
			},
		},
		{
			Name:      "get",
			Use:       "get [blueprint-id] [entity-id]",
			Short:     "Get a specific entity",
			Args:      cobra.ExactArgs(2),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetEntity(ctx, args[0], args[1])
			},
		},
		{
			Name:     "create",
			Use:      "create [blueprint-id]",
			Short:    "Create a new entity",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Entity created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreateEntity(ctx, args[0], api.Entity(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [blueprint-id] [entity-id]",
			Short:    "Update an existing entity",
			Args:     cobra.ExactArgs(2),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Entity updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateEntity(ctx, args[0], args[1], api.Entity(data))
			},
		},
		{
			Name:                "delete",
			Use:                 "delete [blueprint-id] [entity-id]",
			Short:               "Delete an entity",
			Args:                cobra.ExactArgs(2),
			HasForce:            true,
			ConfirmDelete:       true,
			ConfirmDeletePrompt: deleteChildFromBlueprintPrompt("entity"),
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Entity '%s' deleted successfully!\n", args[1])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteEntity(ctx, args[0], args[1])
			},
		},
	}
	return spec
}

func pagesResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "pages",
		Short:    "Page operations",
		Singular: "page",
		Plural:   "pages",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all pages",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetPages(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [page-id]",
			Short:     "Get a specific page",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ExtraFlags: []APIExtraFlagSpec{
				{Name: "compact", Bool: true, DefaultBool: true, Usage: "Remove the widgets key from the printed payload"},
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, extra APIExtraValues) (any, error) {
				result, err := client.GetPage(ctx, args[0])
				if err != nil {
					return nil, err
				}
				if extra.Bools["compact"] {
					compacted := make(api.Page, len(result))
					for key, value := range result {
						if key == "widgets" {
							continue
						}
						compacted[key] = value
					}
					result = compacted
				}
				return result, nil
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new page",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Page created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreatePage(ctx, api.Page(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [page-id]",
			Short:    "Update an existing page",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Page updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdatePage(ctx, args[0], api.Page(data))
			},
		},
		{
			Name:          "delete",
			Use:           "delete [page-id]",
			Short:         "Delete a page",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Page '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeletePage(ctx, args[0])
			},
		},
	}
	return spec
}

func webhooksResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "webhooks",
		Short:    "Webhook operations",
		Singular: "webhook",
		Plural:   "webhooks",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all webhooks",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetWebhooks(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [webhook-id]",
			Short:     "Get a specific webhook",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetWebhook(ctx, args[0])
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new webhook",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Webhook created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreateWebhook(ctx, data)
			},
		},
		{
			Name:     "update",
			Use:      "update [webhook-id]",
			Short:    "Update an existing webhook",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Webhook updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateWebhook(ctx, args[0], data)
			},
		},
		{
			Name:          "delete",
			Use:           "delete [webhook-id]",
			Short:         "Delete a webhook",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Webhook '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteWebhook(ctx, args[0])
			},
		},
	}
	return spec
}

func scorecardsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "scorecards",
		Short:    "Scorecard operations",
		Singular: "scorecard",
		Plural:   "scorecards",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List scorecards",
			HasFormat: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(false),
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, extra APIExtraValues) (any, error) {
				if bp := extra.Strings["blueprint"]; bp != "" {
					return client.GetScorecards(ctx, bp)
				}
				return client.GetAllScorecards(ctx)
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new scorecard",
			DataFile: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(true),
			},
			SuccessPrint: func(_ []string) string {
				return "✓ Scorecard created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, extra APIExtraValues) (any, error) {
				return client.CreateScorecard(ctx, extra.Strings["blueprint"], api.Scorecard(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [blueprint-id] [scorecard-id]",
			Short:    "Update an existing scorecard",
			Args:     cobra.ExactArgs(2),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Scorecard updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateScorecard(ctx, args[0], args[1], api.Scorecard(data))
			},
		},
		{
			Name:                "delete",
			Use:                 "delete [blueprint-id] [scorecard-id]",
			Short:               "Delete a scorecard",
			Args:                cobra.ExactArgs(2),
			HasForce:            true,
			ConfirmDelete:       true,
			ConfirmDeletePrompt: deleteChildFromBlueprintPrompt("scorecard"),
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Scorecard '%s' deleted successfully!\n", args[1])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteScorecard(ctx, args[0], args[1])
			},
		},
	}
	return spec
}

func actionsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "actions",
		Short:    "Action operations",
		Singular: "action",
		Plural:   "actions",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List actions",
			HasFormat: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(false),
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, extra APIExtraValues) (any, error) {
				if bp := extra.Strings["blueprint"]; bp != "" {
					return client.GetActions(ctx, bp)
				}
				return client.GetAllActions(ctx)
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new action",
			DataFile: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(true),
			},
			SuccessPrint: func(_ []string) string {
				return "✓ Action created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, extra APIExtraValues) (any, error) {
				return client.CreateAction(ctx, extra.Strings["blueprint"], api.Action(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [blueprint-id] [action-id]",
			Short:    "Update an existing action",
			Args:     cobra.ExactArgs(2),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateAction(ctx, args[0], args[1], api.Action(data))
			},
		},
		{
			Name:                "delete",
			Use:                 "delete [blueprint-id] [action-id]",
			Short:               "Delete an action",
			Args:                cobra.ExactArgs(2),
			HasForce:            true,
			ConfirmDelete:       true,
			ConfirmDeletePrompt: deleteChildFromBlueprintPrompt("action"),
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Action '%s' deleted successfully!\n", args[1])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteAction(ctx, args[0], args[1])
			},
		},
	}
	return spec
}

func permissionsChildSpec(
	name, singular string,
	getFn func(context.Context, string, *api.Client) (api.Permissions, error),
	updateFn func(context.Context, string, api.Permissions, *api.Client) (api.Permissions, error),
) APIResourceSpec {
	return APIResourceSpec{
		Name:     name,
		Short:    name + " permission operations",
		Singular: singular,
		Plural:   name,
		Operations: []APIOperationSpec{
			{
				Name:      "get",
				Use:       "get [id]",
				Short:     "Get permissions for a " + singular,
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get permissions"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return getFn(ctx, args[0], client)
				},
			},
			{
				Name:     "update",
				Use:      "update [id]",
				Short:    "Update permissions for a " + singular,
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Permissions updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update permissions"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return updateFn(ctx, args[0], api.Permissions(data), client)
				},
			},
		},
	}
}

func actionRunsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "action-runs",
		Short:    "Action run operations",
		Singular: "action run",
		Plural:   "action runs",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all action runs",
			HasFormat: true,
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to list action runs"
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetActionRuns(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [run-id]",
			Short:     "Get a specific action run",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to get action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetActionRun(ctx, args[0])
			},
		},
		{
			Name:     "update",
			Use:      "update [run-id]",
			Short:    "Update an action run",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action run updated successfully!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to update action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateActionRun(ctx, args[0], data)
			},
		},
		{
			Name:     "approve",
			Use:      "approve [run-id]",
			Short:    "Approve or decline an action run",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action run approval submitted!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to approve action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.ApproveActionRun(ctx, args[0], data)
			},
		},
		{
			Name:     "execute",
			Use:      "execute [action-id]",
			Short:    "Execute an action (create a new action run)",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action executed successfully!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to execute action"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.ExecuteAction(ctx, args[0], data)
			},
		},
	}
	return spec
}

func auditResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "audit",
		Short:    "Audit log operations",
		Singular: "audit log entry",
		Plural:   "audit log entries",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List audit log entries",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list audit logs"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetAuditLogs(ctx)
				},
			},
		},
	}
}

func agentsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "agents",
		Short:    "Agent operations",
		Singular: "agent",
		Plural:   "agents",
		Operations: []APIOperationSpec{
			{
				Name:     "invoke",
				Use:      "invoke [agent-id]",
				Short:    "Invoke an agent",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to invoke agent"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "POST",
						Endpoint: fmt.Sprintf("/agent/%s/invoke", args[0]),
						Data:     data,
					})
				},
			},
		},
	}
}

func aiResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "ai",
		Short:    "Port AI operations",
		Singular: "AI invocation",
		Plural:   "AI invocations",
		Operations: []APIOperationSpec{
			{
				Name:     "invoke",
				Use:      "invoke",
				Short:    "Invoke Port AI",
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to invoke AI"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "POST",
						Endpoint: "/ai/invoke",
						Data:     data,
					})
				},
			},
			{
				Name:      "get",
				Use:       "get [invocation-id]",
				Short:     "Get an AI invocation result",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get AI invocation"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "GET",
						Endpoint: fmt.Sprintf("/ai/invoke/%s", args[0]),
					})
				},
			},
		},
	}
}

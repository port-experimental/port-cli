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

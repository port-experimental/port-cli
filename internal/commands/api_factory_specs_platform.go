package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

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

func integrationsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "integrations",
		Short:    "Integration operations",
		Singular: "integration",
		Plural:   "integrations",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List all integrations",
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to list %s", s.Plural)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetIntegrations(ctx)
				},
			},
			{
				Name:      "get",
				Use:       "get [identifier]",
				Short:     "Get a specific integration",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to get %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetIntegration(ctx, args[0])
				},
			},
			{
				Name:     "update",
				Use:      "update [identifier]",
				Short:    "Update an integration",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Integration updated successfully!\n"
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to update %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateIntegration(ctx, args[0], api.Integration(data))
				},
			},
			{
				Name:     "update-config",
				Use:      "update-config [identifier]",
				Short:    "Update an integration's config",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Integration config updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update integration config"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateIntegrationConfig(ctx, args[0], data)
				},
			},
			{
				Name:          "delete",
				Use:           "delete [identifier]",
				Short:         "Delete an integration",
				Args:          cobra.ExactArgs(1),
				HasForce:      true,
				ConfirmDelete: true,
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ Integration '%s' deleted successfully!\n", args[0])
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to delete %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DeleteIntegration(ctx, args[0])
				},
			},
		},
	}
}

func migrationsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "migrations",
		Short:    "Blueprint property migration operations",
		Singular: "migration",
		Plural:   "migrations",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List all migrations",
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to list %s", s.Plural)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMigrations(ctx, nil)
				},
			},
			{
				Name:      "get",
				Use:       "get [identifier]",
				Short:     "Get a specific migration",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to get %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMigration(ctx, args[0])
				},
			},
			{
				Name:     "create",
				Use:      "create",
				Short:    "Create a blueprint property migration",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Migration created successfully!\n"
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to create %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					req, err := migrationRequestFromData(data)
					if err != nil {
						return nil, err
					}
					return client.CreateMigration(ctx, req)
				},
			},
			{
				Name:     "cancel",
				Use:      "cancel [identifier]",
				Short:    "Cancel a running migration",
				Args:     cobra.ExactArgs(1),
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ Migration '%s' cancel requested!\n", args[0])
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to cancel %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CancelMigration(ctx, args[0])
				},
			},
		},
	}
}

func migrationRequestFromData(data map[string]interface{}) (api.MigrationRequest, error) {
	req := api.MigrationRequest{}
	source, ok := data["sourceBlueprint"].(string)
	if !ok || source == "" {
		return req, fmt.Errorf("migration data requires string field \"sourceBlueprint\"")
	}
	mapping, ok := data["mapping"].(map[string]interface{})
	if !ok {
		return req, fmt.Errorf("migration data requires object field \"mapping\"")
	}
	req.SourceBlueprint = source
	req.Mapping = mapping
	return req, nil
}

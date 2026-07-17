package commands

import (
	"context"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func llmProvidersResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "llm-providers",
		Short:    "LLM provider operations",
		Singular: "LLM provider",
		Plural:   "LLM providers",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List configured LLM providers",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list LLM providers"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetLLMProviders(ctx)
				},
			},
			{
				Name:     "create",
				Use:      "create",
				Short:    "Create or connect an LLM provider",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ LLM provider created successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to create LLM provider"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CreateLLMProvider(ctx, api.LLMProvider(data))
				},
			},
			{
				Name:      "get-defaults",
				Use:       "get-defaults",
				Short:     "Get the default LLM provider and model",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get LLM provider defaults"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetLLMProviderDefaults(ctx)
				},
			},
			{
				Name:     "set-defaults",
				Use:      "set-defaults",
				Short:    "Set the default LLM provider and model",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ LLM provider defaults updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to set LLM provider defaults"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.SetLLMProviderDefaults(ctx, data)
				},
			},
		},
	}
}

func memoryResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "memory",
		Short:    "Memory record and settings operations",
		Singular: "memory record",
		Plural:   "memory records",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List memory records",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list memory records"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMemoryRecords(ctx, nil)
				},
			},
			{
				Name:          "delete",
				Use:           "delete",
				Short:         "Delete memory records",
				DataFile:      true,
				HasForce:      true,
				ConfirmDelete: true,
				ConfirmDeletePrompt: func(_ []string) string {
					return "Are you sure you want to delete memory records? [y/N]: "
				},
				SuccessPrint: func(_ []string) string {
					return "✓ Memory records deleted successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to delete memory records"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DeleteMemoryRecords(ctx, data)
				},
			},
			{
				Name:      "get-settings",
				Use:       "get-settings",
				Short:     "Get memory settings",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get memory settings"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMemorySettings(ctx)
				},
			},
			{
				Name:     "update-settings",
				Use:      "update-settings",
				Short:    "Update memory settings",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Memory settings updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update memory settings"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateMemorySettings(ctx, data)
				},
			},
		},
	}
}

func autoDiscoveryResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "auto-discovery",
		Short:    "Catalog auto-discovery operations",
		Singular: "auto-discovery invocation",
		Plural:   "auto-discovery invocations",
		Operations: []APIOperationSpec{
			{
				Name:     "create",
				Use:      "create",
				Short:    "Create an auto-discovery invocation",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Auto-discovery invocation created successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to create auto-discovery invocation"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CreateAutoDiscoveryInvocation(ctx, data)
				},
			},
			{
				Name:      "active",
				Use:       "active",
				Short:     "List active auto-discovery invocations",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list active auto-discovery invocations"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetActiveAutoDiscoveryInvocations(ctx)
				},
			},
			{
				Name:      "latest",
				Use:       "latest [blueprint-identifier]",
				Short:     "Get the latest auto-discovery invocation for a blueprint",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get latest auto-discovery invocation"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetLatestAutoDiscoveryInvocation(ctx, args[0])
				},
			},
			{
				Name:      "suggestions",
				Use:       "suggestions [invocation-id]",
				Short:     "Get suggestions for an auto-discovery invocation",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get auto-discovery suggestions"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetAutoDiscoverySuggestions(ctx, args[0])
				},
			},
			{
				Name:     "review",
				Use:      "review [invocation-id]",
				Short:    "Review auto-discovery invocation suggestions",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Auto-discovery suggestions reviewed successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to review auto-discovery suggestions"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.ReviewAutoDiscoverySuggestions(ctx, args[0], data)
				},
			},
			{
				Name:     "update-suggestion",
				Use:      "update-suggestion [invocation-id] [entity-identifier]",
				Short:    "Update a suggestion for an auto-discovery invocation",
				Args:     cobra.ExactArgs(2),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Auto-discovery suggestion updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update auto-discovery suggestion"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateAutoDiscoverySuggestion(ctx, args[0], args[1], data)
				},
			},
		},
	}
}

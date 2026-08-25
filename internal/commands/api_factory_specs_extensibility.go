package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func mcpResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "mcp",
		Short:    "MCP connector operations",
		Singular: "MCP server",
		Plural:   "MCP servers",
		Operations: []APIOperationSpec{
			{
				Name:      "list-servers",
				Use:       "list-servers",
				Short:     "List MCP servers for the user",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list MCP servers"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMCPServers(ctx)
				},
			},
			{
				Name:      "get-server",
				Use:       "get-server [server-id]",
				Short:     "Get an MCP server by ID",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get MCP server"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMCPServer(ctx, args[0])
				},
			},
			{
				Name:          "disconnect",
				Use:           "disconnect [server-id]",
				Short:         "Disconnect an MCP server",
				Args:          cobra.ExactArgs(1),
				HasForce:      true,
				ConfirmDelete: true,
				ConfirmDeletePrompt: func(args []string) string {
					return fmt.Sprintf("Are you sure you want to disconnect MCP server '%s'? [y/N]: ", args[0])
				},
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ MCP server '%s' disconnected successfully!\n", args[0])
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to disconnect MCP server"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DisconnectMCPServer(ctx, args[0])
				},
			},
			{
				Name:      "list-templates",
				Use:       "list-templates",
				Short:     "List MCP server templates",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list MCP server templates"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMCPServerTemplates(ctx)
				},
			},
			{
				Name:      "list-port-tools",
				Use:       "list-port-tools",
				Short:     "List Port MCP tools",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list Port MCP tools"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetPortMCPTools(ctx)
				},
			},
			{
				Name:      "list-tools",
				Use:       "list-tools [server-id]",
				Short:     "List tools for an MCP server",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list MCP server tools"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMCPServerTools(ctx, args[0])
				},
			},
			{
				Name:     "call-tool",
				Use:      "call-tool [server-id] [tool-name]",
				Short:    "Call a tool on an MCP server",
				Args:     cobra.ExactArgs(2),
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to call MCP server tool"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CallMCPServerTool(ctx, args[0], args[1], data)
				},
			},
			{
				Name:      "session-token",
				Use:       "session-token [server-id]",
				Short:     "Get an OAuth2 session token for an MCP server",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get MCP OAuth2 session token"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetMCPOAuth2SessionToken(ctx, args[0])
				},
			},
		},
	}
}

func appsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "apps",
		Short:    "App credentials operations",
		Singular: "app credentials set",
		Plural:   "app credentials sets",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List all credentials sets",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list apps"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetApps(ctx)
				},
			},
			{
				Name:     "update",
				Use:      "update [app-id]",
				Short:    "Update a credentials set",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ App credentials updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update app"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateApp(ctx, args[0], data)
				},
			},
			{
				Name:          "delete",
				Use:           "delete [app-id]",
				Short:         "Delete a credentials set",
				Args:          cobra.ExactArgs(1),
				HasForce:      true,
				ConfirmDelete: true,
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ App credentials '%s' deleted successfully!\n", args[0])
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to delete app"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DeleteApp(ctx, args[0])
				},
			},
			{
				Name:      "rotate-secret",
				Use:       "rotate-secret [app-id]",
				Short:     "Rotate the secret for an app credentials set",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to rotate app secret"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.RotateAppSecret(ctx, args[0])
				},
			},
			{
				Name:      "rotate-user-credentials",
				Use:       "rotate-user-credentials [user-email]",
				Short:     "Rotate credentials for a user",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to rotate user credentials"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.RotateUserCredentials(ctx, args[0])
				},
			},
		},
	}
}

func pluginsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "plugins",
		Short:    "Plugin operations",
		Singular: "plugin",
		Plural:   "plugins",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List all plugins",
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to list %s", s.Plural)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetPlugins(ctx)
				},
			},
			{
				Name:      "get",
				Use:       "get [identifier]",
				Short:     "Get a plugin by identifier",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to get %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetPlugin(ctx, args[0])
				},
			},
			{
				Name:     "update",
				Use:      "update [identifier]",
				Short:    "Update plugin metadata",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Plugin updated successfully!\n"
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to update %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdatePlugin(ctx, args[0], data)
				},
			},
			{
				Name:          "delete",
				Use:           "delete [identifier]",
				Short:         "Delete a plugin",
				Args:          cobra.ExactArgs(1),
				HasForce:      true,
				ConfirmDelete: true,
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ Plugin '%s' deleted successfully!\n", args[0])
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to delete %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DeletePlugin(ctx, args[0])
				},
			},
			{
				Name:     "upload-url",
				Use:      "upload-url",
				Short:    "Get a presigned URL for uploading a plugin",
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get plugin upload URL"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CreatePluginUploadURL(ctx, data)
				},
			},
			{
				Name:     "update-upload-url",
				Use:      "update-upload-url [identifier]",
				Short:    "Get a presigned URL for updating a plugin",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get plugin update upload URL"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdatePluginUploadURL(ctx, args[0], data)
				},
			},
			{
				Name:     "finalize-upload",
				Use:      "finalize-upload",
				Short:    "Finalize a plugin upload",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Plugin upload finalized successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to finalize plugin upload"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.FinalizePluginUpload(ctx, data)
				},
			},
			{
				Name:     "install",
				Use:      "install",
				Short:    "Install a plugin from the Port public registry",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Plugin installed successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to install plugin"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.InstallPlugin(ctx, data)
				},
			},
		},
	}
}

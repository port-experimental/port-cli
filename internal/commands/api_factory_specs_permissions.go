package commands

import (
	"context"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func permissionsChildSpec(
	name, singular string,
	getFn func(*api.Client, context.Context, string) (api.Permissions, error),
	updateFn func(*api.Client, context.Context, string, api.Permissions) (api.Permissions, error),
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
					return getFn(client, ctx, args[0])
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
					return updateFn(client, ctx, args[0], api.Permissions(data))
				},
			},
		},
	}
}

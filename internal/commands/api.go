package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func formatOutput(data interface{}, format string) error {
	if err := validateStringEnum("--format", format, []string{"json", "yaml"}); err != nil {
		return err
	}
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		defer encoder.Close()
		return encoder.Encode(data)
	default:
		// Print as-is
		fmt.Printf("%+v\n", data)
		return nil
	}
}

func getOrRefreshCommandToken(cmd *cobra.Command, configManager *config.ConfigManager, org string) (*auth.Token, error) {
	return getOrRefreshToken(cmd.Context(), configManager, org)
}

func clientForAPICommand(ctx context.Context, org string) (*api.Client, error) {
	rt := NewRuntime(ctx)
	client, _, err := rt.ClientForOrg(ctx, org)
	return client, err
}

// RegisterAPI registers the API command and all subcommands.
func RegisterAPI(rootCmd *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Direct Port API operations",
		Long:  "Direct Port API operations for blueprints, entities, pages, etc.",
	}

	apiCmd.AddCommand(registerGenericAPICall())
	apiCmd.AddCommand(registerAPIResource(blueprintsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(entitiesResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(pagesResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(teamsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(usersResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(scorecardsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(actionsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(webhooksResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(actionRunsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(auditResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(agentsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(aiResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(integrationsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(migrationsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(organizationResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(secretsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(workflowsResourceSpec()))
	apiCmd.AddCommand(registerAPIResource(workflowRunsResourceSpec()))

	permissionsCmd := &cobra.Command{
		Use:   "permissions",
		Short: "Permission operations for blueprints, actions, and pages",
	}
	permissionsCmd.AddCommand(registerAPIResource(permissionsChildSpec(
		"blueprints", "blueprint",
		(*api.Client).GetBlueprintPermissions,
		(*api.Client).UpdateBlueprintPermissions,
	)))
	permissionsCmd.AddCommand(registerAPIResource(permissionsChildSpec(
		"actions", "action",
		(*api.Client).GetActionPermissions,
		(*api.Client).UpdateActionPermissions,
	)))
	permissionsCmd.AddCommand(registerAPIResource(permissionsChildSpec(
		"pages", "page",
		(*api.Client).GetPagePermissions,
		(*api.Client).UpdatePagePermissions,
	)))
	apiCmd.AddCommand(permissionsCmd)

	rootCmd.AddCommand(apiCmd)
}

func registerGenericAPICall() *cobra.Command {
	var method, org, format, data, unwrap string

	cmd := &cobra.Command{
		Use:   "call",
		Short: "Generic API operations",
		Long:  "Generic API operations. Prints the raw Port API response envelope by default; use --unwrap to print a top-level response field such as blueprints.",
		Example: ` # get raw blueprints response envelope
port api call /blueprints

# print only the blueprints array from the raw response
port api call /blueprints --unwrap blueprints

# trigger an action
port api call /actions/my-action/runs --data '{"properties": {}}'

# get action runs for org
port api call /actions/runs --org my-org`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientForAPICommand(cmd.Context(), org)
			if err != nil {
				return err
			}
			defer client.Close()

			endpoint := args[0]

			if method == "" {
				if data == "" {
					method = "GET"
				} else {
					method = "POST"
				}
			}

			var parsedData map[string]any
			if data == "" {
				parsedData = nil
			} else {
				err := json.Unmarshal([]byte(data), &parsedData)
				if err != nil {
					return fmt.Errorf("failed encoding body (%w)", err)
				}
			}
			result, err := client.Request(cmd.Context(), api.RequestParams{Method: method, Endpoint: endpoint, Data: parsedData})
			if err != nil {
				return fmt.Errorf("failed to perform request %s to %s (%w)", method, endpoint, err)
			}
			if unwrap != "" {
				responseMap, ok := result.(map[string]interface{})
				if !ok {
					return fmt.Errorf("failed to unwrap %q: response is not a JSON object", unwrap)
				}
				value, ok := responseMap[unwrap]
				if !ok {
					return fmt.Errorf("failed to unwrap %q from response", unwrap)
				}
				return formatOutput(value, format)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&method, "method", "X", "", `The HTTP method for the request (default "GET")`)
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")
	cmd.Flags().StringVar(&data, "data", "", "Data passed in the request body")
	cmd.Flags().StringVar(&unwrap, "unwrap", "", "Print only this top-level field from the raw API response envelope")

	return cmd
}

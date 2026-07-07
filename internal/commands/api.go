package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	agentsCmd := &cobra.Command{
		Use:   "agents",
		Short: "Agent operations",
	}
	agentsCmd.AddCommand(registerAgentInvoke())
	apiCmd.AddCommand(agentsCmd)

	aiCmd := &cobra.Command{
		Use:   "ai",
		Short: "Port AI operations",
	}
	aiCmd.AddCommand(registerAIInvoke())
	aiCmd.AddCommand(registerAIGet())
	apiCmd.AddCommand(aiCmd)

	rootCmd.AddCommand(apiCmd)
}

// registerAgentInvoke registers the agent invoke command.
func registerAgentInvoke() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "invoke [agent-id]",
		Short: "Invoke an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			client, err := clientForAPICommand(cmd.Context(), org)
			if err != nil {
				return err
			}
			defer client.Close()

			result, err := client.Request(cmd.Context(), api.RequestParams{
				Method:   "POST",
				Endpoint: fmt.Sprintf("/agent/%s/invoke", agentID),
				Data:     data,
			})
			if err != nil {
				return fmt.Errorf("failed to invoke agent: %w", err)
			}

			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVar(&dataFile, "data", "", "JSON file with invocation body")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerAIInvoke registers the AI invoke command.
func registerAIInvoke() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke Port AI",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			client, err := clientForAPICommand(cmd.Context(), org)
			if err != nil {
				return err
			}
			defer client.Close()

			result, err := client.Request(cmd.Context(), api.RequestParams{
				Method:   "POST",
				Endpoint: "/ai/invoke",
				Data:     data,
			})
			if err != nil {
				return fmt.Errorf("failed to invoke AI: %w", err)
			}

			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVar(&dataFile, "data", "", "JSON file with AI invocation body")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerAIGet registers the AI get invocation command.
func registerAIGet() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "get [invocation-id]",
		Short: "Get an AI invocation result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			invocationID := args[0]
			client, err := clientForAPICommand(cmd.Context(), org)
			if err != nil {
				return err
			}
			defer client.Close()

			result, err := client.Request(cmd.Context(), api.RequestParams{
				Method:   "GET",
				Endpoint: fmt.Sprintf("/ai/invoke/%s", invocationID),
			})
			if err != nil {
				return fmt.Errorf("failed to get AI invocation: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// loadJSONFile loads a JSON file and returns its contents as a map.
func loadJSONFile(filePath string) (map[string]interface{}, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("data file not found: %s", filePath)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
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

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

// formatOutput formats and displays output data.
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

// RegisterAPI registers the API command and all subcommands.
func RegisterAPI(rootCmd *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Direct Port API operations",
		Long:  "Direct Port API operations for blueprints, entities, pages, etc.",
	}

	apiCmd.AddCommand(registerGenericAPICall())

	blueprintsCmd := registerAPIResource(blueprintsResourceSpec())
	entitiesCmd := registerAPIResource(entitiesResourceSpec())
	pagesCmd := registerAPIResource(pagesResourceSpec())
	teamsCmd := registerAPIResource(teamsResourceSpec())
	usersCmd := registerAPIResource(usersResourceSpec())
	scorecardsCmd := registerAPIResource(scorecardsResourceSpec())
	actionsCmd := registerAPIResource(actionsResourceSpec())

	// Permissions subcommands
	permissionsCmd := &cobra.Command{
		Use:   "permissions",
		Short: "Permission operations for blueprints, actions, and pages",
	}

	permissionsCmd.AddCommand(registerPermissionsResourceCmd(
		"blueprints",
		func(ctx context.Context, id string, c *api.Client) (api.Permissions, error) {
			return c.GetBlueprintPermissions(ctx, id)
		},
		func(ctx context.Context, id string, p api.Permissions, c *api.Client) (api.Permissions, error) {
			return c.UpdateBlueprintPermissions(ctx, id, p)
		},
	))
	permissionsCmd.AddCommand(registerPermissionsResourceCmd(
		"actions",
		func(ctx context.Context, id string, c *api.Client) (api.Permissions, error) {
			return c.GetActionPermissions(ctx, id)
		},
		func(ctx context.Context, id string, p api.Permissions, c *api.Client) (api.Permissions, error) {
			return c.UpdateActionPermissions(ctx, id, p)
		},
	))
	permissionsCmd.AddCommand(registerPermissionsResourceCmd(
		"pages",
		func(ctx context.Context, id string, c *api.Client) (api.Permissions, error) {
			return c.GetPagePermissions(ctx, id)
		},
		func(ctx context.Context, id string, p api.Permissions, c *api.Client) (api.Permissions, error) {
			return c.UpdatePagePermissions(ctx, id, p)
		},
	))

	// Agents subcommands
	agentsCmd := &cobra.Command{
		Use:   "agents",
		Short: "Agent operations",
	}
	agentsCmd.AddCommand(registerAgentInvoke())

	// AI subcommands
	aiCmd := &cobra.Command{
		Use:   "ai",
		Short: "Port AI operations",
	}
	aiCmd.AddCommand(registerAIInvoke())
	aiCmd.AddCommand(registerAIGet())

	// Action runs subcommands
	actionRunsCmd := &cobra.Command{
		Use:   "action-runs",
		Short: "Action run operations",
	}
	actionRunsCmd.AddCommand(registerActionRunList())
	actionRunsCmd.AddCommand(registerActionRunGet())
	actionRunsCmd.AddCommand(registerActionRunUpdate())
	actionRunsCmd.AddCommand(registerActionRunApprove())
	actionRunsCmd.AddCommand(registerActionRunExecute())

	// Webhooks subcommands
	webhooksCmd := registerAPIResource(webhooksResourceSpec())

	// Audit subcommands
	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit log operations",
	}
	auditCmd.AddCommand(registerAuditList())

	apiCmd.AddCommand(blueprintsCmd)
	apiCmd.AddCommand(entitiesCmd)
	apiCmd.AddCommand(pagesCmd)
	apiCmd.AddCommand(teamsCmd)
	apiCmd.AddCommand(usersCmd)
	apiCmd.AddCommand(scorecardsCmd)
	apiCmd.AddCommand(actionsCmd)
	apiCmd.AddCommand(permissionsCmd)
	apiCmd.AddCommand(agentsCmd)
	apiCmd.AddCommand(aiCmd)
	apiCmd.AddCommand(actionRunsCmd)
	apiCmd.AddCommand(webhooksCmd)
	apiCmd.AddCommand(auditCmd)

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
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
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
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
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
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
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

// registerActionRunList registers the action run list command.
func registerActionRunList() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all action runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.GetActionRuns(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list action runs: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// registerActionRunGet registers the action run get command.
func registerActionRunGet() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "get [run-id]",
		Short: "Get a specific action run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.GetActionRun(cmd.Context(), runID)
			if err != nil {
				return fmt.Errorf("failed to get action run: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// registerActionRunUpdate registers the action run update command.
func registerActionRunUpdate() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "update [run-id]",
		Short: "Update an action run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.UpdateActionRun(cmd.Context(), runID, data)
			if err != nil {
				return fmt.Errorf("failed to update action run: %w", err)
			}

			cmd.Printf("✓ Action run updated successfully!\n")
			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVar(&dataFile, "data", "", "JSON file with action run update data")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerActionRunApprove registers the action run approve command.
func registerActionRunApprove() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "approve [run-id]",
		Short: "Approve or decline an action run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.ApproveActionRun(cmd.Context(), runID, data)
			if err != nil {
				return fmt.Errorf("failed to approve action run: %w", err)
			}

			cmd.Printf("✓ Action run approval submitted!\n")
			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVar(&dataFile, "data", "", "JSON file with approval data (e.g. {\"status\":\"APPROVED\",\"description\":\"...\"})")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerActionRunExecute registers the action execute command.
func registerActionRunExecute() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "execute [action-id]",
		Short: "Execute an action (create a new action run)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			actionID := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.ExecuteAction(cmd.Context(), actionID, data)
			if err != nil {
				return fmt.Errorf("failed to execute action: %w", err)
			}

			cmd.Printf("✓ Action executed successfully!\n")
			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVar(&dataFile, "data", "", "JSON file with action run body")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerAuditList registers the audit log list command.
func registerAuditList() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List audit log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, org)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := client.GetAuditLogs(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list audit logs: %w", err)
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
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				org,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
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

func registerPermissionsResourceCmd(
	resourceName string,
	getFunc func(ctx context.Context, id string, client *api.Client) (api.Permissions, error),
	updateFunc func(ctx context.Context, id string, perms api.Permissions, client *api.Client) (api.Permissions, error),
) *cobra.Command {
	singular := resourceName[:len(resourceName)-1]

	resourceCmd := &cobra.Command{
		Use:   resourceName,
		Short: resourceName + " permission operations",
	}

	// get subcommand
	var getOrg, getFormat string
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get permissions for a " + singular,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, getOrg)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(getOrg)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := getFunc(cmd.Context(), id, client)
			if err != nil {
				return fmt.Errorf("failed to get permissions: %w", err)
			}

			return formatOutput(result, getFormat)
		},
	}
	getCmd.Flags().StringVar(&getOrg, "org", "", "Organization name (uses default if not specified)")
	getCmd.Flags().StringVarP(&getFormat, "format", "f", "json", "Output format: json, yaml")

	// update subcommand
	var updateOrg, updateDataFile string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update permissions for a " + singular,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, updateOrg)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(updateOrg)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}
			data, err := loadJSONFile(updateDataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}
			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}
			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
				Timeout:      0,
			})
			defer client.Close()

			result, err := updateFunc(cmd.Context(), id, api.Permissions(data), client)
			if err != nil {
				return fmt.Errorf("failed to update permissions: %w", err)
			}

			cmd.Printf("✓ Permissions updated successfully!\n")
			return formatOutput(result, "json")
		},
	}
	updateCmd.Flags().StringVar(&updateOrg, "org", "", "Organization name (uses default if not specified)")
	updateCmd.Flags().StringVar(&updateDataFile, "data", "", "JSON file with permissions data")
	updateCmd.MarkFlagRequired("data")

	resourceCmd.AddCommand(getCmd)
	resourceCmd.AddCommand(updateCmd)

	return resourceCmd
}

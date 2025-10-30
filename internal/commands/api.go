package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/port-labs/port-cli/internal/api"
	"github.com/port-labs/port-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// formatOutput formats and displays output data.
func formatOutput(data interface{}, format string) error {
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

// RegisterAPI registers the API command and all subcommands.
func RegisterAPI(rootCmd *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Direct Port API operations",
		Long:  "Direct Port API operations for blueprints and entities",
	}

	// Blueprint subcommands
	blueprintsCmd := &cobra.Command{
		Use:   "blueprints",
		Short: "Blueprint operations",
	}

	blueprintsCmd.AddCommand(registerBlueprintList())
	blueprintsCmd.AddCommand(registerBlueprintGet())
	blueprintsCmd.AddCommand(registerBlueprintCreate())
	blueprintsCmd.AddCommand(registerBlueprintDelete())

	// Entity subcommands
	entitiesCmd := &cobra.Command{
		Use:   "entities",
		Short: "Entity operations",
	}

	entitiesCmd.AddCommand(registerEntityList())
	entitiesCmd.AddCommand(registerEntityGet())
	entitiesCmd.AddCommand(registerEntityCreate())
	entitiesCmd.AddCommand(registerEntityDelete())

	apiCmd.AddCommand(blueprintsCmd)
	apiCmd.AddCommand(entitiesCmd)

	rootCmd.AddCommand(apiCmd)
}

// registerBlueprintList registers the blueprint list command.
func registerBlueprintList() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all blueprints",
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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			result, err := client.GetBlueprints(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list blueprints: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// registerBlueprintGet registers the blueprint get command.
func registerBlueprintGet() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "get [blueprint-id]",
		Short: "Get a specific blueprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintID := args[0]
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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			result, err := client.GetBlueprint(cmd.Context(), blueprintID)
			if err != nil {
				return fmt.Errorf("failed to get blueprint: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// registerBlueprintCreate registers the blueprint create command.
func registerBlueprintCreate() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new blueprint",
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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			// Load data file
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			result, err := client.CreateBlueprint(cmd.Context(), api.Blueprint(data))
			if err != nil {
				return fmt.Errorf("failed to create blueprint: %w", err)
			}

			cmd.Printf("✓ Blueprint created successfully!\n")
			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&dataFile, "data", "d", "", "JSON file with blueprint data")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerBlueprintDelete registers the blueprint delete command.
func registerBlueprintDelete() *cobra.Command {
	var org string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [blueprint-id]",
		Short: "Delete a blueprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintID := args[0]

			if !force {
				confirm, err := cmd.Flags().GetBool("yes")
				if err != nil || !confirm {
					cmd.Printf("Are you sure you want to delete blueprint '%s'? [y/N]: ", blueprintID)
					var response string
					fmt.Scanln(&response)
					if response != "y" && response != "Y" {
						cmd.Println("Operation cancelled")
						return nil
					}
				}
			}

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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			if err := client.DeleteBlueprint(cmd.Context(), blueprintID); err != nil {
				return fmt.Errorf("failed to delete blueprint: %w", err)
			}

			cmd.Printf("✓ Blueprint '%s' deleted successfully!\n", blueprintID)
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

// registerEntityList registers the entity list command.
func registerEntityList() *cobra.Command {
	var org, format, blueprint string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entities",
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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			var result []api.Entity
			if blueprint != "" {
				entities, err := client.GetEntities(cmd.Context(), blueprint, nil)
				if err != nil {
					return fmt.Errorf("failed to list entities: %w", err)
				}
				result = entities
			} else {
				// Get all blueprints and then all entities
				blueprints, err := client.GetBlueprints(cmd.Context())
				if err != nil {
					return fmt.Errorf("failed to get blueprints: %w", err)
				}

				for _, bp := range blueprints {
					if identifier, ok := bp["identifier"].(string); ok {
						entities, err := client.GetEntities(cmd.Context(), identifier, nil)
						if err != nil {
							continue // Skip blueprints without entities
						}
						result = append(result, entities...)
					}
				}
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")
	cmd.Flags().StringVarP(&blueprint, "blueprint", "b", "", "Filter by blueprint ID")

	return cmd
}

// registerEntityGet registers the entity get command.
func registerEntityGet() *cobra.Command {
	var org, format string

	cmd := &cobra.Command{
		Use:   "get [blueprint-id] [entity-id]",
		Short: "Get a specific entity",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintID := args[0]
			entityID := args[1]

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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			result, err := client.GetEntity(cmd.Context(), blueprintID, entityID)
			if err != nil {
				return fmt.Errorf("failed to get entity: %w", err)
			}

			return formatOutput(result, format)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")

	return cmd
}

// registerEntityCreate registers the entity create command.
func registerEntityCreate() *cobra.Command {
	var org, dataFile string

	cmd := &cobra.Command{
		Use:   "create [blueprint-id]",
		Short: "Create a new entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintID := args[0]

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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			// Load data file
			data, err := loadJSONFile(dataFile)
			if err != nil {
				return fmt.Errorf("failed to load data file: %w", err)
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			result, err := client.CreateEntity(cmd.Context(), blueprintID, api.Entity(data))
			if err != nil {
				return fmt.Errorf("failed to create entity: %w", err)
			}

			cmd.Printf("✓ Entity created successfully!\n")
			return formatOutput(result, "json")
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&dataFile, "data", "d", "", "JSON file with entity data")
	cmd.MarkFlagRequired("data")

	return cmd
}

// registerEntityDelete registers the entity delete command.
func registerEntityDelete() *cobra.Command {
	var org string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [blueprint-id] [entity-id]",
		Short: "Delete an entity",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintID := args[0]
			entityID := args[1]

			if !force {
				cmd.Printf("Are you sure you want to delete entity '%s' from blueprint '%s'? [y/N]: ", entityID, blueprintID)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					cmd.Println("Operation cancelled")
					return nil
				}
			}

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

			orgConfig, err := cfg.GetOrgConfig(org)
			if err != nil {
				return err
			}

			client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
			defer client.Close()

			if err := client.DeleteEntity(cmd.Context(), blueprintID, entityID); err != nil {
				return fmt.Errorf("failed to delete entity: %w", err)
			}

			cmd.Printf("✓ Entity '%s' deleted successfully!\n", entityID)
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

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

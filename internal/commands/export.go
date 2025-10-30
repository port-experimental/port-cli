package commands

import (
	"fmt"
	"strings"

	"github.com/port-labs/port-cli/internal/config"
	"github.com/port-labs/port-cli/internal/modules/export"
	"github.com/spf13/cobra"
)

// RegisterExport registers the export command.
func RegisterExport(rootCmd *cobra.Command) {
	var (
		output        string
		org           string
		baseOrg       string
		blueprints    string
		format        string
		skipEntities  bool
		include       string
	)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export data from Port",
		Long: `Export data from Port organization.

Exports blueprints, entities, scorecards, actions, and teams to a file.
Use --skip-entities to only export configuration without entity data.
Use --include to selectively export specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			// Use base-org if provided, otherwise use org
			orgName := baseOrg
			if orgName == "" {
				orgName = org
			}

			_, baseOrgConfig, _, err := configManager.LoadWithDualOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				orgName,
				"", "", "", "", // No target org for export
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if baseOrgConfig == nil {
				return fmt.Errorf("base organization configuration not found")
			}

			orgConfig := baseOrgConfig

			// Parse blueprints list
			var blueprintList []string
			if blueprints != "" {
				blueprintList = strings.Split(blueprints, ",")
				for i := range blueprintList {
					blueprintList[i] = strings.TrimSpace(blueprintList[i])
				}
			}

			// Parse include list
			var includeList []string
			if include != "" {
				includeList = strings.Split(include, ",")
				for i := range includeList {
					includeList[i] = strings.TrimSpace(includeList[i])
				}

				// Validate resource types
				validResources := map[string]bool{
					"blueprints":   true,
					"entities":      true,
					"scorecards":    true,
					"actions":       true,
					"teams":         true,
					"users":         true,
					"automations":   true,
					"pages":         true,
					"integrations":  true,
				}

				for _, r := range includeList {
					if !validResources[r] {
						return fmt.Errorf("invalid resource: %s. Valid resources: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations", r)
					}
				}

				// Handle conflict between skip_entities and include
				if skipEntities {
					for _, r := range includeList {
						if r == "entities" {
							cmd.Printf("Warning: --skip-entities conflicts with --include entities, ignoring --skip-entities\n")
							skipEntities = false
							break
						}
					}
				}
			}

			// Create export module
			exportModule := export.NewModule(orgConfig)
			defer exportModule.Close()

			cmd.Printf("\nExporting data from base organization: %s\n", orgName)
			if orgName == "" {
				cmd.Printf("(using default organization)\n")
			}
			cmd.Printf("Output file: %s\n", output)
			if len(blueprintList) > 0 {
				cmd.Printf("Blueprints filter: %s\n", strings.Join(blueprintList, ", "))
			}
			if len(includeList) > 0 {
				cmd.Printf("Including only: %s\n", strings.Join(includeList, ", "))
			} else if skipEntities {
				cmd.Printf("Skipping entities (schema only)\n")
			}

			// Execute export
			result, err := exportModule.Execute(cmd.Context(), export.Options{
				OutputPath:       output,
				Blueprints:       blueprintList,
				Format:           format,
				SkipEntities:     skipEntities,
				IncludeResources: includeList,
			})

			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}

			if !result.Success {
				return fmt.Errorf("export failed: %v", result.Error)
			}

			cmd.Printf("\n✓ Export completed successfully!\n")
			cmd.Printf("%s\n", result.Message)
			cmd.Printf("Blueprints: %d\n", result.BlueprintsCount)
			cmd.Printf("Entities: %d\n", result.EntitiesCount)
			cmd.Printf("Actions: %d\n", result.ActionsCount)
			cmd.Printf("Users: %d\n", result.UsersCount)
			cmd.Printf("Pages: %d\n", result.PagesCount)
			cmd.Printf("Integrations: %d\n", result.IntegrationsCount)

			return nil
		},
	}

	exportCmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (e.g., backup.tar.gz or backup.json)")
	exportCmd.MarkFlagRequired("output")
	exportCmd.Flags().StringVar(&org, "org", "", "Base organization name (uses default if not specified, deprecated: use --base-org)")
	exportCmd.Flags().StringVar(&baseOrg, "base-org", "", "Base organization name (uses default if not specified)")
	exportCmd.Flags().StringVarP(&blueprints, "blueprints", "b", "", "Comma-separated list of blueprint IDs to export (exports all if not specified)")
	exportCmd.Flags().StringVarP(&format, "format", "f", "", "Export format: tar (tar.gz) or json")
	exportCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip exporting entities (only export schema and configuration)")
	exportCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to export (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, exports all resources.")

	rootCmd.AddCommand(exportCmd)
}

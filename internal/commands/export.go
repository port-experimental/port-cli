package commands

import (
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
)

// RegisterExport registers the export command.
func RegisterExport(rootCmd *cobra.Command) {
	var (
		outputPath    string
		org           string
		baseOrg       string
		blueprints    string
		format        string
		skipEntities  bool
		include       string
		outputFormat  string
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
							output.WarningPrintln("Warning: --skip-entities conflicts with --include entities, ignoring --skip-entities")
							skipEntities = false
							break
						}
					}
				}
			}

			// Create export module
			exportModule := export.NewModule(orgConfig)
			defer exportModule.Close()

			// Show info only if not quiet and output format is text
			if outputFormat != "json" {
				output.Printf("\nExporting data from base organization: %s\n", orgName)
				if orgName == "" {
					output.Printf("(using default organization)\n")
				}
				output.Printf("Output file: %s\n", outputPath)
				if len(blueprintList) > 0 {
					output.Printf("Blueprints filter: %s\n", strings.Join(blueprintList, ", "))
				}
				if len(includeList) > 0 {
					output.Printf("Including only: %s\n", strings.Join(includeList, ", "))
				} else if skipEntities {
					output.Printf("Skipping entities (schema only)\n")
				}
			}

			// Execute export
			result, err := exportModule.Execute(cmd.Context(), export.Options{
				OutputPath:       outputPath,
				Blueprints:       blueprintList,
				Format:           format,
				SkipEntities:     skipEntities,
				IncludeResources: includeList,
			})

			if err != nil {
				if outputFormat == "json" {
					jsonResult := output.JSONResult{
						Success: false,
						Error:   err.Error(),
					}
					output.PrintJSON(jsonResult)
					return err
				}
				return fmt.Errorf("export failed: %w", err)
			}

			if !result.Success {
				if outputFormat == "json" {
					jsonResult := output.JSONResult{
						Success: false,
						Error:   fmt.Sprintf("%v", result.Error),
					}
					output.PrintJSON(jsonResult)
					return fmt.Errorf("export failed: %v", result.Error)
				}
				return fmt.Errorf("export failed: %v", result.Error)
			}

			// Output in JSON format if requested
			if outputFormat == "json" {
				jsonData := map[string]interface{}{
					"output_path":       result.OutputPath,
					"blueprints_count":  result.BlueprintsCount,
					"entities_count":    result.EntitiesCount,
					"actions_count":     result.ActionsCount,
					"users_count":       result.UsersCount,
					"teams_count":       result.TeamsCount,
					"pages_count":       result.PagesCount,
					"integrations_count": result.IntegrationsCount,
				}
				if len(result.TimeoutErrors) > 0 {
					jsonData["timeout_errors"] = result.TimeoutErrors
					jsonData["warnings"] = fmt.Sprintf("%d blueprint(s) timed out during export", len(result.TimeoutErrors))
				}
				jsonResult := output.JSONResult{
					Success: true,
					Message: result.Message,
					Data:    jsonData,
				}
				return output.PrintJSON(jsonResult)
			}

			// Text output
			output.SuccessPrintln("\n✓ Export completed successfully!")
			output.Printf("%s\n", result.Message)
			output.Printf("Blueprints: %d\n", result.BlueprintsCount)
			output.Printf("Entities: %d\n", result.EntitiesCount)
			output.Printf("Actions: %d\n", result.ActionsCount)
			output.Printf("Users: %d\n", result.UsersCount)
			output.Printf("Teams: %d\n", result.TeamsCount)
			output.Printf("Pages: %d\n", result.PagesCount)
			output.Printf("Integrations: %d\n", result.IntegrationsCount)

			// Display timeout warnings if any
			if len(result.TimeoutErrors) > 0 {
				output.WarningPrintln("\n⚠ Warning: Some blueprints timed out during export:")
				for _, timeoutErr := range result.TimeoutErrors {
					output.WarningPrintf("  - %s\n", timeoutErr)
				}
				output.WarningPrintln("These blueprints were skipped. Consider exporting them separately or contact Port support if this persists.")
			}

			return nil
		},
	}

	exportCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (e.g., backup.tar.gz or backup.json)")
	exportCmd.MarkFlagRequired("output")
	exportCmd.Flags().StringVar(&org, "org", "", "Base organization name (uses default if not specified, deprecated: use --base-org)")
	exportCmd.Flags().StringVar(&baseOrg, "base-org", "", "Base organization name (uses default if not specified)")
	exportCmd.Flags().StringVarP(&blueprints, "blueprints", "b", "", "Comma-separated list of blueprint IDs to export (exports all if not specified)")
	exportCmd.Flags().StringVarP(&format, "format", "f", "", "Export format: tar (tar.gz) or json")
	exportCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip exporting entities (only export schema and configuration)")
	exportCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to export (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, exports all resources.")
	exportCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(exportCmd)
}

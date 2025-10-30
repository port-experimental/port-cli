package commands

import (
	"fmt"
	"strings"

	"github.com/port-labs/port-cli/internal/config"
	"github.com/port-labs/port-cli/internal/modules/import_module"
	"github.com/spf13/cobra"
)

// RegisterImport registers the import command.
func RegisterImport(rootCmd *cobra.Command) {
	var (
		input       string
		org         string
		targetOrg   string
		dryRun      bool
		skipEntities bool
		include     string
	)

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import data to Port",
		Long: `Import data to Port organization.

Imports blueprints, entities, scorecards, actions, teams, automations, pages, and integrations from a file.
Use --skip-entities to only import configuration without entity data.
Use --include to selectively import specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			// Use target-org if provided, otherwise use org
			orgName := targetOrg
			if orgName == "" {
				orgName = org
			}

			// Use target org flags if provided, otherwise fall back to base flags
			targetClientID := flags.TargetClientID
			targetClientSecret := flags.TargetClientSecret
			targetAPIURL := flags.TargetAPIURL
			if targetClientID == "" {
				targetClientID = flags.ClientID
				targetClientSecret = flags.ClientSecret
				targetAPIURL = flags.APIURL
			}

			_, _, targetOrgConfig, err := configManager.LoadWithDualOverrides(
				"", "", "", "", // No base org for import
				targetClientID,
				targetClientSecret,
				targetAPIURL,
				orgName,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if targetOrgConfig == nil {
				return fmt.Errorf("target organization configuration not found")
			}

			orgConfig := targetOrgConfig

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

			// Create import module
			importModule := import_module.NewModule(orgConfig)
			defer importModule.Close()

			cmd.Printf("\nImporting data to target organization: %s\n", orgName)
			if orgName == "" {
				cmd.Printf("(using default organization)\n")
			}
			cmd.Printf("Input file: %s\n", input)
			if dryRun {
				cmd.Printf("Dry run mode - no changes will be applied\n")
			}
			cmd.Printf("Diff validation enabled - comparing with current organization state\n")
			if len(includeList) > 0 {
				cmd.Printf("Including only: %s\n", strings.Join(includeList, ", "))
			} else if skipEntities {
				cmd.Printf("Skipping entities (schema only)\n")
			}

			// Execute import
			result, err := importModule.Execute(cmd.Context(), import_module.Options{
				InputPath:       input,
				DryRun:          dryRun,
				SkipEntities:    skipEntities,
				IncludeResources: includeList,
			})

			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}

			if !result.Success {
				return fmt.Errorf("import failed")
			}

			cmd.Printf("\n✓ Import completed successfully!\n")
			cmd.Printf("%s\n", result.Message)
			
			// Show diff stats (always available now)
			if result.DiffResult != nil {
				cmd.Printf("\nDiff analysis:\n")
				if len(result.DiffResult.BlueprintsToCreate) > 0 || len(result.DiffResult.BlueprintsToUpdate) > 0 || len(result.DiffResult.BlueprintsToSkip) > 0 {
					cmd.Printf("  Blueprints: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.BlueprintsToCreate),
						len(result.DiffResult.BlueprintsToUpdate),
						len(result.DiffResult.BlueprintsToSkip))
				}
				if len(result.DiffResult.EntitiesToCreate) > 0 || len(result.DiffResult.EntitiesToUpdate) > 0 || len(result.DiffResult.EntitiesToSkip) > 0 {
					cmd.Printf("  Entities: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.EntitiesToCreate),
						len(result.DiffResult.EntitiesToUpdate),
						len(result.DiffResult.EntitiesToSkip))
				}
				if len(result.DiffResult.ScorecardsToCreate) > 0 || len(result.DiffResult.ScorecardsToUpdate) > 0 || len(result.DiffResult.ScorecardsToSkip) > 0 {
					cmd.Printf("  Scorecards: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.ScorecardsToCreate),
						len(result.DiffResult.ScorecardsToUpdate),
						len(result.DiffResult.ScorecardsToSkip))
				}
				if len(result.DiffResult.ActionsToCreate) > 0 || len(result.DiffResult.ActionsToUpdate) > 0 || len(result.DiffResult.ActionsToSkip) > 0 {
					cmd.Printf("  Actions: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.ActionsToCreate),
						len(result.DiffResult.ActionsToUpdate),
						len(result.DiffResult.ActionsToSkip))
				}
				if len(result.DiffResult.TeamsToCreate) > 0 || len(result.DiffResult.TeamsToUpdate) > 0 || len(result.DiffResult.TeamsToSkip) > 0 {
					cmd.Printf("  Teams: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.TeamsToCreate),
						len(result.DiffResult.TeamsToUpdate),
						len(result.DiffResult.TeamsToSkip))
				}
				if len(result.DiffResult.UsersToCreate) > 0 || len(result.DiffResult.UsersToUpdate) > 0 || len(result.DiffResult.UsersToSkip) > 0 {
					cmd.Printf("  Users: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.UsersToCreate),
						len(result.DiffResult.UsersToUpdate),
						len(result.DiffResult.UsersToSkip))
				}
				if len(result.DiffResult.PagesToCreate) > 0 || len(result.DiffResult.PagesToUpdate) > 0 || len(result.DiffResult.PagesToSkip) > 0 {
					cmd.Printf("  Pages: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.PagesToCreate),
						len(result.DiffResult.PagesToUpdate),
						len(result.DiffResult.PagesToSkip))
				}
				if len(result.DiffResult.IntegrationsToUpdate) > 0 || len(result.DiffResult.IntegrationsToSkip) > 0 {
					cmd.Printf("  Integrations: %d updated, %d skipped (identical)\n",
						len(result.DiffResult.IntegrationsToUpdate),
						len(result.DiffResult.IntegrationsToSkip))
				}
				cmd.Printf("\n")
			}
			
			cmd.Printf("Blueprints created: %d, updated: %d\n", result.BlueprintsCreated, result.BlueprintsUpdated)
			cmd.Printf("Entities created: %d, updated: %d\n", result.EntitiesCreated, result.EntitiesUpdated)
			cmd.Printf("Scorecards created: %d, updated: %d\n", result.ScorecardsCreated, result.ScorecardsUpdated)
			cmd.Printf("Actions created: %d, updated: %d\n", result.ActionsCreated, result.ActionsUpdated)
			cmd.Printf("Teams created: %d, updated: %d\n", result.TeamsCreated, result.TeamsUpdated)
			cmd.Printf("Users created: %d, updated: %d\n", result.UsersCreated, result.UsersUpdated)
			cmd.Printf("Pages created: %d, updated: %d\n", result.PagesCreated, result.PagesUpdated)
			cmd.Printf("Integrations updated: %d\n", result.IntegrationsUpdated)

			if len(result.Errors) > 0 {
				cmd.Printf("\nErrors encountered:\n")
				for _, errMsg := range result.Errors {
					cmd.Printf("  - %s\n", errMsg)
				}
			}

			return nil
		},
	}

	importCmd.Flags().StringVarP(&input, "input", "i", "", "Input file path (e.g., backup.tar.gz or backup.json)")
	importCmd.MarkFlagRequired("input")
	importCmd.Flags().StringVar(&org, "org", "", "Target organization name (uses default if not specified, deprecated: use --target-org)")
	importCmd.Flags().StringVar(&targetOrg, "target-org", "", "Target organization name (uses default if not specified)")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate import without applying changes")
	importCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip importing entities (only import schema and configuration)")
	importCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to import (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, imports all resources.")

	rootCmd.AddCommand(importCmd)
}

package commands

import (
	"fmt"
	"strings"

	"github.com/port-labs/port-cli/internal/config"
	"github.com/port-labs/port-cli/internal/modules/migrate"
	"github.com/port-labs/port-cli/internal/output"
	"github.com/spf13/cobra"
)

// RegisterMigrate registers the migrate command.
func RegisterMigrate(rootCmd *cobra.Command) {
	var (
		sourceOrg    string
		baseOrg      string
		targetOrg    string
		blueprints   string
		dryRun       bool
		skipEntities bool
		include      string
		outputFormat string
	)

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between Port organizations",
		Long: `Migrate data between Port organizations.

Migrates blueprints, entities, scorecards, actions, teams, users, pages, and integrations from source to target organization.
Use --skip-entities to only migrate configuration without entity data.
Use --include to selectively migrate specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			// Use base-org if provided, otherwise use source-org
			sourceOrgName := baseOrg
			if sourceOrgName == "" {
				sourceOrgName = sourceOrg
			}

			// Validate that source org is provided
			if sourceOrgName == "" {
				return fmt.Errorf("source organization is required. Use --source-org or --base-org")
			}

			// Validate that target org is provided
			if targetOrg == "" {
				return fmt.Errorf("target organization is required. Use --target-org")
			}

			// Use CLI flags if provided, otherwise use org names from config
			baseClientID := flags.ClientID
			baseClientSecret := flags.ClientSecret
			baseAPIURL := flags.APIURL
			targetClientID := flags.TargetClientID
			targetClientSecret := flags.TargetClientSecret
			targetAPIURL := flags.TargetAPIURL

			_, baseOrgConfig, targetOrgConfig, err := configManager.LoadWithDualOverrides(
				baseClientID,
				baseClientSecret,
				baseAPIURL,
				sourceOrgName,
				targetClientID,
				targetClientSecret,
				targetAPIURL,
				targetOrg,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if baseOrgConfig == nil {
				return fmt.Errorf("base organization configuration not found")
			}

			if targetOrgConfig == nil {
				return fmt.Errorf("target organization configuration not found")
			}

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

			// Create migration module
			migrateModule := migrate.NewModule(baseOrgConfig, targetOrgConfig)
			defer migrateModule.Close()

			// Show info only if not quiet and output format is text
			if outputFormat != "json" {
				output.Printf("\nMigration:\n")
				output.Printf("  Source (base org): %s\n", sourceOrgName)
				output.Printf("  Target org: %s\n", targetOrg)
				if len(blueprintList) > 0 {
					output.Printf("  Blueprints: %s\n", strings.Join(blueprintList, ", "))
				}
				output.Printf("Diff validation enabled - comparing source with target organization state\n")
				if len(includeList) > 0 {
					output.Printf("  Including only: %s\n", strings.Join(includeList, ", "))
				} else if skipEntities {
					output.Printf("  Skipping entities (schema only)\n")
				}
				if dryRun {
					output.Printf("  Dry run mode - no changes will be applied\n")
				}
			}

			// Execute migration
			result, err := migrateModule.Execute(cmd.Context(), migrate.Options{
				Blueprints:       blueprintList,
				DryRun:           dryRun,
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
				return fmt.Errorf("migration failed: %w", err)
			}

			if !result.Success {
				if outputFormat == "json" {
					jsonResult := output.JSONResult{
						Success: false,
						Error:   "migration failed",
					}
					output.PrintJSON(jsonResult)
					return fmt.Errorf("migration failed")
				}
				return fmt.Errorf("migration failed")
			}

			// Output in JSON format if requested
			if outputFormat == "json" {
				jsonData := map[string]interface{}{
					"success":              true,
					"message":              result.Message,
					"blueprints_created":   result.BlueprintsCreated,
					"blueprints_updated":   result.BlueprintsUpdated,
					"blueprints_skipped":   result.BlueprintsSkipped,
					"entities_created":     result.EntitiesCreated,
					"entities_updated":     result.EntitiesUpdated,
					"entities_skipped":     result.EntitiesSkipped,
					"scorecards_created":   result.ScorecardsCreated,
					"scorecards_updated":   result.ScorecardsUpdated,
					"scorecards_skipped":   result.ScorecardsSkipped,
					"actions_created":      result.ActionsCreated,
					"actions_updated":      result.ActionsUpdated,
					"actions_skipped":      result.ActionsSkipped,
					"teams_created":        result.TeamsCreated,
					"teams_updated":        result.TeamsUpdated,
					"teams_skipped":        result.TeamsSkipped,
					"users_created":        result.UsersCreated,
					"users_updated":        result.UsersUpdated,
					"users_skipped":        result.UsersSkipped,
					"pages_created":        result.PagesCreated,
					"pages_updated":        result.PagesUpdated,
					"pages_skipped":        result.PagesSkipped,
					"integrations_updated": result.IntegrationsUpdated,
					"integrations_skipped": result.IntegrationsSkipped,
				}
				if len(result.Errors) > 0 {
					jsonData["errors"] = result.Errors
				}
				return output.PrintJSON(jsonData)
			}

			// Text output
			output.SuccessPrintln("\n✓ Migration completed successfully!")
			output.Printf("%s\n", result.Message)

			// Show diff stats (always available now)
			if result.DiffResult != nil {
				output.Printf("\nDiff analysis:\n")
				if len(result.DiffResult.BlueprintsToCreate) > 0 || len(result.DiffResult.BlueprintsToUpdate) > 0 || len(result.DiffResult.BlueprintsToSkip) > 0 {
					output.Printf("  Blueprints: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.BlueprintsToCreate),
						len(result.DiffResult.BlueprintsToUpdate),
						len(result.DiffResult.BlueprintsToSkip))
				}
				if len(result.DiffResult.EntitiesToCreate) > 0 || len(result.DiffResult.EntitiesToUpdate) > 0 || len(result.DiffResult.EntitiesToSkip) > 0 {
					output.Printf("  Entities: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.EntitiesToCreate),
						len(result.DiffResult.EntitiesToUpdate),
						len(result.DiffResult.EntitiesToSkip))
				}
				if len(result.DiffResult.ScorecardsToCreate) > 0 || len(result.DiffResult.ScorecardsToUpdate) > 0 || len(result.DiffResult.ScorecardsToSkip) > 0 {
					output.Printf("  Scorecards: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.ScorecardsToCreate),
						len(result.DiffResult.ScorecardsToUpdate),
						len(result.DiffResult.ScorecardsToSkip))
				}
				if len(result.DiffResult.ActionsToCreate) > 0 || len(result.DiffResult.ActionsToUpdate) > 0 || len(result.DiffResult.ActionsToSkip) > 0 {
					output.Printf("  Actions: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.ActionsToCreate),
						len(result.DiffResult.ActionsToUpdate),
						len(result.DiffResult.ActionsToSkip))
				}
				if len(result.DiffResult.TeamsToCreate) > 0 || len(result.DiffResult.TeamsToUpdate) > 0 || len(result.DiffResult.TeamsToSkip) > 0 {
					output.Printf("  Teams: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.TeamsToCreate),
						len(result.DiffResult.TeamsToUpdate),
						len(result.DiffResult.TeamsToSkip))
				}
				if len(result.DiffResult.UsersToCreate) > 0 || len(result.DiffResult.UsersToUpdate) > 0 || len(result.DiffResult.UsersToSkip) > 0 {
					output.Printf("  Users: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.UsersToCreate),
						len(result.DiffResult.UsersToUpdate),
						len(result.DiffResult.UsersToSkip))
				}
				if len(result.DiffResult.PagesToCreate) > 0 || len(result.DiffResult.PagesToUpdate) > 0 || len(result.DiffResult.PagesToSkip) > 0 {
					output.Printf("  Pages: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.PagesToCreate),
						len(result.DiffResult.PagesToUpdate),
						len(result.DiffResult.PagesToSkip))
				}
				if len(result.DiffResult.IntegrationsToUpdate) > 0 || len(result.DiffResult.IntegrationsToSkip) > 0 {
					output.Printf("  Integrations: %d updated, %d skipped (identical)\n",
						len(result.DiffResult.IntegrationsToUpdate),
						len(result.DiffResult.IntegrationsToSkip))
				}
				output.Printf("\n")
			}
			
			output.Printf("Blueprints created: %d, updated: %d, skipped: %d\n", result.BlueprintsCreated, result.BlueprintsUpdated, result.BlueprintsSkipped)
			output.Printf("Entities created: %d, updated: %d, skipped: %d\n", result.EntitiesCreated, result.EntitiesUpdated, result.EntitiesSkipped)
			output.Printf("Scorecards created: %d, updated: %d, skipped: %d\n", result.ScorecardsCreated, result.ScorecardsUpdated, result.ScorecardsSkipped)
			output.Printf("Actions created: %d, updated: %d, skipped: %d\n", result.ActionsCreated, result.ActionsUpdated, result.ActionsSkipped)
			output.Printf("Teams created: %d, updated: %d, skipped: %d\n", result.TeamsCreated, result.TeamsUpdated, result.TeamsSkipped)
			output.Printf("Users created: %d, updated: %d, skipped: %d\n", result.UsersCreated, result.UsersUpdated, result.UsersSkipped)
			output.Printf("Pages created: %d, updated: %d, skipped: %d\n", result.PagesCreated, result.PagesUpdated, result.PagesSkipped)
			output.Printf("Integrations updated: %d, skipped: %d\n", result.IntegrationsUpdated, result.IntegrationsSkipped)

			if len(result.Errors) > 0 {
				output.Printf("\nWarnings:\n")
				maxErrors := 5
				if len(result.Errors) < maxErrors {
					maxErrors = len(result.Errors)
				}
				for i := 0; i < maxErrors; i++ {
					output.Printf("  - %s\n", result.Errors[i])
				}
				if len(result.Errors) > maxErrors {
					output.Printf("  ... and %d more\n", len(result.Errors)-maxErrors)
				}
			}

			return nil
		},
	}

	migrateCmd.Flags().StringVarP(&sourceOrg, "source-org", "s", "", "Source organization name (base org)")
	migrateCmd.Flags().StringVar(&baseOrg, "base-org", "", "Base organization name (alias for --source-org)")
	migrateCmd.Flags().StringVarP(&targetOrg, "target-org", "t", "", "Target organization name")
	migrateCmd.MarkFlagRequired("target-org")
	migrateCmd.Flags().StringVarP(&blueprints, "blueprints", "b", "", "Comma-separated list of blueprint IDs to migrate (migrates all if not specified)")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate migration without applying changes")
	migrateCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip migrating entities (only migrate schema and configuration)")
	migrateCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to migrate (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, migrates all resources.")
	migrateCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(migrateCmd)
}

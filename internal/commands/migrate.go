package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
)

// RegisterMigrate registers the migrate command.
func RegisterMigrate(rootCmd *cobra.Command) {
	var (
		sourceOrg              string
		baseOrg                string
		targetOrg              string
		blueprints             string
		mode                   string
		dryRun                 bool
		yes                    bool
		skipEntities           bool
		skipSystemBlueprints   bool
		includeRuleResults     bool
		include                string
		outputFormat           string
		excludeBlueprints      string
		excludeBlueprintSchema string
	)

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between Port organizations",
		Long: `Migrate data between Port organizations.

Migrates blueprints, entities, scorecards, actions, teams, users, pages, and integrations from source to target organization.
Use --skip-entities to only migrate configuration without entity data.
Use --include to selectively migrate specific resource types.
Use --mode to control update behavior:
  update   (default) - additive: create new, merge-update existing, never delete
  converge           - full sync: create, replace, and delete target-only resources`,
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
					"blueprints":            true,
					"entities":              true,
					"scorecards":            true,
					"actions":               true,
					"teams":                 true,
					"users":                 true,
					"automations":           true,
					"pages":                 true,
					"integrations":          true,
					"blueprint-permissions": true,
					"action-permissions":    true,
					"page-permissions":      true,
				}

				for _, r := range includeList {
					if !validResources[r] {
						return fmt.Errorf("invalid resource: %s. Valid resources: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations, blueprint-permissions, action-permissions, page-permissions", r)
					}
				}

				if slices.Contains(includeList, "page-permissions") && !slices.Contains(includeList, "pages") {
					return fmt.Errorf("page-permissions requires pages to also be included (add 'pages' to --include)")
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
				if skipEntities {
					for _, r := range includeList {
						if r == "users" {
							output.WarningPrintln("Warning: --skip-entities conflicts with --include users, ignoring --skip-entities")
							skipEntities = false
							break
						}
						if r == "teams" {
							output.WarningPrintln("Warning: --skip-entities conflicts with --include teams, ignoring --skip-entities")
							skipEntities = false
							break
						}
					}
				}
			}

			// Parse exclude-blueprints flag
			var excludeBlueprintList []string
			if excludeBlueprints != "" {
				for _, id := range strings.Split(excludeBlueprints, ",") {
					if trimmed := strings.TrimSpace(id); trimmed != "" {
						excludeBlueprintList = append(excludeBlueprintList, trimmed)
					}
				}
			}

			// Parse exclude-blueprint-schema flag
			var excludeBlueprintSchemaList []string
			if excludeBlueprintSchema != "" {
				for _, id := range strings.Split(excludeBlueprintSchema, ",") {
					if trimmed := strings.TrimSpace(id); trimmed != "" {
						excludeBlueprintSchemaList = append(excludeBlueprintSchemaList, trimmed)
					}
				}
			}

			// Validate mode
			if mode == "" {
				mode = import_module.ModeUpdate
			}
			if mode != import_module.ModeUpdate && mode != import_module.ModeConverge {
				return fmt.Errorf("invalid mode: %s. Valid modes: update, converge", mode)
			}

			// Create migration module
			sourceToken, err := configManager.GetOrRefreshToken(cmd.Context(), sourceOrgName)
			if err != nil {
				if !config.ShouldIgnoreGetOrRefreshTokenError(err) {
					return err
				}
			}
			targetToken, err := configManager.GetOrRefreshToken(cmd.Context(), targetOrg)
			if err != nil {
				if !config.ShouldIgnoreGetOrRefreshTokenError(err) {
					return err
				}
			}
			migrateModule := migrate.NewModule(sourceToken, targetToken, baseOrgConfig, targetOrgConfig)
			defer migrateModule.Close()

			// Show info only if not quiet and output format is text
			if outputFormat != "json" {
				output.Printf("\nMigration:\n")
				output.Printf("  Source (base org): %s\n", sourceOrgName)
				output.Printf("  Target org: %s\n", targetOrg)
				output.Printf("  Mode: %s\n", mode)
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
			// Set up converge confirmation callback
			var confirmCb import_module.ConfirmFunc
			if !yes && mode == import_module.ModeConverge {
				confirmCb = func(summary string) (bool, error) {
					return confirmPrompt("Converge Mode Confirmation", summary)
				}
			}

			result, err := migrateModule.Execute(cmd.Context(), migrate.Options{
				Blueprints:             blueprintList,
				Mode:                   mode,
				DryRun:                 dryRun,
				Yes:                    yes,
				SkipEntities:           skipEntities,
				SkipSystemBlueprints:   skipSystemBlueprints,
				IncludeRuleResults:     includeRuleResults,
				IncludeResources:       includeList,
				ExcludeBlueprints:      excludeBlueprintList,
				ExcludeBlueprintSchema: excludeBlueprintSchemaList,
				ConfirmCallback:        confirmCb,
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
					"success":                       true,
					"message":                       result.Message,
					"blueprints_created":            result.BlueprintsCreated,
					"blueprints_updated":            result.BlueprintsUpdated,
					"blueprints_skipped":            result.BlueprintsSkipped,
					"entities_created":              result.EntitiesCreated,
					"entities_updated":              result.EntitiesUpdated,
					"entities_skipped":              result.EntitiesSkipped,
					"scorecards_created":            result.ScorecardsCreated,
					"scorecards_updated":            result.ScorecardsUpdated,
					"scorecards_skipped":            result.ScorecardsSkipped,
					"actions_created":               result.ActionsCreated,
					"actions_updated":               result.ActionsUpdated,
					"actions_skipped":               result.ActionsSkipped,
					"teams_created":                 result.TeamsCreated,
					"teams_updated":                 result.TeamsUpdated,
					"teams_skipped":                 result.TeamsSkipped,
					"users_created":                 result.UsersCreated,
					"users_updated":                 result.UsersUpdated,
					"users_skipped":                 result.UsersSkipped,
					"pages_created":                 result.PagesCreated,
					"pages_updated":                 result.PagesUpdated,
					"pages_skipped":                 result.PagesSkipped,
					"integrations_updated":          result.IntegrationsUpdated,
					"integrations_skipped":          result.IntegrationsSkipped,
					"blueprint_permissions_updated": result.BlueprintPermissionsUpdated,
					"action_permissions_updated":    result.ActionPermissionsUpdated,
					"page_permissions_updated":      result.PagePermissionsUpdated,
				}
				if len(result.Errors) > 0 {
					jsonData["errors"] = result.Errors
				}
				if len(result.Warnings) > 0 {
					jsonData["warnings"] = result.Warnings
				}
				if result.IgnoredRuleResultTargetRelationCount > 0 {
					jsonData["ignored_rule_result_target_relations_count"] = result.IgnoredRuleResultTargetRelationCount
					jsonData["ignored_rule_result_target_relation_keys"] = result.IgnoredRuleResultTargetRelationKeys
				}
				return output.PrintJSON(jsonData)
			}

			// Text output
			output.SuccessPrintln("\n✓ Migration completed successfully!")
			output.Printf("%s\n", result.Message)
			if result.IgnoredRuleResultTargetRelationCount > 0 {
				output.Printf("\n_rule_result: ignored %d relation(s) with type rule_result_target (not sent to API): %s\n",
					result.IgnoredRuleResultTargetRelationCount,
					strings.Join(result.IgnoredRuleResultTargetRelationKeys, ", "))
			}

			// Show diff stats (always available now)
			if result.DiffResult != nil {
				output.Printf("\nDiff analysis:\n")
				if len(result.DiffResult.BlueprintsToCreate) > 0 || len(result.DiffResult.BlueprintsToUpdate) > 0 || len(result.DiffResult.BlueprintsToSkip) > 0 || len(result.DiffResult.BlueprintsToDelete) > 0 {
					msg := fmt.Sprintf("  Blueprints: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.BlueprintsToCreate),
						len(result.DiffResult.BlueprintsToUpdate),
						len(result.DiffResult.BlueprintsToSkip))
					if len(result.DiffResult.BlueprintsToDelete) > 0 {
						msg += fmt.Sprintf(", %d to delete", len(result.DiffResult.BlueprintsToDelete))
					}
					output.Printf("%s\n", msg)
				}
				entDel := result.DiffResult.TotalEntitiesToDelete()
				if len(result.DiffResult.EntitiesToCreate) > 0 || len(result.DiffResult.EntitiesToUpdate) > 0 || len(result.DiffResult.EntitiesToSkip) > 0 || entDel > 0 {
					msg := fmt.Sprintf("  Entities: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.EntitiesToCreate),
						len(result.DiffResult.EntitiesToUpdate),
						len(result.DiffResult.EntitiesToSkip))
					if entDel > 0 {
						msg += fmt.Sprintf(", %d to delete", entDel)
					}
					output.Printf("%s\n", msg)
				}
				scDel := result.DiffResult.TotalScorecardsToDelete()
				if len(result.DiffResult.ScorecardsToCreate) > 0 || len(result.DiffResult.ScorecardsToUpdate) > 0 || len(result.DiffResult.ScorecardsToSkip) > 0 || scDel > 0 {
					msg := fmt.Sprintf("  Scorecards: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.ScorecardsToCreate),
						len(result.DiffResult.ScorecardsToUpdate),
						len(result.DiffResult.ScorecardsToSkip))
					if scDel > 0 {
						msg += fmt.Sprintf(", %d to delete", scDel)
					}
					output.Printf("%s\n", msg)
				}
				if len(result.DiffResult.ActionsToCreate) > 0 || len(result.DiffResult.ActionsToUpdate) > 0 || len(result.DiffResult.ActionsToSkip) > 0 || len(result.DiffResult.ActionsToDelete) > 0 {
					msg := fmt.Sprintf("  Actions: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.ActionsToCreate),
						len(result.DiffResult.ActionsToUpdate),
						len(result.DiffResult.ActionsToSkip))
					if len(result.DiffResult.ActionsToDelete) > 0 {
						msg += fmt.Sprintf(", %d to delete", len(result.DiffResult.ActionsToDelete))
					}
					output.Printf("%s\n", msg)
				}
				if len(result.DiffResult.TeamsToCreate) > 0 || len(result.DiffResult.TeamsToUpdate) > 0 || len(result.DiffResult.TeamsToSkip) > 0 || len(result.DiffResult.TeamsToDelete) > 0 {
					msg := fmt.Sprintf("  Teams: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.TeamsToCreate),
						len(result.DiffResult.TeamsToUpdate),
						len(result.DiffResult.TeamsToSkip))
					if len(result.DiffResult.TeamsToDelete) > 0 {
						msg += fmt.Sprintf(", %d to delete", len(result.DiffResult.TeamsToDelete))
					}
					output.Printf("%s\n", msg)
				}
				if len(result.DiffResult.UsersToCreate) > 0 || len(result.DiffResult.UsersToUpdate) > 0 || len(result.DiffResult.UsersToSkip) > 0 {
					output.Printf("  Users: %d new, %d updated, %d skipped (identical)\n",
						len(result.DiffResult.UsersToCreate),
						len(result.DiffResult.UsersToUpdate),
						len(result.DiffResult.UsersToSkip))
				}
				if len(result.DiffResult.PagesToCreate) > 0 || len(result.DiffResult.PagesToUpdate) > 0 || len(result.DiffResult.PagesToSkip) > 0 || len(result.DiffResult.PagesToDelete) > 0 {
					msg := fmt.Sprintf("  Pages: %d new, %d updated, %d skipped (identical)",
						len(result.DiffResult.PagesToCreate),
						len(result.DiffResult.PagesToUpdate),
						len(result.DiffResult.PagesToSkip))
					if len(result.DiffResult.PagesToDelete) > 0 {
						msg += fmt.Sprintf(", %d to delete", len(result.DiffResult.PagesToDelete))
					}
					output.Printf("%s\n", msg)
				}
				if len(result.DiffResult.IntegrationsToUpdate) > 0 || len(result.DiffResult.IntegrationsToSkip) > 0 || len(result.DiffResult.IntegrationsToDelete) > 0 {
					msg := fmt.Sprintf("  Integrations: %d updated, %d skipped (identical)",
						len(result.DiffResult.IntegrationsToUpdate),
						len(result.DiffResult.IntegrationsToSkip))
					if len(result.DiffResult.IntegrationsToDelete) > 0 {
						msg += fmt.Sprintf(", %d to delete", len(result.DiffResult.IntegrationsToDelete))
					}
					output.Printf("%s\n", msg)
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
			if result.PagePermissionsUpdated > 0 {
				output.Printf("Page permissions updated: %d\n", result.PagePermissionsUpdated)
			}

			if len(result.Warnings) > 0 {
				output.Printf("\nWarnings:\n")
				for _, w := range result.Warnings {
					output.WarningPrintln(fmt.Sprintf("  ⚠ %s", w))
				}
			}

			if len(result.Errors) > 0 {
				output.Printf("\nErrors:\n")
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
	migrateCmd.Flags().StringVar(&mode, "mode", "update", "Update mode: 'update' (additive, safe) or 'converge' (full sync, may delete)")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate migration without applying changes")
	migrateCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts (required for converge mode in non-interactive environments)")
	migrateCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip migrating entities (only migrate schema and configuration)")
	migrateCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	migrateCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	migrateCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to migrate (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, migrates all resources.")
	migrateCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	migrateCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still migrated)")
	migrateCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(migrateCmd)
}

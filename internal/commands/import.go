package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
)

// RegisterImport registers the import command.
func RegisterImport(rootCmd *cobra.Command) {
	var (
		input                  string
		org                    string
		targetOrg              string
		mode                   string
		dryRun                 bool
		yes                    bool
		skipEntities           bool
		skipSystemBlueprints   bool
		includeRuleResults     bool
		include                string
		outputFormat           string
		verbose                bool
		showPagesPipeline      bool
		excludeBlueprints      string
		excludeBlueprintSchema string
	)

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import data to Port",
		Long: `Import data to Port organization.

Imports blueprints, entities, scorecards, actions, teams, automations, pages, and integrations from a file.
Use --skip-entities to only import configuration without entity data.
Use --include to selectively import specific resource types.
Use --mode to control update behavior:
  update   (default) - additive: create new, merge-update existing, never delete
  converge           - full sync: create, replace, and delete target-only resources`,
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

			// Parse exclude-blueprints (deep)
			var excludeBlueprintList []string
			if excludeBlueprints != "" {
				excludeBlueprintList = strings.Split(excludeBlueprints, ",")
				for i := range excludeBlueprintList {
					excludeBlueprintList[i] = strings.TrimSpace(excludeBlueprintList[i])
				}
			}

			// Parse exclude-blueprint-schema (schema-only)
			var excludeBlueprintSchemaList []string
			if excludeBlueprintSchema != "" {
				excludeBlueprintSchemaList = strings.Split(excludeBlueprintSchema, ",")
				for i := range excludeBlueprintSchemaList {
					excludeBlueprintSchemaList[i] = strings.TrimSpace(excludeBlueprintSchemaList[i])
				}
			}

			// Validate mode
			if mode == "" {
				mode = import_module.ModeUpdate
			}
			if mode != import_module.ModeUpdate && mode != import_module.ModeConverge {
				return fmt.Errorf("invalid mode: %s. Valid modes: update, converge", mode)
			}

			token, err := configManager.GetOrRefreshToken(cmd.Context(), orgName)
			if err != nil {
				if !config.ShouldIgnoreGetOrRefreshTokenError(err) {
					return err
				}
			}
			// Create import module
			importModule := import_module.NewModule(token, orgConfig)
			defer importModule.Close()

			// Show info only if not quiet and output format is text
			if outputFormat != "json" {
				output.Printf("\nImporting data to target organization: %s\n", orgName)
				if orgName == "" {
					output.Printf("(using default organization)\n")
				}
				output.Printf("Input file: %s\n", input)
				output.Printf("Mode: %s\n", mode)
				if dryRun {
					output.Printf("Dry run mode - no changes will be applied\n")
				}
				output.Printf("Diff validation enabled - comparing with current organization state\n")
				if len(includeList) > 0 {
					output.Printf("Including only: %s\n", strings.Join(includeList, ", "))
				} else if skipEntities {
					output.Printf("Skipping entities (schema only)\n")
				}
			}

			// Progress callback for real-time updates
			var progressCallback import_module.ProgressCallback
			var logCallback func(string)
			if outputFormat != "json" {
				lastPhase := ""
				progressCallback = func(phase string, current, total int) {
					if phase != lastPhase {
						if lastPhase != "" {
							output.Printf("\n")
						}
						lastPhase = phase
					}
					output.Printf("\r  %s: %d/%d", phase, current, total)
				}
				if showPagesPipeline || verbose {
					logCallback = func(message string) {
						output.Printf("%s\n", message)
					}
				}
			}

			// Set up converge confirmation callback
			var confirmCb import_module.ConfirmFunc
			if !yes && mode == import_module.ModeConverge {
				confirmCb = func(summary string) (bool, error) {
					return confirmPrompt("Converge Mode Confirmation", summary)
				}
			}

			// Execute import
			result, err := importModule.Execute(cmd.Context(), import_module.Options{
				InputPath:              input,
				Mode:                   mode,
				DryRun:                 dryRun,
				Yes:                    yes,
				SkipEntities:           skipEntities,
				SkipSystemBlueprints:   skipSystemBlueprints,
				IncludeRuleResults:     includeRuleResults,
				IncludeResources:       includeList,
				ExcludeBlueprints:      excludeBlueprintList,
				ExcludeBlueprintSchema: excludeBlueprintSchemaList,
				Verbose:                verbose,
				ShowPagesPipeline:      showPagesPipeline,
				ProgressCallback:       progressCallback,
				LogCallback:            logCallback,
				ConfirmCallback:        confirmCb,
			})

			// Clear progress line
			if outputFormat != "json" && progressCallback != nil {
				output.Printf("\n")
			}

			if err != nil {
				if outputFormat == "json" {
					jsonResult := output.JSONResult{
						Success: false,
						Error:   err.Error(),
					}
					output.PrintJSON(jsonResult)
					return err
				}
				return fmt.Errorf("import failed: %w", err)
			}

			// Output in JSON format if requested
			if outputFormat == "json" {
				jsonData := map[string]interface{}{
					"success":                       result.Success,
					"message":                       result.Message,
					"blueprints_created":            result.BlueprintsCreated,
					"blueprints_updated":            result.BlueprintsUpdated,
					"entities_created":              result.EntitiesCreated,
					"entities_updated":              result.EntitiesUpdated,
					"scorecards_created":            result.ScorecardsCreated,
					"scorecards_updated":            result.ScorecardsUpdated,
					"actions_created":               result.ActionsCreated,
					"actions_updated":               result.ActionsUpdated,
					"teams_created":                 result.TeamsCreated,
					"teams_updated":                 result.TeamsUpdated,
					"users_created":                 result.UsersCreated,
					"users_updated":                 result.UsersUpdated,
					"pages_created":                 result.PagesCreated,
					"pages_updated":                 result.PagesUpdated,
					"integrations_updated":          result.IntegrationsUpdated,
					"blueprint_permissions_updated": result.BlueprintPermissionsUpdated,
					"action_permissions_updated":    result.ActionPermissionsUpdated,
					"page_permissions_updated":      result.PagePermissionsUpdated,
				}
				if len(result.Errors) > 0 {
					jsonData["errors"] = result.Errors
				}
				if result.IgnoredRuleResultTargetRelationCount > 0 {
					jsonData["ignored_rule_result_target_relations_count"] = result.IgnoredRuleResultTargetRelationCount
					jsonData["ignored_rule_result_target_relation_keys"] = result.IgnoredRuleResultTargetRelationKeys
				}
				if showPagesPipeline && len(result.SidebarPipeline) > 0 {
					jsonData["sidebar_pipeline"] = result.SidebarPipeline
				}
				output.PrintJSON(jsonData)
				if !result.Success {
					return fmt.Errorf("import completed with errors")
				}
				return nil
			}

			// Text output
			if result.Success {
				output.SuccessPrintln("\n✓ Import completed successfully!")
			} else {
				output.WarningPrintln("\n⚠ Import completed with errors")
			}
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
				if len(result.DiffResult.BlueprintPermissions) > 0 {
					output.Printf("  Blueprint permissions: %d to update\n",
						len(result.DiffResult.BlueprintPermissions))
				}
				if len(result.DiffResult.ActionPermissions) > 0 {
					output.Printf("  Action permissions: %d to update\n",
						len(result.DiffResult.ActionPermissions))
				}
				if len(result.DiffResult.PagePermissions) > 0 {
					output.Printf("  Page permissions: %d to update\n",
						len(result.DiffResult.PagePermissions))
				}
				output.Printf("\n")
			}

			output.Printf("Blueprints created: %d, updated: %d\n", result.BlueprintsCreated, result.BlueprintsUpdated)
			output.Printf("Entities created: %d, updated: %d\n", result.EntitiesCreated, result.EntitiesUpdated)
			output.Printf("Scorecards created: %d, updated: %d\n", result.ScorecardsCreated, result.ScorecardsUpdated)
			output.Printf("Actions created: %d, updated: %d\n", result.ActionsCreated, result.ActionsUpdated)
			output.Printf("Teams created: %d, updated: %d\n", result.TeamsCreated, result.TeamsUpdated)
			output.Printf("Users created: %d, updated: %d\n", result.UsersCreated, result.UsersUpdated)
			output.Printf("Pages created: %d, updated: %d\n", result.PagesCreated, result.PagesUpdated)
			output.Printf("Integrations updated: %d\n", result.IntegrationsUpdated)
			if result.BlueprintPermissionsUpdated > 0 || result.ActionPermissionsUpdated > 0 || result.PagePermissionsUpdated > 0 {
				output.Printf("Blueprint permissions updated: %d\n", result.BlueprintPermissionsUpdated)
				output.Printf("Action permissions updated: %d\n", result.ActionPermissionsUpdated)
				output.Printf("Page permissions updated: %d\n", result.PagePermissionsUpdated)
			}

			if showPagesPipeline && len(result.SidebarPipeline) > 0 {
				output.Printf("\nSidebar pipeline used:\n")
				for _, step := range result.SidebarPipeline {
					output.Printf("  %s\n", step)
				}
			}

			// Show warnings (cycle detection, etc.)
			if len(result.Warnings) > 0 {
				output.Printf("\nWarnings:\n")
				for _, warning := range result.Warnings {
					output.WarningPrintln(fmt.Sprintf("  ⚠ %s", warning.Message))
					if verbose && len(warning.Details) > 0 {
						for _, detail := range warning.Details {
							output.Printf("      - %s\n", detail)
						}
					}
				}
			}

			// Show errors
			if len(result.Errors) > 0 {
				if verbose && len(result.ErrorsByCategory) > 0 {
					// Verbose output: show errors grouped by category
					output.Printf("\nErrors by category:\n")
					categoryOrder := []string{"DEPENDENCY", "VALIDATION", "SCHEMA_MISMATCH", "BLUEPRINT_CONFIG", "AUTH", "NOT_FOUND", "CONFLICT", "RATE_LIMIT", "NETWORK", "UNKNOWN"}
					for _, category := range categoryOrder {
						if errs, ok := result.ErrorsByCategory[category]; ok && len(errs) > 0 {
							output.Printf("\n  %s (%d):\n", category, len(errs))
							for _, errMsg := range errs {
								output.Printf("    - %s\n", errMsg)
							}
						}
					}
				} else {
					// Standard output: simple error list
					output.Printf("\nErrors encountered:\n")
					for _, errMsg := range result.Errors {
						output.Printf("  - %s\n", errMsg)
					}
				}
			}

			if !result.Success {
				return fmt.Errorf("import completed with errors")
			}
			return nil
		},
	}

	importCmd.Flags().StringVarP(&input, "input", "i", "", "Input file path (e.g., backup.tar.gz or backup.json)")
	importCmd.MarkFlagRequired("input")
	importCmd.Flags().StringVar(&org, "org", "", "Target organization name (uses default if not specified, deprecated: use --target-org)")
	importCmd.Flags().StringVar(&targetOrg, "target-org", "", "Target organization name (uses default if not specified)")
	importCmd.Flags().StringVar(&mode, "mode", "update", "Update mode: 'update' (additive, safe) or 'converge' (full sync, may delete)")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate import without applying changes")
	importCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts (required for converge mode in non-interactive environments)")
	importCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip importing entities (only import schema and configuration)")
	importCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	importCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	importCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to import (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, imports all resources.")
	importCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	importCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still imported)")
	importCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	importCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed error information with categorization")
	importCmd.Flags().BoolVar(&showPagesPipeline, "show-pages-pipeline", false, "Show the planned sidebar pages/folders pipeline before execution and include the pipeline used in the output")

	rootCmd.AddCommand(importCmd)
}

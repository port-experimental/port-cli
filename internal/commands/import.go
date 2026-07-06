package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/port-experimental/port-cli/internal/render"
	"github.com/spf13/cobra"
)

// RegisterImport registers the import command.
func RegisterImport(rootCmd *cobra.Command) {
	var (
		input                         string
		org                           string
		targetOrg                     string
		dryRun                        bool
		skipEntities                  bool
		skipSystemBlueprints          bool
		skipSystemBlueprintProperties bool
		includeRuleResults            bool
		include                       string
		outputFormat                  string
		verbose                       bool
		showPagesPipeline             bool
		excludeBlueprints             string
		excludeBlueprintSchema        string
		usersAsDisabled               bool
		maxErrors                     int
	)

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import data to Port",
		Long: `Import data to Port organization.

Imports blueprints, entities, scorecards, actions, teams, automations, pages, and integrations from a file.
Use --skip-entities to only import configuration without entity data.
Use --include to selectively import specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateStringEnum("--output-format", outputFormat, []string{"text", "json"}); err != nil {
				return err
			}

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
			if err := validateMaxErrorsFlag(maxErrors); err != nil {
				return err
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

			token, err := configManager.GetOrRefreshToken(cmd.Context(), orgName)
			if err != nil {
				if !config.ShouldIgnoreGetOrRefreshTokenError(err) {
					return err
				}
			}
			// Create import module
			importModule := import_module.NewModule(token, orgConfig)
			defer importModule.Close()

			importRenderer := render.ImportRenderer{}
			if outputFormat != "json" {
				importRenderer.PrintPreflight(render.ImportPreflightOptions{
					OrgName:      orgName,
					InputPath:    input,
					DryRun:       dryRun,
					IncludeList:  includeList,
					SkipEntities: skipEntities,
				})
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

			// Execute import
			result, err := importModule.Execute(cmd.Context(), import_module.Options{
				InputPath:                     input,
				DryRun:                        dryRun,
				SkipEntities:                  skipEntities,
				SkipSystemBlueprints:          skipSystemBlueprints,
				SkipSystemBlueprintProperties: skipSystemBlueprintProperties,
				IncludeRuleResults:            includeRuleResults,
				IncludeResources:              includeList,
				ExcludeBlueprints:             excludeBlueprintList,
				ExcludeBlueprintSchema:        excludeBlueprintSchemaList,
				UsersAsDisabled:               usersAsDisabled,
				Verbose:                       verbose,
				ShowPagesPipeline:             showPagesPipeline,
				ProgressCallback:              progressCallback,
				LogCallback:                   logCallback,
			})

			// Clear progress line
			if outputFormat != "json" && progressCallback != nil {
				output.Printf("\n")
			}

			return importRenderer.Render(result, err, render.ImportResultOptions{
				Format:            render.Format(outputFormat),
				MaxErrors:         maxErrors,
				Verbose:           verbose,
				ShowPagesPipeline: showPagesPipeline,
			})
		},
	}

	importCmd.Flags().StringVarP(&input, "input", "i", "", "Input file path (e.g., backup.tar.gz or backup.json)")
	importCmd.MarkFlagRequired("input")
	importCmd.Flags().StringVar(&org, "org", "", "Target organization name (uses default if not specified, deprecated: use --target-org)")
	importCmd.Flags().StringVar(&targetOrg, "target-org", "", "Target organization name (uses default if not specified)")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate import without applying changes")
	importCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip importing entities (only import schema and configuration)")
	importCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	importCmd.Flags().BoolVar(&skipSystemBlueprintProperties, "skip-system-blueprint-properties", false, "When used with --skip-system-blueprints, do not import custom properties on known system blueprints")
	importCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	importCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to import (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, imports all resources.")
	importCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	importCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still imported)")
	importCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	importCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed error information with categorization")
	importCmd.Flags().BoolVar(&showPagesPipeline, "show-pages-pipeline", false, "Show the planned sidebar pages/folders pipeline before execution and include the pipeline used in the output")
	importCmd.Flags().BoolVar(&usersAsDisabled, "users-as-disabled", false, "Import non-admin users as DISABLED (admin users are imported normally)")
	importCmd.Flags().IntVar(&maxErrors, "max-errors", defaultMaxErrors, "Maximum number of errors to show in text output (-1 hides errors, 0 shows all)")

	rootCmd.AddCommand(importCmd)
}

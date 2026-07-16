package commands

import (
	"fmt"

	"github.com/port-experimental/port-cli/internal/commands/resourceflags"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/port-experimental/port-cli/internal/render"
	"github.com/spf13/cobra"
)

// RegisterMigrate registers the migrate command.
func RegisterMigrate(rootCmd *cobra.Command) {
	var (
		sourceOrg                     string
		baseOrg                       string
		targetOrg                     string
		blueprints                    string
		dryRun                        bool
		skipEntities                  bool
		skipSystemBlueprints          bool
		skipSystemBlueprintProperties bool
		includeRuleResults            bool
		include                       string
		outputFormat                  string
		excludeBlueprints             string
		excludeBlueprintSchema        string
		usersAsDisabled               bool
		maxErrors                     int
		onError                       []string

		scorecards   string
		actions      string
		pages        string
		integrations string
		teams        string
		users        string
		entities     string
	)

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between Port organizations",
		Long: `Migrate data between Port organizations.

Migrates blueprints, entities, scorecards, actions, teams, users, pages, and integrations from source to target organization.
Use --skip-entities to only migrate configuration without entity data.
Use --include to selectively migrate specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateStringEnum("--output-format", outputFormat, []string{"text", "json"}); err != nil {
				return err
			}

			flags := GetGlobalFlags(cmd.Context())
			rt := NewRuntime(cmd.Context())

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
			if err := validateMaxErrorsFlag(maxErrors); err != nil {
				return err
			}
			errorHandling, err := buildErrorHandlingOptions(cmd, onError)
			if err != nil {
				return err
			}

			blueprintList := resourceflags.ParseCSV(blueprints)
			excludeBlueprintList := resourceflags.ParseCSV(excludeBlueprints)
			excludeBlueprintSchemaList := resourceflags.ParseCSV(excludeBlueprintSchema)

			filterRefs := resourceflags.PerResourceFlagRefs{
				Blueprints: &blueprints, Scorecards: &scorecards, Actions: &actions,
				Pages: &pages, Integrations: &integrations, Teams: &teams,
				Users: &users, Entities: &entities,
			}
			selection, err := resourceflags.BuildSelection(resourceflags.BuildInput{
				IncludeCSV:        include,
				BlueprintsChanged: cmd.Flags().Changed("blueprints"),
				Changed: resourceflags.Changed(cmd.Flags(),
					"entities", "scorecards", "actions", "pages", "integrations", "teams", "users",
				),
				Filters: resourceflags.ParseFilters(filterRefs),
			})
			if err != nil {
				return err
			}

			var skipWarnings []string
			skipEntities, skipWarnings = resourceflags.ReconcileSkipEntities(selection.IncludeResources, skipEntities)
			for _, w := range skipWarnings {
				output.WarningPrintln(w)
			}

			// Create migration module
			sourceClient, targetClient, err := rt.SourceTargetClients(cmd.Context(), sourceOrgName, targetOrg)
			if err != nil {
				return err
			}
			migrateModule := migrate.NewModuleFromClients(sourceClient, targetClient)
			defer migrateModule.Close()

			migrateRenderer := render.MigrateRenderer{}
			if outputFormat != "json" {
				migrateRenderer.PrintPreflight(render.MigratePreflightOptions{
					SourceOrg:       sourceOrgName,
					TargetOrg:       targetOrg,
					BlueprintList:   blueprintList,
					EntityList:      selection.Filters.Entities,
					ScorecardList:   selection.Filters.Scorecards,
					ActionList:      selection.Filters.Actions,
					PageList:        selection.Filters.Pages,
					IntegrationList: selection.Filters.Integrations,
					TeamList:        selection.Filters.Teams,
					UserList:        selection.Filters.Users,
					IncludeList:     selection.IncludeResources,
					SkipEntities:    skipEntities,
					DryRun:          dryRun,
				})
			}

			// Execute migration
			result, err := migrateModule.Execute(cmd.Context(), migrate.Options{
				Blueprints:                    blueprintList,
				DryRun:                        dryRun,
				SkipEntities:                  skipEntities,
				SkipSystemBlueprints:          skipSystemBlueprints,
				SkipSystemBlueprintProperties: skipSystemBlueprintProperties,
				IncludeRuleResults:            includeRuleResults,
				IncludeResources:              selection.IncludeResources,
				AutoScopeBlueprints:           selection.AutoScopeBlueprints,
				ExcludeBlueprints:             excludeBlueprintList,
				ExcludeBlueprintSchema:        excludeBlueprintSchemaList,
				UsersAsDisabled:               usersAsDisabled,
				ErrorHandling:                 errorHandling,
				Entities:                      selection.Filters.Entities,
				Scorecards:                    selection.Filters.Scorecards,
				Actions:                       selection.Filters.Actions,
				Pages:                         selection.Filters.Pages,
				Integrations:                  selection.Filters.Integrations,
				Teams:                         selection.Filters.Teams,
				Users:                         selection.Filters.Users,
			})
			return migrateRenderer.Render(result, err, render.MigrateResultOptions{
				Format:    render.Format(outputFormat),
				MaxErrors: maxErrors,
				Verbose:   flags.Verbose,
			})
		},
	}

	migrateCmd.Flags().StringVarP(&sourceOrg, "source-org", "s", "", "Source organization name (base org)")
	migrateCmd.Flags().StringVar(&baseOrg, "base-org", "", "Base organization name (alias for --source-org)")
	migrateCmd.Flags().StringVarP(&targetOrg, "target-org", "t", "", "Target organization name")
	migrateCmd.MarkFlagRequired("target-org")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate migration without applying changes")
	migrateCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip migrating entities (only migrate schema and configuration)")
	migrateCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	migrateCmd.Flags().BoolVar(&skipSystemBlueprintProperties, "skip-system-blueprint-properties", false, "When used with --skip-system-blueprints, do not migrate custom properties on known system blueprints")
	migrateCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	resourceflags.RegisterInclude(migrateCmd, &include, "migrate")
	migrateCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	migrateCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still migrated)")
	migrateCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	migrateCmd.Flags().BoolVar(&usersAsDisabled, "users-as-disabled", false, "Import non-admin users as DISABLED (admin users are imported normally)")
	migrateCmd.Flags().IntVar(&maxErrors, "max-errors", defaultMaxErrors, "Maximum number of errors to show in text output (-1 hides errors, 0 shows all)")
	migrateCmd.Flags().StringArrayVar(&onError, "on-error", nil, "Handle a Port API error type (repeatable, e.g. forbidden_format_change=ignore-property or forbidden_format_change=recreate-property)")

	filterRefs := resourceflags.PerResourceFlagRefs{
		Blueprints: &blueprints, Scorecards: &scorecards, Actions: &actions,
		Pages: &pages, Integrations: &integrations, Teams: &teams,
		Users: &users, Entities: &entities,
	}
	resourceflags.RegisterPerResourceFilters(migrateCmd, filterRefs, "migrate")

	rootCmd.AddCommand(migrateCmd)
}

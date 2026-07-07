package commands

import (
	"fmt"

	"github.com/port-experimental/port-cli/internal/commands/resourceflags"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/port-experimental/port-cli/internal/render"
	"github.com/port-experimental/port-cli/internal/snapshot"
	"github.com/spf13/cobra"
)

// RegisterExport registers the export command.
func RegisterExport(rootCmd *cobra.Command) {
	var (
		outputPath                    string
		org                           string
		baseOrg                       string
		blueprints                    string
		excludeBlueprints             string
		excludeBlueprintSchema        string
		format                        string
		skipEntities                  bool
		skipSystemBlueprints          bool
		skipSystemBlueprintProperties bool
		includeRuleResults            bool
		include                       string
		outputFormat                  string
		maxErrors                     int

		scorecards   string
		actions      string
		pages        string
		integrations string
		teams        string
		users        string
		entities     string
	)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export data from Port",
		Long: `Export data from Port organization.

Exports blueprints, entities, scorecards, actions, and teams to a file.
Use --skip-entities to only export configuration without entity data.
Use --include to selectively export specific resource types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateStringEnum("--output-format", outputFormat, []string{"text", "json"}); err != nil {
				return err
			}
			if format != "" {
				if err := validateStringEnum("--format", format, []string{"tar", "json"}); err != nil {
					return err
				}
			}

			rt := NewRuntime(cmd.Context())

			// Use base-org if provided, otherwise use org
			orgName := baseOrg
			if orgName == "" {
				orgName = org
			}

			client, useOrg, err := rt.ClientForBaseOrg(cmd.Context(), orgName)
			if err != nil {
				return err
			}
			defer client.Close()
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

			blueprintList := resourceflags.ParseCSV(blueprints)
			exportOpts := export.Options{
				OutputPath:                    outputPath,
				Blueprints:                    blueprintList,
				ExcludeBlueprints:             excludeBlueprintList,
				ExcludeBlueprintSchema:        excludeBlueprintSchemaList,
				Format:                        format,
				SkipEntities:                  skipEntities,
				SkipSystemBlueprints:          skipSystemBlueprints,
				SkipSystemBlueprintProperties: skipSystemBlueprintProperties,
				IncludeRuleResults:            includeRuleResults,
				IncludeResources:              selection.IncludeResources,
				AutoScopeBlueprints:           selection.AutoScopeBlueprints,
				Entities:                      selection.Filters.Entities,
				Scorecards:                    selection.Filters.Scorecards,
				Actions:                       selection.Filters.Actions,
				Pages:                         selection.Filters.Pages,
				Integrations:                  selection.Filters.Integrations,
				Teams:                         selection.Filters.Teams,
				Users:                         selection.Filters.Users,
			}
			if err := exportOpts.Validate(); err != nil {
				return err
			}

			if err := validateMaxErrorsFlag(maxErrors); err != nil {
				return err
			}

			snap, err := snapshot.NewCollector(client).Collect(cmd.Context(), useOrg, snapshot.ExportMetadataCollectPlan(exportOpts))
			if err != nil {
				return fmt.Errorf("export collection failed: %w", err)
			}

			exportModule := export.NewModuleFromClient(client)
			defer exportModule.Close()

			exportRenderer := render.ExportRenderer{}
			if outputFormat != "json" {
				exportRenderer.PrintPreflight(render.ExportPreflightOptions{
					OrgName:         orgName,
					OutputPath:      outputPath,
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
				})
			}

			result, err := exportModule.ExecuteWithCollectedData(cmd.Context(), snap.Data, exportOpts)
			return exportRenderer.Render(result, err, render.ExportResultOptions{
				Format:                   render.Format(outputFormat),
				SkipEntities:             skipEntities,
				IncludedResources:        selection.IncludeResources,
				ExcludedBlueprints:       excludeBlueprintList,
				SchemaExcludedBlueprints: excludeBlueprintSchemaList,
				MaxErrors:                maxErrors,
			})
		},
	}

	exportCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (e.g., backup.tar.gz or backup.json)")
	exportCmd.MarkFlagRequired("output")
	exportCmd.Flags().StringVar(&org, "org", "", "Base organization name (uses default if not specified, deprecated: use --base-org)")
	exportCmd.Flags().StringVar(&baseOrg, "base-org", "", "Base organization name (uses default if not specified)")
	exportCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	exportCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still exported)")
	exportCmd.Flags().StringVarP(&format, "format", "f", "", "Export format: tar (tar.gz) or json")
	exportCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip exporting entities (only export schema and configuration)")
	exportCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	exportCmd.Flags().BoolVar(&skipSystemBlueprintProperties, "skip-system-blueprint-properties", false, "When used with --skip-system-blueprints, do not export custom properties on known system blueprints")
	exportCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	resourceflags.RegisterInclude(exportCmd, &include, "export")
	exportCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	exportCmd.Flags().IntVar(&maxErrors, "max-errors", defaultMaxErrors, "Maximum number of errors to show in text output (-1 hides errors, 0 shows all)")

	filterRefs := resourceflags.PerResourceFlagRefs{
		Blueprints: &blueprints, Scorecards: &scorecards, Actions: &actions,
		Pages: &pages, Integrations: &integrations, Teams: &teams,
		Users: &users, Entities: &entities,
	}
	resourceflags.RegisterPerResourceFilters(exportCmd, filterRefs, "export")

	rootCmd.AddCommand(exportCmd)
}

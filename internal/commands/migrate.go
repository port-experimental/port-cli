package commands

import (
	"fmt"
	"slices"
	"strings"

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

			// Parse blueprints list
			var blueprintList []string
			if blueprints != "" {
				blueprintList = strings.Split(blueprints, ",")
				for i := range blueprintList {
					blueprintList[i] = strings.TrimSpace(blueprintList[i])
				}
			}

			// Parse per-resource ID filters
			parseCSV := func(s string) []string {
				if s == "" {
					return nil
				}
				parts := strings.Split(s, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				return parts
			}
			entityList := parseCSV(entities)
			scorecardList := parseCSV(scorecards)
			actionList := parseCSV(actions)
			pageList := parseCSV(pages)
			integrationList := parseCSV(integrations)
			teamList := parseCSV(teams)
			userList := parseCSV(users)

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

			// True when the caller explicitly wants blueprint schemas, either via
			// --blueprints or --include blueprints — as opposed to blueprints only
			// being pulled in as a byproduct of --actions/--scorecards/--entities.
			blueprintsExplicitlyRequested := cmd.Flags().Changed("blueprints") || slices.Contains(includeList, "blueprints")

			// Auto-include resource types when per-resource flags are explicitly set
			// (with or without specific IDs — Changed() detects explicit flag usage)
			ensureContains := func(list []string, item string) []string {
				for _, v := range list {
					if v == item {
						return list
					}
				}
				return append(list, item)
			}
			needBlueprints := false
			if len(entityList) > 0 || cmd.Flags().Changed("entities") {
				includeList = ensureContains(includeList, "entities")
				needBlueprints = true
			}
			if len(scorecardList) > 0 || cmd.Flags().Changed("scorecards") {
				includeList = ensureContains(includeList, "scorecards")
				needBlueprints = true
			}
			if len(actionList) > 0 || cmd.Flags().Changed("actions") {
				includeList = ensureContains(includeList, "actions")
				includeList = ensureContains(includeList, "action-permissions")
				needBlueprints = true
			}
			if len(pageList) > 0 || cmd.Flags().Changed("pages") {
				includeList = ensureContains(includeList, "pages")
				includeList = ensureContains(includeList, "page-permissions")
			}
			if len(integrationList) > 0 || cmd.Flags().Changed("integrations") {
				includeList = ensureContains(includeList, "integrations")
			}
			if len(teamList) > 0 || cmd.Flags().Changed("teams") {
				includeList = ensureContains(includeList, "teams")
			}
			if len(userList) > 0 || cmd.Flags().Changed("users") {
				includeList = ensureContains(includeList, "users")
			}
			if cmd.Flags().Changed("blueprints") || (needBlueprints && len(includeList) > 0) {
				includeList = ensureContains(includeList, "blueprints")
			}
			autoScopeBlueprints := needBlueprints && !blueprintsExplicitlyRequested

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
					EntityList:      entityList,
					ScorecardList:   scorecardList,
					ActionList:      actionList,
					PageList:        pageList,
					IntegrationList: integrationList,
					TeamList:        teamList,
					UserList:        userList,
					IncludeList:     includeList,
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
				IncludeResources:              includeList,
				AutoScopeBlueprints:           autoScopeBlueprints,
				ExcludeBlueprints:             excludeBlueprintList,
				ExcludeBlueprintSchema:        excludeBlueprintSchemaList,
				UsersAsDisabled:               usersAsDisabled,
				Entities:                      entityList,
				Scorecards:                    scorecardList,
				Actions:                       actionList,
				Pages:                         pageList,
				Integrations:                  integrationList,
				Teams:                         teamList,
				Users:                         userList,
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
	migrateCmd.Flags().StringVarP(&blueprints, "blueprints", "b", "", "Comma-separated list of blueprint IDs to migrate (restricts migration to blueprints resource type; migrates all blueprints if flag set without IDs; pass this flag explicitly to migrate the full blueprint set even when combined with --actions/--scorecards/--entities)")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate migration without applying changes")
	migrateCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip migrating entities (only migrate schema and configuration)")
	migrateCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	migrateCmd.Flags().BoolVar(&skipSystemBlueprintProperties, "skip-system-blueprint-properties", false, "When used with --skip-system-blueprints, do not migrate custom properties on known system blueprints")
	migrateCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	migrateCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to migrate (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, migrates all resources.")
	migrateCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	migrateCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still migrated)")
	migrateCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	migrateCmd.Flags().BoolVar(&usersAsDisabled, "users-as-disabled", false, "Import non-admin users as DISABLED (admin users are imported normally)")
	migrateCmd.Flags().IntVar(&maxErrors, "max-errors", defaultMaxErrors, "Maximum number of errors to show in text output (-1 hides errors, 0 shows all)")

	migrateCmd.Flags().StringVar(&scorecards, "scorecards", "", "Comma-separated scorecard IDs to migrate (restricts migration to scorecards resource type; blueprint schemas migrated alongside are scoped to only the blueprints the selected scorecards belong to — use --blueprints to migrate the full set instead)")
	migrateCmd.Flags().StringVar(&actions, "actions", "", "Comma-separated action IDs to migrate (restricts migration to actions resource type; migrates all actions if flag set without IDs; blueprint schemas migrated alongside are scoped to only the blueprints the selected actions belong to — use --blueprints to migrate the full set instead)")
	migrateCmd.Flags().StringVar(&pages, "pages", "", "Comma-separated page IDs to migrate (restricts migration to pages resource type)")
	migrateCmd.Flags().StringVar(&integrations, "integrations", "", "Comma-separated integration IDs to migrate (restricts migration to integrations resource type; exports integration mapping only)")
	migrateCmd.Flags().StringVar(&teams, "teams", "", "Comma-separated team names to migrate (restricts migration to teams resource type)")
	migrateCmd.Flags().StringVar(&users, "users", "", "Comma-separated user emails to migrate (restricts migration to users resource type)")
	migrateCmd.Flags().StringVar(&entities, "entities", "", "Comma-separated entity IDs to migrate (restricts migration to entities resource type; blueprint schemas migrated alongside are scoped to only the blueprints the selected entities belong to — use --blueprints to migrate the full set instead)")

	rootCmd.AddCommand(migrateCmd)
}

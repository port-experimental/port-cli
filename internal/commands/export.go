package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/output"
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

				// page-permissions requires pages so that page identifiers can be collected
				hasPagePerms := slices.Contains(includeList, "page-permissions")
				hasPages := slices.Contains(includeList, "pages")
				if hasPagePerms && !hasPages {
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
			// --blueprints also restricts to blueprints resource type, consistent with other per-resource flags
			if cmd.Flags().Changed("blueprints") || (needBlueprints && len(includeList) > 0) {
				includeList = ensureContains(includeList, "blueprints")
			}

			token, err := configManager.GetOrRefreshToken(cmd.Context(), orgName)
			if err != nil {
				if !config.ShouldIgnoreGetOrRefreshTokenError(err) {
					return err
				}
			}
			// Create export module
			exportModule := export.NewModule(token, orgConfig)
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
				if len(entityList) > 0 {
					output.Printf("Entities filter: %s\n", strings.Join(entityList, ", "))
				}
				if len(scorecardList) > 0 {
					output.Printf("Scorecards filter: %s\n", strings.Join(scorecardList, ", "))
				}
				if len(actionList) > 0 {
					output.Printf("Actions filter: %s\n", strings.Join(actionList, ", "))
				}
				if len(pageList) > 0 {
					output.Printf("Pages filter: %s\n", strings.Join(pageList, ", "))
				}
				if len(integrationList) > 0 {
					output.Printf("Integrations filter: %s\n", strings.Join(integrationList, ", "))
				}
				if len(teamList) > 0 {
					output.Printf("Teams filter: %s\n", strings.Join(teamList, ", "))
				}
				if len(userList) > 0 {
					output.Printf("Users filter: %s\n", strings.Join(userList, ", "))
				}
				if len(includeList) > 0 {
					output.Printf("Including only: %s\n", strings.Join(includeList, ", "))
				} else if skipEntities {
					output.Printf("Skipping entities (schema only)\n")
				}
			}

			// Execute export
			result, err := exportModule.Execute(cmd.Context(), export.Options{
				OutputPath:                    outputPath,
				Blueprints:                    blueprintList,
				ExcludeBlueprints:             excludeBlueprintList,
				ExcludeBlueprintSchema:        excludeBlueprintSchemaList,
				Format:                        format,
				SkipEntities:                  skipEntities,
				SkipSystemBlueprints:          skipSystemBlueprints,
				SkipSystemBlueprintProperties: skipSystemBlueprintProperties,
				IncludeRuleResults:            includeRuleResults,
				IncludeResources:              includeList,
				Entities:                      entityList,
				Scorecards:                    scorecardList,
				Actions:                       actionList,
				Pages:                         pageList,
				Integrations:                  integrationList,
				Teams:                         teamList,
				Users:                         userList,
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
					"output_path":        result.OutputPath,
					"blueprints_count":   result.BlueprintsCount,
					"entities_count":     result.EntitiesCount,
					"actions_count":      result.ActionsCount,
					"users_count":        result.UsersCount,
					"teams_count":        result.TeamsCount,
					"pages_count":        result.PagesCount,
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
	exportCmd.Flags().StringVarP(&blueprints, "blueprints", "b", "", "Comma-Separated list of blueprint IDs to export (restricts export to blueprints resource type; exports all blueprints if flag set without IDs)")
	exportCmd.Flags().StringVar(&excludeBlueprints, "exclude-blueprints", "", "Comma-separated blueprint IDs to exclude entirely (schema + entities + scorecards + actions)")
	exportCmd.Flags().StringVar(&excludeBlueprintSchema, "exclude-blueprint-schema", "", "Comma-separated blueprint IDs to exclude schema only (entities, scorecards, actions still exported)")
	exportCmd.Flags().StringVarP(&format, "format", "f", "", "Export format: tar (tar.gz) or json")
	exportCmd.Flags().BoolVar(&skipEntities, "skip-entities", false, "Skip exporting entities (only export schema and configuration)")
	exportCmd.Flags().BoolVar(&skipSystemBlueprints, "skip-system-blueprints", false, "Skip system blueprint schemas (identifiers starting with _) and their entities")
	exportCmd.Flags().BoolVar(&skipSystemBlueprintProperties, "skip-system-blueprint-properties", false, "When used with --skip-system-blueprints, do not export custom properties on known system blueprints")
	exportCmd.Flags().BoolVar(&includeRuleResults, "include-rule-results", true, "Include _rule_result system blueprint entities (use --include-rule-results=false to exclude)")
	exportCmd.Flags().StringVar(&include, "include", "", "Comma-separated list of resources to export (e.g., 'blueprints,pages'). Available: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations. If not specified, exports all resources.")
	exportCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")

	exportCmd.Flags().StringVar(&scorecards, "scorecards", "", "Comma-Separated scorecard IDs to export (restricts export to scorecards resource type)")
	exportCmd.Flags().StringVar(&actions, "actions", "", "Comma-Separated action IDs to export (restricts export to actions resource type; exports all actions if flag set without IDs)")
	exportCmd.Flags().StringVar(&pages, "pages", "", "Comma-Separated page IDs to export (restricts export to pages resource type)")
	exportCmd.Flags().StringVar(&integrations, "integrations", "", "Comma-separated integration IDs to export (restricts export to integrations resource type; exports integration mapping only)")
	exportCmd.Flags().StringVar(&teams, "teams", "", "Comma-Separated team names to export (restricts export to teams resource type)")
	exportCmd.Flags().StringVar(&users, "users", "", "Comma-Separated user emails to export (restricts export to users resource type)")
	exportCmd.Flags().StringVar(&entities, "entities", "", "Comma-Separated entity IDs to export (restricts export to entities resource type)")

	rootCmd.AddCommand(exportCmd)
}

package render

import (
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
)

// MigrateRenderer formats migrate command output.
type MigrateRenderer struct{}

// MigratePreflightOptions configures pre-execution text output.
type MigratePreflightOptions struct {
	SourceOrg       string
	TargetOrg       string
	BlueprintList   []string
	EntityList      []string
	ScorecardList   []string
	ActionList      []string
	PageList        []string
	IntegrationList []string
	TeamList        []string
	UserList        []string
	IncludeList     []string
	SkipEntities    bool
	DryRun          bool
}

// MigrateResultOptions configures migrate result rendering.
type MigrateResultOptions struct {
	Format    Format
	MaxErrors int
	Verbose   bool
}

// PrintPreflight prints migration setup details for text mode.
func (MigrateRenderer) PrintPreflight(opts MigratePreflightOptions) {
	output.Printf("\nMigration:\n")
	output.Printf("  Source (base org): %s\n", opts.SourceOrg)
	output.Printf("  Target org: %s\n", opts.TargetOrg)
	printFilter := func(label string, values []string) {
		if len(values) > 0 {
			output.Printf("  %s filter: %s\n", label, joinCSV(values))
		}
	}
	printFilter("Blueprints", opts.BlueprintList)
	printFilter("Entities", opts.EntityList)
	printFilter("Scorecards", opts.ScorecardList)
	printFilter("Actions", opts.ActionList)
	printFilter("Pages", opts.PageList)
	printFilter("Integrations", opts.IntegrationList)
	printFilter("Teams", opts.TeamList)
	printFilter("Users", opts.UserList)
	output.Printf("Diff validation enabled - comparing source with target organization state\n")
	if len(opts.IncludeList) > 0 {
		output.Printf("  Including only: %s\n", joinCSV(opts.IncludeList))
	} else if opts.SkipEntities {
		output.Printf("  Skipping entities (schema only)\n")
	}
	if opts.DryRun {
		output.Printf("  Dry run mode - no changes will be applied\n")
	}
}

// Render formats migrate results.
func (r MigrateRenderer) Render(result *migrate.Result, execErr error, opts MigrateResultOptions) error {
	if execErr != nil {
		return r.renderExecutionError(execErr, result, opts)
	}
	if result == nil {
		return fmt.Errorf("migration failed: no result")
	}
	if !result.Success {
		return r.renderFailure(result, opts)
	}

	if opts.Format == FormatJSON {
		return r.renderJSON(result)
	}
	return r.renderText(result, opts)
}

func (MigrateRenderer) renderExecutionError(execErr error, result *migrate.Result, opts MigrateResultOptions) error {
	failureMessage := migrationExecutionErrorMessage(execErr, result, opts.MaxErrors)
	if opts.Format == FormatJSON {
		jsonData := map[string]interface{}{
			"success": false,
			"error":   failureMessage,
		}
		if result != nil {
			populateMigrateCounts(jsonData, result)
			if len(result.Errors) > 0 {
				jsonData["errors"] = result.Errors
			}
			if len(result.Warnings) > 0 {
				jsonData["warnings"] = result.Warnings
			}
		}
		output.PrintJSON(jsonData)
		return fmt.Errorf("%s", failureMessage)
	}

	output.ErrorPrintf("%s\n", failureMessage)
	if result != nil {
		output.Printf("\nPartial migration results:\n")
		output.Printf("Blueprints created: %d, updated: %d, skipped: %d\n", result.BlueprintsCreated, result.BlueprintsUpdated, result.BlueprintsSkipped)
		output.Printf("Entities created: %d, updated: %d, skipped: %d\n", result.EntitiesCreated, result.EntitiesUpdated, result.EntitiesSkipped)
		output.Printf("Scorecards created: %d, updated: %d, skipped: %d\n", result.ScorecardsCreated, result.ScorecardsUpdated, result.ScorecardsSkipped)
		output.Printf("Actions created: %d, updated: %d, skipped: %d\n", result.ActionsCreated, result.ActionsUpdated, result.ActionsSkipped)
		output.Printf("Teams created: %d, updated: %d, skipped: %d\n", result.TeamsCreated, result.TeamsUpdated, result.TeamsSkipped)
		output.Printf("Users created: %d, updated: %d, skipped: %d\n", result.UsersCreated, result.UsersUpdated, result.UsersSkipped)
		output.Printf("Pages created: %d, updated: %d, skipped: %d\n", result.PagesCreated, result.PagesUpdated, result.PagesSkipped)
		output.Printf("Integrations updated: %d, skipped: %d\n", result.IntegrationsUpdated, result.IntegrationsSkipped)
	}
	return fmt.Errorf("%s", failureMessage)
}

func (MigrateRenderer) renderFailure(result *migrate.Result, opts MigrateResultOptions) error {
	failureMessage := migrationFailureMessage(result, opts.MaxErrors)
	if opts.Format == FormatJSON {
		jsonData := map[string]interface{}{
			"success": false,
			"error":   failureMessage,
		}
		if len(result.Errors) > 0 {
			jsonData["errors"] = result.Errors
		}
		if len(result.Warnings) > 0 {
			jsonData["warnings"] = result.Warnings
		}
		output.PrintJSON(jsonData)
		return fmt.Errorf("%s", failureMessage)
	}
	return fmt.Errorf("%s", failureMessage)
}

func (MigrateRenderer) renderJSON(result *migrate.Result) error {
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
	addMigrationDetailJSON(jsonData, result)
	return output.PrintJSON(jsonData)
}

func (MigrateRenderer) renderText(result *migrate.Result, opts MigrateResultOptions) error {
	output.SuccessPrintln("\n✓ Migration completed successfully!")
	output.Printf("%s\n", result.Message)
	if result.IgnoredRuleResultTargetRelationCount > 0 {
		output.Printf("\n_rule_result: ignored %d relation(s) with type rule_result_target (not sent to API): %s\n",
			result.IgnoredRuleResultTargetRelationCount,
			strings.Join(result.IgnoredRuleResultTargetRelationKeys, ", "))
	}

	printDiffStats(result.DiffResult, false)

	output.Printf("Blueprints created: %d, updated: %d, skipped: %d\n", result.BlueprintsCreated, result.BlueprintsUpdated, result.BlueprintsSkipped)
	output.Printf("Entities created: %d, updated: %d, skipped: %d\n", result.EntitiesCreated, result.EntitiesUpdated, result.EntitiesSkipped)
	output.Printf("Scorecards created: %d, updated: %d, skipped: %d\n", result.ScorecardsCreated, result.ScorecardsUpdated, result.ScorecardsSkipped)
	output.Printf("Actions created: %d, updated: %d, skipped: %d\n", result.ActionsCreated, result.ActionsUpdated, result.ActionsSkipped)
	output.Printf("Teams created: %d, updated: %d, skipped: %d\n", result.TeamsCreated, result.TeamsUpdated, result.TeamsSkipped)
	output.Printf("Users created: %d, updated: %d, skipped: %d\n", result.UsersCreated, result.UsersUpdated, result.UsersSkipped)
	output.Printf("Pages created: %d, updated: %d, skipped: %d\n", result.PagesCreated, result.PagesUpdated, result.PagesSkipped)
	output.Printf("Integrations updated: %d, skipped: %d\n", result.IntegrationsUpdated, result.IntegrationsSkipped)
	if opts.Verbose {
		printMigrationVerboseDetails(result)
	}
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
		limit := errorLimit(len(result.Errors), opts.MaxErrors)
		if limit > 0 {
			output.Printf("\nErrors:\n")
			for i := 0; i < limit; i++ {
				output.Printf("  - %s\n", result.Errors[i])
			}
			if len(result.Errors) > limit {
				output.Printf("  ... and %d more\n", len(result.Errors)-limit)
			}
		}
	}
	return nil
}

func populateMigrateCounts(data map[string]interface{}, result *migrate.Result) {
	data["blueprints_created"] = result.BlueprintsCreated
	data["blueprints_updated"] = result.BlueprintsUpdated
	data["blueprints_skipped"] = result.BlueprintsSkipped
	data["entities_created"] = result.EntitiesCreated
	data["entities_updated"] = result.EntitiesUpdated
	data["entities_skipped"] = result.EntitiesSkipped
	data["scorecards_created"] = result.ScorecardsCreated
	data["scorecards_updated"] = result.ScorecardsUpdated
	data["scorecards_skipped"] = result.ScorecardsSkipped
	data["actions_created"] = result.ActionsCreated
	data["actions_updated"] = result.ActionsUpdated
	data["actions_skipped"] = result.ActionsSkipped
	data["teams_created"] = result.TeamsCreated
	data["teams_updated"] = result.TeamsUpdated
	data["teams_skipped"] = result.TeamsSkipped
	data["users_created"] = result.UsersCreated
	data["users_updated"] = result.UsersUpdated
	data["users_skipped"] = result.UsersSkipped
	data["pages_created"] = result.PagesCreated
	data["pages_updated"] = result.PagesUpdated
	data["pages_skipped"] = result.PagesSkipped
	data["integrations_updated"] = result.IntegrationsUpdated
	data["integrations_skipped"] = result.IntegrationsSkipped
}

func MigrationFailureMessage(result *migrate.Result, maxErrors int) string {
	return migrationFailureMessage(result, maxErrors)
}

func MigrationExecutionErrorMessage(err error, result *migrate.Result, maxErrors int) string {
	return migrationExecutionErrorMessage(err, result, maxErrors)
}

func migrationFailureMessage(result *migrate.Result, maxErrors int) string {
	if result == nil || len(result.Errors) == 0 {
		return "migration failed"
	}
	var b strings.Builder
	if result.Message != "" {
		b.WriteString(result.Message)
	} else {
		b.WriteString("migration failed")
	}
	limit := errorLimit(len(result.Errors), maxErrors)
	if limit == 0 {
		return b.String()
	}
	b.WriteString(":")
	for i := 0; i < limit; i++ {
		b.WriteString("\n  - ")
		b.WriteString(result.Errors[i])
	}
	if len(result.Errors) > limit {
		b.WriteString(fmt.Sprintf("\n  ... and %d more", len(result.Errors)-limit))
	}
	return b.String()
}

func migrationExecutionErrorMessage(err error, result *migrate.Result, maxErrors int) string {
	if result != nil && len(result.Errors) > 0 {
		return migrationFailureMessage(result, maxErrors)
	}
	if err == nil {
		return "migration failed"
	}
	return fmt.Sprintf("migration failed: %v", err)
}

func addMigrationDetailJSON(data map[string]interface{}, result *migrate.Result) {
	if result == nil {
		return
	}
	if len(result.BlueprintsToCreate) > 0 {
		data["blueprints_to_create"] = result.BlueprintsToCreate
	}
	if len(result.BlueprintsToUpdate) > 0 {
		data["blueprints_to_update"] = result.BlueprintsToUpdate
	}
	if len(result.BlueprintsToSkip) > 0 {
		data["blueprints_skipped_ids"] = result.BlueprintsToSkip
	}
	if len(result.BlueprintPermissionsToUpdate) > 0 {
		data["blueprint_permissions_to_update"] = result.BlueprintPermissionsToUpdate
	}
	if len(result.ActionPermissionsToUpdate) > 0 {
		data["action_permissions_to_update"] = result.ActionPermissionsToUpdate
	}
	if len(result.PagePermissionsToUpdate) > 0 {
		data["page_permissions_to_update"] = result.PagePermissionsToUpdate
	}
}

// AddMigrationDetailJSON exposes dry-run detail fields for contract tests.
func AddMigrationDetailJSON(data map[string]interface{}, result *migrate.Result) {
	addMigrationDetailJSON(data, result)
}

func printMigrationVerboseDetails(result *migrate.Result) {
	if result == nil {
		return
	}
	printedHeader := false
	printList := func(label string, values []string) {
		if len(values) == 0 {
			return
		}
		if !printedHeader {
			output.Printf("\nDry-run details:\n")
			printedHeader = true
		}
		output.Printf("  %s: %s\n", label, joinCSV(values))
	}
	printList("Blueprints to create", result.BlueprintsToCreate)
	printList("Blueprints to update", result.BlueprintsToUpdate)
	printList("Blueprints skipped", result.BlueprintsToSkip)
	printList("Blueprint permissions to update", result.BlueprintPermissionsToUpdate)
	printList("Action permissions to update", result.ActionPermissionsToUpdate)
	printList("Page permissions to update", result.PagePermissionsToUpdate)
}

package render

import (
	"strings"

	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/output"
)

func printDiffStats(diff *importmodule.DiffResult, showPermissions bool) {
	if diff == nil {
		return
	}

	output.Printf("\nDiff analysis:\n")
	printDiffLine := func(label string, create, update, skip int) {
		if create > 0 || update > 0 || skip > 0 {
			output.Printf("  %s: %d new, %d updated, %d skipped (identical)\n", label, create, update, skip)
		}
	}
	printDiffLine("Blueprints",
		len(diff.BlueprintsToCreate), len(diff.BlueprintsToUpdate), len(diff.BlueprintsToSkip))
	printDiffLine("Entities",
		len(diff.EntitiesToCreate), len(diff.EntitiesToUpdate), len(diff.EntitiesToSkip))
	printDiffLine("Scorecards",
		len(diff.ScorecardsToCreate), len(diff.ScorecardsToUpdate), len(diff.ScorecardsToSkip))
	printDiffLine("Actions",
		len(diff.ActionsToCreate), len(diff.ActionsToUpdate), len(diff.ActionsToSkip))
	printDiffLine("Teams",
		len(diff.TeamsToCreate), len(diff.TeamsToUpdate), len(diff.TeamsToSkip))
	printDiffLine("Users",
		len(diff.UsersToCreate), len(diff.UsersToUpdate), len(diff.UsersToSkip))
	printDiffLine("Pages",
		len(diff.PagesToCreate), len(diff.PagesToUpdate), len(diff.PagesToSkip))
	if len(diff.IntegrationsToUpdate) > 0 || len(diff.IntegrationsToSkip) > 0 {
		output.Printf("  Integrations: %d updated, %d skipped (identical)\n",
			len(diff.IntegrationsToUpdate), len(diff.IntegrationsToSkip))
	}
	if showPermissions {
		if len(diff.BlueprintPermissions) > 0 {
			output.Printf("  Blueprint permissions: %d to update\n", len(diff.BlueprintPermissions))
		}
		if len(diff.ActionPermissions) > 0 {
			output.Printf("  Action permissions: %d to update\n", len(diff.ActionPermissions))
		}
		if len(diff.PagePermissions) > 0 {
			output.Printf("  Page permissions: %d to update\n", len(diff.PagePermissions))
		}
	}
	output.Printf("\n")
}

func joinCSV(values []string) string {
	return strings.Join(values, ", ")
}

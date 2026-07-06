package render

import (
	"fmt"

	exportmodule "github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/output"
)

// ExportRenderer formats export command output.
type ExportRenderer struct{}

// ExportPreflightOptions configures pre-execution text output.
type ExportPreflightOptions struct {
	OrgName          string
	OutputPath       string
	BlueprintList    []string
	EntityList       []string
	ScorecardList    []string
	ActionList       []string
	PageList         []string
	IntegrationList  []string
	TeamList         []string
	UserList         []string
	IncludeList      []string
	SkipEntities     bool
}

// ExportResultOptions configures export result rendering.
type ExportResultOptions struct {
	Format                   Format
	SkipEntities             bool
	IncludedResources        []string
	ExcludedBlueprints       []string
	SchemaExcludedBlueprints []string
	MaxErrors                int
}

// PrintPreflight prints export setup details for text mode.
func (ExportRenderer) PrintPreflight(opts ExportPreflightOptions) {
	output.Printf("\nExporting data from base organization: %s\n", opts.OrgName)
	if opts.OrgName == "" {
		output.Printf("(using default organization)\n")
	}
	output.Printf("Output file: %s\n", opts.OutputPath)
	printFilter := func(label string, values []string) {
		if len(values) > 0 {
			output.Printf("%s filter: %s\n", label, joinCSV(values))
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
	if len(opts.IncludeList) > 0 {
		output.Printf("Including only: %s\n", joinCSV(opts.IncludeList))
	} else if opts.SkipEntities {
		output.Printf("Skipping entities (schema only)\n")
	}
}

// RenderExecutionError formats an export execution error.
func (ExportRenderer) RenderExecutionError(format Format, err error) error {
	if format == FormatJSON {
		output.PrintJSON(output.JSONResult{Success: false, Error: err.Error()})
		return err
	}
	return fmt.Errorf("export failed: %w", err)
}

// Render formats export results.
func (r ExportRenderer) Render(result *exportmodule.Result, execErr error, opts ExportResultOptions) error {
	if execErr != nil {
		return r.RenderExecutionError(opts.Format, execErr)
	}
	if result == nil {
		return fmt.Errorf("export failed: no result")
	}
	if !result.Success {
		if opts.Format == FormatJSON {
			output.PrintJSON(output.JSONResult{
				Success: false,
				Error:   fmt.Sprintf("%v", result.Error),
			})
			return fmt.Errorf("export failed: %v", result.Error)
		}
		return fmt.Errorf("export failed: %v", result.Error)
	}

	if opts.Format == FormatJSON {
		return r.renderJSON(result, opts)
	}
	return r.renderText(result, opts)
}

func (ExportRenderer) renderJSON(result *exportmodule.Result, opts ExportResultOptions) error {
	jsonData := exportJSONSummary(result, exportJSONSummaryOptions{
		SkipEntities:             opts.SkipEntities,
		IncludedResources:        opts.IncludedResources,
		ExcludedBlueprints:       opts.ExcludedBlueprints,
		SchemaExcludedBlueprints: opts.SchemaExcludedBlueprints,
	})
	if len(result.TimeoutErrors) > 0 {
		jsonData["timeout_errors"] = result.TimeoutErrors
		jsonData["warnings"] = fmt.Sprintf("%d blueprint(s) timed out during export", len(result.TimeoutErrors))
	}
	return output.PrintJSON(output.JSONResult{
		Success: true,
		Message: result.Message,
		Data:    jsonData,
	})
}

func (ExportRenderer) renderText(result *exportmodule.Result, opts ExportResultOptions) error {
	output.SuccessPrintln("\n✓ Export completed successfully!")
	output.Printf("%s\n", result.Message)
	output.Printf("Blueprints: %d\n", result.BlueprintsCount)
	output.Printf("Entities: %d\n", result.EntitiesCount)
	output.Printf("Actions: %d\n", result.ActionsCount)
	output.Printf("Users: %d\n", result.UsersCount)
	output.Printf("Teams: %d\n", result.TeamsCount)
	output.Printf("Pages: %d\n", result.PagesCount)
	output.Printf("Integrations: %d\n", result.IntegrationsCount)

	if len(result.TimeoutErrors) > 0 && shouldPrintErrors(len(result.TimeoutErrors), opts.MaxErrors) {
		output.WarningPrintln("\n⚠ Warning: Some blueprints timed out during export:")
		limit := errorLimit(len(result.TimeoutErrors), opts.MaxErrors)
		for i := 0; i < limit; i++ {
			output.WarningPrintf("  - %s\n", result.TimeoutErrors[i])
		}
		if len(result.TimeoutErrors) > limit {
			output.WarningPrintf("  ... and %d more\n", len(result.TimeoutErrors)-limit)
		}
		output.WarningPrintln("These blueprints were skipped. Consider exporting them separately or contact Port support if this persists.")
	}
	return nil
}

type exportJSONSummaryOptions struct {
	SkipEntities             bool
	IncludedResources        []string
	ExcludedBlueprints       []string
	SchemaExcludedBlueprints []string
}

func exportJSONSummary(result *exportmodule.Result, opts exportJSONSummaryOptions) map[string]interface{} {
	return map[string]interface{}{
		"output_path":          result.OutputPath,
		"format":               result.Format,
		"blueprints_count":     result.BlueprintsCount,
		"entities_count":       result.EntitiesCount,
		"actions_count":        result.ActionsCount,
		"users_count":          result.UsersCount,
		"teams_count":          result.TeamsCount,
		"folders_count":        result.FoldersCount,
		"pages_count":          result.PagesCount,
		"integrations_count":   result.IntegrationsCount,
		"skipped_entities":     opts.SkipEntities,
		"included_resources":   opts.IncludedResources,
		"excluded_blueprints":  opts.ExcludedBlueprints,
		"schema_only_excluded": opts.SchemaExcludedBlueprints,
	}
}

// ExportJSONSummary exposes the export JSON summary map for contract tests.
func ExportJSONSummary(result *exportmodule.Result, skipEntities bool, included, excluded, schemaExcluded []string) map[string]interface{} {
	return exportJSONSummary(result, exportJSONSummaryOptions{
		SkipEntities:             skipEntities,
		IncludedResources:        included,
		ExcludedBlueprints:       excluded,
		SchemaExcludedBlueprints: schemaExcluded,
	})
}

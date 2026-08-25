package render

import (
	"fmt"
	"strings"

	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/output"
)

// ImportRenderer formats import command output.
type ImportRenderer struct{}

// ImportPreflightOptions configures pre-execution text output.
type ImportPreflightOptions struct {
	OrgName      string
	InputPath    string
	DryRun       bool
	IncludeList  []string
	SkipEntities bool
}

// ImportResultOptions configures import result rendering.
type ImportResultOptions struct {
	Format            Format
	MaxErrors         int
	Verbose           bool
	ShowPagesPipeline bool
}

// PrintPreflight prints import setup details for text mode.
func (ImportRenderer) PrintPreflight(opts ImportPreflightOptions) {
	output.Printf("\nImporting data to target organization: %s\n", opts.OrgName)
	if opts.OrgName == "" {
		output.Printf("(using default organization)\n")
	}
	output.Printf("Input file: %s\n", opts.InputPath)
	if opts.DryRun {
		output.Printf("Dry run mode - no changes will be applied\n")
	}
	output.Printf("Diff validation enabled - comparing with current organization state\n")
	if len(opts.IncludeList) > 0 {
		output.Printf("Including only: %s\n", joinCSV(opts.IncludeList))
	} else if opts.SkipEntities {
		output.Printf("Skipping entities (schema only)\n")
	}
}

// RenderExecutionError formats an import execution error.
func (ImportRenderer) RenderExecutionError(format Format, err error) error {
	if format == FormatJSON {
		output.PrintJSON(output.JSONResult{Success: false, Error: err.Error()})
		return err
	}
	return fmt.Errorf("import failed: %w", err)
}

// Render formats import results.
func (r ImportRenderer) Render(result *importmodule.Result, execErr error, opts ImportResultOptions) error {
	if execErr != nil {
		return r.RenderExecutionError(opts.Format, execErr)
	}
	if result == nil {
		return fmt.Errorf("import failed: no result")
	}

	if opts.Format == FormatJSON {
		return r.renderJSON(result, opts)
	}
	return r.renderText(result, opts)
}

func (ImportRenderer) renderJSON(result *importmodule.Result, opts ImportResultOptions) error {
	jsonData := map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	}
	PopulateApplyCountsJSON(jsonData, ApplyCountsFromImport(result), false)
	if len(result.Errors) > 0 {
		jsonData["errors"] = result.Errors
	}
	if result.IgnoredRuleResultTargetRelationCount > 0 {
		jsonData["ignored_rule_result_target_relations_count"] = result.IgnoredRuleResultTargetRelationCount
		jsonData["ignored_rule_result_target_relation_keys"] = result.IgnoredRuleResultTargetRelationKeys
	}
	if opts.ShowPagesPipeline && len(result.SidebarPipeline) > 0 {
		jsonData["sidebar_pipeline"] = result.SidebarPipeline
	}
	output.PrintJSON(jsonData)
	if !result.Success {
		return fmt.Errorf("import completed with errors")
	}
	return nil
}

func (ImportRenderer) renderText(result *importmodule.Result, opts ImportResultOptions) error {
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

	printDiffStats(result.DiffResult, true)

	PrintApplyCountsText(ApplyCountsFromImport(result), false)

	if opts.ShowPagesPipeline && len(result.SidebarPipeline) > 0 {
		output.Printf("\nSidebar pipeline used:\n")
		for _, step := range result.SidebarPipeline {
			output.Printf("  %s\n", step)
		}
	}

	if len(result.Warnings) > 0 {
		output.Printf("\nWarnings:\n")
		for _, warning := range result.Warnings {
			output.WarningPrintln(fmt.Sprintf("  ⚠ %s", warning.Message))
			if opts.Verbose && len(warning.Details) > 0 {
				for _, detail := range warning.Details {
					output.Printf("      - %s\n", detail)
				}
			}
		}
	}

	if len(result.Errors) > 0 && shouldPrintErrors(len(result.Errors), opts.MaxErrors) {
		limit := errorLimit(len(result.Errors), opts.MaxErrors)
		if opts.Verbose && len(result.ErrorsByCategory) > 0 {
			output.Printf("\nErrors by category:\n")
			categoryOrder := []string{"DEPENDENCY", "VALIDATION", "SCHEMA_MISMATCH", "BLUEPRINT_CONFIG", "AUTH", "NOT_FOUND", "CONFLICT", "RATE_LIMIT", "NETWORK", "UNKNOWN"}
			displayed := 0
		categories:
			for _, category := range categoryOrder {
				if errs, ok := result.ErrorsByCategory[category]; ok && len(errs) > 0 {
					output.Printf("\n  %s (%d):\n", category, len(errs))
					for _, errMsg := range errs {
						if displayed >= limit {
							break categories
						}
						output.Printf("    - %s\n", errMsg)
						displayed++
					}
				}
			}
			if len(result.Errors) > displayed {
				output.Printf("\n  ... and %d more\n", len(result.Errors)-displayed)
			}
		} else {
			output.Printf("\nErrors encountered:\n")
			for i := 0; i < limit; i++ {
				output.Printf("  - %s\n", result.Errors[i])
			}
			if len(result.Errors) > limit {
				output.Printf("  ... and %d more\n", len(result.Errors)-limit)
			}
		}
	}

	if !result.Success {
		return fmt.Errorf("import completed with errors")
	}
	return nil
}

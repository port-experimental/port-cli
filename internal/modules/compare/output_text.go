// Package compare provides functionality for comparing two Port organizations.
package compare

import (
	"fmt"
	"io"
	"strings"
)

// TextFormatter formats comparison results as text.
type TextFormatter struct {
	w       io.Writer
	verbose bool
	full    bool
}

// NewTextFormatter creates a new text formatter.
func NewTextFormatter(w io.Writer, verbose, full bool) *TextFormatter {
	return &TextFormatter{
		w:       w,
		verbose: verbose,
		full:    full,
	}
}

// Format outputs the comparison result as text.
func (f *TextFormatter) Format(result *CompareResult) error {
	// Header
	fmt.Fprintf(f.w, "Comparing %s -> %s\n\n", result.Source, result.Target)

	// Summary for each resource type
	f.formatResourceType("Blueprints", result.Blueprints)
	f.formatResourceType("Actions", result.Actions)
	f.formatResourceType("Scorecards", result.Scorecards)
	f.formatResourceType("Pages", result.Pages)
	f.formatResourceType("Integrations", result.Integrations)
	f.formatResourceType("Teams", result.Teams)
	f.formatResourceType("Users", result.Users)
	f.formatResourceType("Automations", result.Automations)

	// Total
	total := f.calculateTotal(result)
	fmt.Fprintf(f.w, "\nTotal: %d added, %d modified, %d removed\n",
		total.Added, total.Modified, total.Removed)

	if result.Identical {
		fmt.Fprintf(f.w, "\nOrganizations are identical.\n")
	}

	return nil
}

func (f *TextFormatter) formatResourceType(name string, diff ResourceDiff) {
	s := diff.Summary

	if s.Added == 0 && s.Modified == 0 && s.Removed == 0 {
		fmt.Fprintf(f.w, "%-14s identical\n", name+":")
		return
	}

	fmt.Fprintf(f.w, "%-14s %d added, %d modified, %d removed\n",
		name+":", s.Added, s.Modified, s.Removed)

	// Verbose: show identifiers
	if f.verbose || f.full {
		if len(diff.Added) > 0 {
			ids := f.getIdentifiers(diff.Added)
			fmt.Fprintf(f.w, "  Added:    %s\n", strings.Join(ids, ", "))
		}
		if len(diff.Modified) > 0 {
			ids := f.getIdentifiers(diff.Modified)
			fmt.Fprintf(f.w, "  Modified: %s\n", strings.Join(ids, ", "))
		}
		if len(diff.Removed) > 0 {
			ids := f.getIdentifiers(diff.Removed)
			fmt.Fprintf(f.w, "  Removed:  %s\n", strings.Join(ids, ", "))
		}
	}

	// Full: show field-level diffs
	if f.full {
		for _, change := range diff.Added {
			fmt.Fprintf(f.w, "\n  [+] %s (added)\n", change.Identifier)
			f.formatData(change.TargetData, "      ")
		}
		for _, change := range diff.Modified {
			fmt.Fprintf(f.w, "\n  [~] %s (modified)\n", change.Identifier)
			for _, fd := range change.FieldDiffs {
				fmt.Fprintf(f.w, "      %s:\n", fd.Path)
				fmt.Fprintf(f.w, "        - %v\n", fd.SourceValue)
				fmt.Fprintf(f.w, "        + %v\n", fd.TargetValue)
			}
		}
		for _, change := range diff.Removed {
			fmt.Fprintf(f.w, "\n  [-] %s (removed)\n", change.Identifier)
		}
	}
}

func (f *TextFormatter) getIdentifiers(changes []ResourceChange) []string {
	ids := make([]string, len(changes))
	for i, c := range changes {
		ids[i] = c.Identifier
	}
	return ids
}

func (f *TextFormatter) formatData(data map[string]interface{}, indent string) {
	for k, v := range data {
		if ExcludedFields[k] {
			continue
		}
		fmt.Fprintf(f.w, "%s%s: %v\n", indent, k, v)
	}
}

func (f *TextFormatter) calculateTotal(result *CompareResult) DiffSummary {
	resources := []ResourceDiff{
		result.Blueprints, result.Actions, result.Scorecards,
		result.Pages, result.Integrations, result.Teams,
		result.Users, result.Automations,
	}

	var total DiffSummary
	for _, r := range resources {
		total.Added += r.Summary.Added
		total.Modified += r.Summary.Modified
		total.Removed += r.Summary.Removed
	}
	return total
}

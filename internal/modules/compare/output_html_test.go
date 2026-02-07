// internal/modules/compare/output_html_test.go
package compare

import (
	"bytes"
	"strings"
	"testing"
)

func TestHTMLFormatter_InteractiveReport(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Timestamp: "2026-02-05T19:30:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1, Modified: 1, Removed: 0},
			Added: []ResourceChange{
				{Identifier: "new-bp"},
			},
			Modified: []ResourceChange{
				{
					Identifier: "existing-bp",
					FieldDiffs: []FieldDiff{
						{Path: "title", SourceValue: "Old", TargetValue: "New"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewHTMLFormatter(&buf, false) // interactive mode
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify basic HTML structure
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(output, "staging") {
		t.Error("expected source name in output")
	}
	if !strings.Contains(output, "production") {
		t.Error("expected target name in output")
	}
	if !strings.Contains(output, "new-bp") {
		t.Error("expected added blueprint identifier")
	}
	if !strings.Contains(output, "existing-bp") {
		t.Error("expected modified blueprint identifier")
	}
}

func TestHTMLFormatter_SimpleTemplate(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Timestamp: "2026-02-05T19:30:00Z",
		Identical: false,
		Actions: ResourceDiff{
			Summary: DiffSummary{Added: 2, Modified: 0, Removed: 1},
		},
	}

	var buf bytes.Buffer
	formatter := NewHTMLFormatter(&buf, true) // simple mode
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify simple template structure
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(output, "Organization Comparison") {
		t.Error("expected title in output")
	}
}

func TestHTMLFormatter_IdenticalOrgs(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Timestamp: "2026-02-05T19:30:00Z",
		Identical: true,
	}

	var buf bytes.Buffer
	formatter := NewHTMLFormatter(&buf, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "identical") {
		t.Error("expected identical message in output")
	}
}

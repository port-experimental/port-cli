// Package compare provides functionality for comparing two Port organizations.
package compare

import (
	"bytes"
	"strings"
	"testing"
)

func TestTextFormatter_Summary(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 2, Modified: 1, Removed: 0},
		},
		Actions: ResourceDiff{
			Summary: DiffSummary{Added: 0, Modified: 0, Removed: 0},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "staging") {
		t.Error("expected output to contain source name")
	}
	if !strings.Contains(output, "2 added") {
		t.Error("expected output to contain '2 added'")
	}
}

func TestTextFormatter_Verbose(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1, Modified: 1, Removed: 1},
			Added: []ResourceChange{
				{Identifier: "new-bp", TargetData: map[string]interface{}{"title": "New"}},
			},
			Modified: []ResourceChange{
				{Identifier: "mod-bp", FieldDiffs: []FieldDiff{{Path: "title", SourceValue: "Old", TargetValue: "New"}}},
			},
			Removed: []ResourceChange{
				{Identifier: "old-bp", SourceData: map[string]interface{}{"title": "Old"}},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, true, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "new-bp") {
		t.Error("expected verbose output to contain added identifier")
	}
	if !strings.Contains(output, "mod-bp") {
		t.Error("expected verbose output to contain modified identifier")
	}
	if !strings.Contains(output, "old-bp") {
		t.Error("expected verbose output to contain removed identifier")
	}
}

func TestTextFormatter_Full(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 0, Modified: 1, Removed: 0},
			Modified: []ResourceChange{
				{
					Identifier: "mod-bp",
					FieldDiffs: []FieldDiff{
						{Path: "title", SourceValue: "Old Title", TargetValue: "New Title"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, true)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Old Title") {
		t.Error("expected full output to contain source value")
	}
	if !strings.Contains(output, "New Title") {
		t.Error("expected full output to contain target value")
	}
	if !strings.Contains(output, "title:") {
		t.Error("expected full output to contain field path")
	}
}

func TestTextFormatter_Identical(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Identical: true,
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "identical") {
		t.Error("expected output to indicate organizations are identical")
	}
}

func TestTextFormatter_Total(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1, Modified: 0, Removed: 0},
		},
		Actions: ResourceDiff{
			Summary: DiffSummary{Added: 2, Modified: 1, Removed: 0},
		},
		Teams: ResourceDiff{
			Summary: DiffSummary{Added: 0, Modified: 0, Removed: 3},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Total should be: 3 added, 1 modified, 3 removed
	if !strings.Contains(output, "3 added") {
		t.Errorf("expected total to show '3 added', got: %s", output)
	}
	if !strings.Contains(output, "1 modified") {
		t.Errorf("expected total to show '1 modified', got: %s", output)
	}
	if !strings.Contains(output, "3 removed") {
		t.Errorf("expected total to show '3 removed', got: %s", output)
	}
}

func TestTextFormatter_AllResourceTypes(t *testing.T) {
	result := &CompareResult{
		Source:       "staging",
		Target:       "production",
		Blueprints:   ResourceDiff{Summary: DiffSummary{Added: 1}},
		Actions:      ResourceDiff{Summary: DiffSummary{Modified: 1}},
		Scorecards:   ResourceDiff{Summary: DiffSummary{Removed: 1}},
		Pages:        ResourceDiff{Summary: DiffSummary{}},
		Integrations: ResourceDiff{Summary: DiffSummary{}},
		Teams:        ResourceDiff{Summary: DiffSummary{}},
		Users:        ResourceDiff{Summary: DiffSummary{}},
		Automations:  ResourceDiff{Summary: DiffSummary{}},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check that all resource types are mentioned
	resourceTypes := []string{"Blueprints", "Actions", "Scorecards", "Pages", "Integrations", "Teams", "Users", "Automations"}
	for _, rt := range resourceTypes {
		if !strings.Contains(output, rt) {
			t.Errorf("expected output to contain resource type %s", rt)
		}
	}
}

func TestTextFormatter_IdenticalResourceType(t *testing.T) {
	result := &CompareResult{
		Source: "staging",
		Target: "production",
		Actions: ResourceDiff{
			Summary: DiffSummary{Added: 0, Modified: 0, Removed: 0},
		},
	}

	var buf bytes.Buffer
	formatter := NewTextFormatter(&buf, false, false)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// When a resource type has no changes, it should say "identical"
	if !strings.Contains(output, "identical") {
		t.Error("expected identical resource types to be marked as 'identical'")
	}
}

func TestCalculateTotal(t *testing.T) {
	result := &CompareResult{
		Blueprints:   ResourceDiff{Summary: DiffSummary{Added: 1, Modified: 2, Removed: 3}},
		Actions:      ResourceDiff{Summary: DiffSummary{Added: 4, Modified: 5, Removed: 6}},
		Scorecards:   ResourceDiff{Summary: DiffSummary{Added: 0, Modified: 0, Removed: 0}},
		Pages:        ResourceDiff{Summary: DiffSummary{Added: 1, Modified: 0, Removed: 0}},
		Integrations: ResourceDiff{Summary: DiffSummary{Added: 0, Modified: 1, Removed: 0}},
		Teams:        ResourceDiff{Summary: DiffSummary{Added: 0, Modified: 0, Removed: 1}},
		Users:        ResourceDiff{Summary: DiffSummary{Added: 2, Modified: 0, Removed: 0}},
		Automations:  ResourceDiff{Summary: DiffSummary{Added: 0, Modified: 2, Removed: 0}},
	}

	formatter := NewTextFormatter(nil, false, false)
	total := formatter.calculateTotal(result)

	expectedAdded := 1 + 4 + 0 + 1 + 0 + 0 + 2 + 0    // = 8
	expectedModified := 2 + 5 + 0 + 0 + 1 + 0 + 0 + 2 // = 10
	expectedRemoved := 3 + 6 + 0 + 0 + 0 + 1 + 0 + 0  // = 10

	if total.Added != expectedAdded {
		t.Errorf("expected total added %d, got %d", expectedAdded, total.Added)
	}
	if total.Modified != expectedModified {
		t.Errorf("expected total modified %d, got %d", expectedModified, total.Modified)
	}
	if total.Removed != expectedRemoved {
		t.Errorf("expected total removed %d, got %d", expectedRemoved, total.Removed)
	}
}

func TestGetIdentifiers(t *testing.T) {
	changes := []ResourceChange{
		{Identifier: "bp1"},
		{Identifier: "bp2"},
		{Identifier: "bp3"},
	}

	formatter := NewTextFormatter(nil, false, false)
	ids := formatter.getIdentifiers(changes)

	if len(ids) != 3 {
		t.Fatalf("expected 3 identifiers, got %d", len(ids))
	}
	if ids[0] != "bp1" || ids[1] != "bp2" || ids[2] != "bp3" {
		t.Errorf("unexpected identifiers: %v", ids)
	}
}

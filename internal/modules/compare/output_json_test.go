package compare

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONFormatter(t *testing.T) {
	result := &CompareResult{
		Source:    "staging",
		Target:    "production",
		Timestamp: "2026-02-05T19:30:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1, Modified: 0, Removed: 0},
			Added: []ResourceChange{
				{Identifier: "new-bp"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["source"] != "staging" {
		t.Errorf("expected source 'staging', got %v", parsed["source"])
	}
}

func TestJSONFormatterIdentical(t *testing.T) {
	result := &CompareResult{
		Source:    "org-a",
		Target:    "org-b",
		Timestamp: "2026-02-05T20:00:00Z",
		Identical: true,
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["identical"] != true {
		t.Errorf("expected identical=true, got %v", parsed["identical"])
	}

	// Differences should be empty for identical orgs
	diffs, ok := parsed["differences"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected differences to be a map, got %T", parsed["differences"])
	}
	if len(diffs) != 0 {
		t.Errorf("expected no differences, got %d", len(diffs))
	}
}

func TestJSONFormatterAllResourceTypes(t *testing.T) {
	result := &CompareResult{
		Source:    "source-org",
		Target:    "target-org",
		Timestamp: "2026-02-05T21:00:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Added: 1},
			Added:   []ResourceChange{{Identifier: "bp-1"}},
		},
		Actions: ResourceDiff{
			Summary: DiffSummary{Modified: 1},
			Modified: []ResourceChange{{
				Identifier: "action-1",
				SourceData: map[string]interface{}{"title": "Old Title"},
				TargetData: map[string]interface{}{"title": "New Title"},
				FieldDiffs: []FieldDiff{{Path: "title", SourceValue: "Old Title", TargetValue: "New Title"}},
			}},
		},
		Scorecards: ResourceDiff{
			Summary: DiffSummary{Removed: 1},
			Removed: []ResourceChange{{Identifier: "sc-1", SourceData: map[string]interface{}{"name": "scorecard"}}},
		},
		Pages: ResourceDiff{
			Summary: DiffSummary{Added: 2},
			Added: []ResourceChange{
				{Identifier: "page-1"},
				{Identifier: "page-2"},
			},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check summary
	if parsed.Summary.TotalAdded != 3 {
		t.Errorf("expected TotalAdded=3, got %d", parsed.Summary.TotalAdded)
	}
	if parsed.Summary.TotalModified != 1 {
		t.Errorf("expected TotalModified=1, got %d", parsed.Summary.TotalModified)
	}
	if parsed.Summary.TotalRemoved != 1 {
		t.Errorf("expected TotalRemoved=1, got %d", parsed.Summary.TotalRemoved)
	}

	// Check blueprints
	bps, ok := parsed.Diffs["blueprints"]
	if !ok {
		t.Fatal("expected blueprints in diffs")
	}
	if len(bps.Added) != 1 || bps.Added[0].Identifier != "bp-1" {
		t.Errorf("unexpected blueprints added: %+v", bps.Added)
	}

	// Check actions (modified)
	actions, ok := parsed.Diffs["actions"]
	if !ok {
		t.Fatal("expected actions in diffs")
	}
	if len(actions.Modified) != 1 {
		t.Errorf("expected 1 modified action, got %d", len(actions.Modified))
	}
	if len(actions.Modified[0].Changes) != 1 {
		t.Errorf("expected 1 change for action, got %d", len(actions.Modified[0].Changes))
	}
	if actions.Modified[0].Changes[0].Path != "title" {
		t.Errorf("expected change path 'title', got '%s'", actions.Modified[0].Changes[0].Path)
	}

	// Check scorecards (removed)
	scorecards, ok := parsed.Diffs["scorecards"]
	if !ok {
		t.Fatal("expected scorecards in diffs")
	}
	if len(scorecards.Removed) != 1 {
		t.Errorf("expected 1 removed scorecard, got %d", len(scorecards.Removed))
	}

	// Check pages
	pages, ok := parsed.Diffs["pages"]
	if !ok {
		t.Fatal("expected pages in diffs")
	}
	if len(pages.Added) != 2 {
		t.Errorf("expected 2 added pages, got %d", len(pages.Added))
	}
}

func TestJSONFormatterFieldDiffs(t *testing.T) {
	result := &CompareResult{
		Source:    "src",
		Target:    "tgt",
		Timestamp: "2026-02-05T22:00:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary: DiffSummary{Modified: 1},
			Modified: []ResourceChange{{
				Identifier: "test-bp",
				SourceData: map[string]interface{}{
					"title":       "Old Title",
					"description": "Old Desc",
				},
				TargetData: map[string]interface{}{
					"title":       "New Title",
					"description": "New Desc",
				},
				FieldDiffs: []FieldDiff{
					{Path: "title", SourceValue: "Old Title", TargetValue: "New Title"},
					{Path: "description", SourceValue: "Old Desc", TargetValue: "New Desc"},
				},
			}},
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	bps := parsed.Diffs["blueprints"]
	if len(bps.Modified) != 1 {
		t.Fatalf("expected 1 modified blueprint")
	}

	modified := bps.Modified[0]
	if len(modified.Changes) != 2 {
		t.Errorf("expected 2 field changes, got %d", len(modified.Changes))
	}

	// Verify source and target data are included
	if modified.SourceData == nil {
		t.Error("expected SourceData to be populated")
	}
	if modified.TargetData == nil {
		t.Error("expected TargetData to be populated")
	}
}

func TestJSONFormatterEmptySlices(t *testing.T) {
	// Test that empty slices are omitted from JSON output
	result := &CompareResult{
		Source:    "src",
		Target:    "tgt",
		Timestamp: "2026-02-05T23:00:00Z",
		Identical: false,
		Blueprints: ResourceDiff{
			Summary:  DiffSummary{Added: 1},
			Added:    []ResourceChange{{Identifier: "bp-1"}},
			Modified: []ResourceChange{}, // empty
			Removed:  []ResourceChange{}, // empty
		},
	}

	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.Format(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check raw JSON to verify empty arrays are omitted
	var rawParsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &rawParsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	diffs := rawParsed["differences"].(map[string]interface{})
	bps := diffs["blueprints"].(map[string]interface{})

	// Added should exist
	if _, ok := bps["added"]; !ok {
		t.Error("expected 'added' field to exist")
	}

	// Modified and removed should be omitted (omitempty)
	if _, ok := bps["modified"]; ok {
		t.Error("expected 'modified' field to be omitted when empty")
	}
	if _, ok := bps["removed"]; ok {
		t.Error("expected 'removed' field to be omitted when empty")
	}
}

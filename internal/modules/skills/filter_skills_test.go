package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestFilterSkills(t *testing.T) {
	tests := []struct {
		name               string
		fetched            *FetchedSkills
		selectAll          bool
		selectAllGroups    bool
		selectAllUngrouped bool
		selectedGroups     []string
		selectedSkills     []string
		wantIDs            []string
	}{
		{
			name: "SelectAll includes everything",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-1", GroupIDs: []string{"group-a"}}, {Identifier: "opt-2"}},
			},
			selectAll: true,
			wantIDs:   []string{"req-1", "opt-1", "opt-2"},
		},
		{
			name: "required always included with no selection",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-1", GroupIDs: []string{"group-a"}}},
			},
			wantIDs: []string{"req-1"},
		},
		{
			name: "SelectAllGroups includes grouped only",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-grouped", GroupIDs: []string{"group-a"}}, {Identifier: "opt-ungrouped"}},
			},
			selectAllGroups: true,
			wantIDs:         []string{"req-1", "opt-grouped"},
		},
		{
			name: "SelectAllUngrouped includes ungrouped only",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "grouped", GroupIDs: []string{"group-a"}},
					{Identifier: "ungrouped-1"},
					{Identifier: "ungrouped-2"},
				},
			},
			selectAllUngrouped: true,
			wantIDs:            []string{"ungrouped-1", "ungrouped-2"},
		},
		{
			name: "specific groups",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "skill-a", GroupIDs: []string{"group-a"}},
					{Identifier: "skill-b", GroupIDs: []string{"group-b"}},
					{Identifier: "skill-c", GroupIDs: []string{"group-c"}},
				},
			},
			selectedGroups: []string{"group-a", "group-b"},
			wantIDs:        []string{"skill-a", "skill-b"},
		},
		{
			name: "specific skills",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "skill-1"},
					{Identifier: "skill-2"},
					{Identifier: "skill-3"},
				},
			},
			selectedSkills: []string{"skill-1", "skill-3"},
			wantIDs:        []string{"skill-1", "skill-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterSkills(tt.fetched, tt.selectAll, tt.selectAllGroups, tt.selectAllUngrouped, tt.selectedGroups, tt.selectedSkills)
			ids := identifiers(result)
			if len(ids) != len(tt.wantIDs) {
				t.Fatalf("want %d skills (%v), got %d (%v)", len(tt.wantIDs), tt.wantIDs, len(ids), ids)
			}
			for _, want := range tt.wantIDs {
				if !contains(ids, want) {
					t.Errorf("expected %q in result %v", want, ids)
				}
			}
		})
	}
}

func TestParseFetchedSkills_GroupRelationAndRequired(t *testing.T) {
	groupEntities := []api.Entity{
		{
			"identifier": "group-required",
			"title":      "Required Group",
			"properties": map[string]interface{}{"enforcement": "required"},
			"relations":  map[string]interface{}{"skills": []interface{}{"skill-a", "skill-b"}},
		},
		{
			"identifier": "group-optional",
			"title":      "Optional Group",
			"properties": map[string]interface{}{"enforcement": "optional"},
			"relations":  map[string]interface{}{"skills": []interface{}{"skill-c"}},
		},
	}
	skillEntities := []api.Entity{
		{"identifier": "skill-a", "title": "A", "properties": map[string]interface{}{"instructions": "do a"}},
		{"identifier": "skill-b", "title": "B", "properties": map[string]interface{}{"instructions": "do b"}},
		{"identifier": "skill-c", "title": "C", "properties": map[string]interface{}{"instructions": "do c"}},
	}
	fetched := ParseFetchedSkills(groupEntities, skillEntities)

	if len(fetched.Required) != 2 {
		t.Errorf("want 2 required, got %d", len(fetched.Required))
	}
	if len(fetched.Optional) != 1 || fetched.Optional[0].Identifier != "skill-c" {
		t.Errorf("want 1 optional (skill-c), got %v", fetched.Optional)
	}
	for _, s := range fetched.Required {
		if !contains(s.GroupIDs, "group-required") {
			t.Errorf("expected GroupIDs to contain group-required for %s, got %v", s.Identifier, s.GroupIDs)
		}
	}
}

func TestParseFetchedSkills_UngroupedAndFiles(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "skill-with-files",
			"title":      "Skill With Files",
			"properties": map[string]interface{}{
				"instructions":     "do it",
				"references":       []interface{}{map[string]interface{}{"path": "refs/guide.md", "content": "# Guide"}},
				"assets":           []interface{}{map[string]interface{}{"path": "assets/tpl.yaml", "content": "key: value"}},
				"scripts":          []interface{}{map[string]interface{}{"path": "scripts/run.sh", "content": "#!/bin/sh\n"}},
				"additional_files": []interface{}{map[string]interface{}{"path": "LICENSE", "content": "MIT"}},
			},
		},
	}
	fetched := ParseFetchedSkills(nil, skillEntities)
	s := fetched.Optional[0]
	if len(s.GroupIDs) != 0 {
		t.Errorf("expected empty GroupIDs, got %v", s.GroupIDs)
	}
	if len(s.References) != 1 || s.References[0].Path != "refs/guide.md" {
		t.Errorf("unexpected references: %+v", s.References)
	}
	if len(s.Assets) != 1 || s.Assets[0].Path != "assets/tpl.yaml" {
		t.Errorf("unexpected assets: %+v", s.Assets)
	}
	if len(s.Scripts) != 1 || s.Scripts[0].Path != "scripts/run.sh" {
		t.Errorf("unexpected scripts: %+v", s.Scripts)
	}
	if len(s.AdditionalFiles) != 1 || s.AdditionalFiles[0].Path != "LICENSE" {
		t.Errorf("unexpected additional_files: %+v", s.AdditionalFiles)
	}
}

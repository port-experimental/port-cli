package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestParseSkillLocation(t *testing.T) {
	tests := []struct {
		input string
		want  SkillLocation
	}{
		{"project", SkillLocationProject},
		{"global", SkillLocationGlobal},
		{"", SkillLocationGlobal},
		{"other", SkillLocationGlobal},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseSkillLocation(tt.input); got != tt.want {
				t.Errorf("parseSkillLocation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildSkillMD(t *testing.T) {
	t.Run("with instructions", func(t *testing.T) {
		md := buildSkillMD(Skill{Identifier: "s", Description: "desc", Instructions: "step 1\nstep 2"})
		for _, want := range []string{"name: s", "description: desc", "step 1"} {
			if !containsStr(md, want) {
				t.Errorf("missing %q in output", want)
			}
		}
	})
	t.Run("no instructions fallback", func(t *testing.T) {
		md := buildSkillMD(Skill{Identifier: "empty", Title: "Empty"})
		if !containsStr(md, "_No instructions provided._") {
			t.Error("expected fallback text")
		}
	})
}

func TestGroupName(t *testing.T) {
	groups := []SkillGroup{
		{Identifier: "grp-1", Title: "My Group"},
		{Identifier: "grp-2", Title: ""},
	}
	tests := []struct{ groupID, want string }{
		{"grp-1", "My Group"},
		{"grp-2", "grp-2"},
		{"unknown", "unknown"},
		{"", NoGroupDir},
	}
	for _, tt := range tests {
		t.Run(tt.groupID, func(t *testing.T) {
			if got := GroupName(groups, tt.groupID); got != tt.want {
				t.Errorf("GroupName(%q) = %q, want %q", tt.groupID, got, tt.want)
			}
		})
	}
}

func TestValidatePathComponent(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"my-skill", false},
		{"..", true},
		{".", true},
		{"a/b", true},
		{"a\\b", true},
		{"my.skill.v2", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validatePathComponent(tt.input)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseFetchedSkills_MultiGroupSkill(t *testing.T) {
	groupA := api.Entity{
		"identifier": "group-a",
		"title":      "Group A",
		"properties": map[string]interface{}{"enforcement": "optional"},
		"relations":  map[string]interface{}{"skills": []interface{}{"shared-skill", "only-a"}},
	}
	groupB := api.Entity{
		"identifier": "group-b",
		"title":      "Group B",
		"properties": map[string]interface{}{"enforcement": "optional"},
		"relations":  map[string]interface{}{"skills": []interface{}{"shared-skill"}},
	}
	skillEntities := []api.Entity{
		{"identifier": "shared-skill", "title": "Shared", "properties": map[string]interface{}{}},
		{"identifier": "only-a", "title": "Only A", "properties": map[string]interface{}{}},
	}

	result := ParseFetchedSkills([]api.Entity{groupA, groupB}, skillEntities)

	skillByID := make(map[string]Skill)
	for _, s := range result.Optional {
		skillByID[s.Identifier] = s
	}

	shared, ok := skillByID["shared-skill"]
	if !ok {
		t.Fatal("shared-skill not found in Optional")
	}
	if len(shared.GroupIDs) != 2 {
		t.Fatalf("expected shared-skill to have 2 GroupIDs, got %v", shared.GroupIDs)
	}
	if !contains(shared.GroupIDs, "group-a") || !contains(shared.GroupIDs, "group-b") {
		t.Errorf("expected GroupIDs to contain both groups, got %v", shared.GroupIDs)
	}

	onlyA, ok := skillByID["only-a"]
	if !ok {
		t.Fatal("only-a not found in Optional")
	}
	if len(onlyA.GroupIDs) != 1 || onlyA.GroupIDs[0] != "group-a" {
		t.Errorf("expected only-a to have GroupIDs=[group-a], got %v", onlyA.GroupIDs)
	}
}

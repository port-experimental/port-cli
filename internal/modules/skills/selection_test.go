package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestApplySelectionToConfig_ReplaceSelection(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectAll:          true,
		SelectAllGroups:    true,
		SelectAllUngrouped: true,
		SelectedGroups:     []string{"old-group"},
		SelectedSkills:     []string{"old-skill"},
	}

	applySelectionToConfig(cfg, LoadSkillsOptions{
		ReplaceSelection:   true,
		SelectedGroups:     []string{"new-group"},
		SelectedSkills:     []string{"new-skill"},
		SelectAll:          false,
		SelectAllGroups:    false,
		SelectAllUngrouped: false,
	})

	if cfg.SelectAll || cfg.SelectAllGroups || cfg.SelectAllUngrouped {
		t.Fatalf("expected select-all flags cleared, got %+v", cfg)
	}
	if len(cfg.SelectedGroups) != 1 || cfg.SelectedGroups[0] != "new-group" {
		t.Fatalf("SelectedGroups: %v", cfg.SelectedGroups)
	}
	if len(cfg.SelectedSkills) != 1 || cfg.SelectedSkills[0] != "new-skill" {
		t.Fatalf("SelectedSkills: %v", cfg.SelectedSkills)
	}
}

func TestMergeSelection_AddsGroupsAndSkills(t *testing.T) {
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-b"},
		},
		Optional: []Skill{
			{Identifier: "skill-1"},
			{Identifier: "skill-2", GroupIDs: []string{"group-a"}},
		},
	}
	cfg := &config.SkillsConfig{
		SelectedGroups: []string{"group-a"},
		SelectedSkills: []string{"skill-1"},
	}

	result, err := MergeSelection(cfg, fetched, []string{"group-b"}, []string{"skill-1"})
	if err != nil {
		t.Fatalf("MergeSelection: %v", err)
	}
	if !result.HasChanges() {
		t.Fatal("expected changes")
	}
	if len(result.AddedGroups) != 1 || result.AddedGroups[0] != "group-b" {
		t.Errorf("AddedGroups: %v", result.AddedGroups)
	}
	if len(result.AddedSkills) != 0 {
		t.Errorf("AddedSkills: %v", result.AddedSkills)
	}
	if len(result.SkippedSkills) != 1 || result.SkippedSkills[0] != "skill-1" {
		t.Errorf("expected skill-1 skipped (already selected), got skips=%v", result.SkippedSkills)
	}
	if !contains(cfg.SelectedGroups, "group-b") {
		t.Errorf("config groups: %v", cfg.SelectedGroups)
	}
}

func TestMergeSelection_SkipsAlreadySelected(t *testing.T) {
	fetched := &FetchedSkills{
		Groups:   []SkillGroup{{Identifier: "group-a"}},
		Optional: []Skill{{Identifier: "skill-1"}},
	}
	cfg := &config.SkillsConfig{
		SelectAllGroups:    true,
		SelectAllUngrouped: true,
	}

	result, err := MergeSelection(cfg, fetched, []string{"group-a"}, []string{"skill-1"})
	if err != nil {
		t.Fatalf("MergeSelection: %v", err)
	}
	if result.HasChanges() {
		t.Fatal("expected no changes")
	}
	if len(result.SkippedGroups) != 1 || len(result.SkippedSkills) != 1 {
		t.Errorf("skips: groups=%v skills=%v", result.SkippedGroups, result.SkippedSkills)
	}
}

func TestMergeSelection_UnknownIdentifiers(t *testing.T) {
	cfg := &config.SkillsConfig{}
	_, err := MergeSelection(cfg, &FetchedSkills{}, []string{"missing-group"}, []string{"missing-skill"})
	if err == nil {
		t.Fatal("expected error for unknown identifiers")
	}
}

func TestAvailableGroupsToAdd(t *testing.T) {
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "a"},
			{Identifier: "b"},
			{Identifier: "req", Required: true},
		},
	}
	cfg := &config.SkillsConfig{SelectedGroups: []string{"a"}}

	got := AvailableGroupsToAdd(cfg, fetched)
	if len(got) != 1 || got[0].Identifier != "b" {
		t.Errorf("AvailableGroupsToAdd: got %v", got)
	}
}

func TestAvailableUngroupedSkillsToAdd(t *testing.T) {
	fetched := &FetchedSkills{
		Optional: []Skill{
			{Identifier: "u1"},
			{Identifier: "u2"},
			{Identifier: "g1", GroupIDs: []string{"grp"}},
		},
	}
	cfg := &config.SkillsConfig{SelectedSkills: []string{"u1"}}

	got := AvailableUngroupedSkillsToAdd(cfg, fetched)
	if len(got) != 1 || got[0].Identifier != "u2" {
		t.Errorf("AvailableUngroupedSkillsToAdd: got %v", got)
	}
}

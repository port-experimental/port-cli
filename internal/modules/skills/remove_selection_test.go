package skills

import (
	"sort"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = sortedCopy(a)
	b = sortedCopy(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRemoveSelection_DropsExplicitGroup(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectedGroups: []string{"group-a", "group-b"},
	}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-b"},
		},
	}

	result, err := RemoveSelection(cfg, fetched, []string{"group-a"}, nil)
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if !equalStrings(result.RemovedGroups, []string{"group-a"}) {
		t.Errorf("RemovedGroups: got %v", result.RemovedGroups)
	}
	if !equalStrings(cfg.SelectedGroups, []string{"group-b"}) {
		t.Errorf("SelectedGroups after remove: got %v", cfg.SelectedGroups)
	}
	if result.Materialized {
		t.Error("Materialized should be false when no SelectAll* was set")
	}
}

func TestRemoveSelection_DropsExplicitSkill(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectedSkills: []string{"skill-x", "skill-y"},
	}
	fetched := &FetchedSkills{
		Optional: []Skill{{Identifier: "skill-x"}, {Identifier: "skill-y"}},
	}

	result, err := RemoveSelection(cfg, fetched, nil, []string{"skill-x"})
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if !equalStrings(result.RemovedSkills, []string{"skill-x"}) {
		t.Errorf("RemovedSkills: got %v", result.RemovedSkills)
	}
	if !equalStrings(cfg.SelectedSkills, []string{"skill-y"}) {
		t.Errorf("SelectedSkills after remove: got %v", cfg.SelectedSkills)
	}
}

func TestRemoveSelection_MaterializesSelectAllGroups(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectAllGroups: true,
	}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-b"},
			{Identifier: "group-c"},
			{Identifier: "group-req", Required: true},
		},
	}

	result, err := RemoveSelection(cfg, fetched, []string{"group-a"}, nil)
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if !result.Materialized {
		t.Fatal("expected Materialized=true after expanding SelectAllGroups")
	}
	if cfg.SelectAllGroups {
		t.Error("SelectAllGroups should be cleared after materialization")
	}
	if !equalStrings(cfg.SelectedGroups, []string{"group-b", "group-c"}) {
		t.Errorf("SelectedGroups after materialize+remove: got %v", cfg.SelectedGroups)
	}
	if !equalStrings(result.RemovedGroups, []string{"group-a"}) {
		t.Errorf("RemovedGroups: got %v", result.RemovedGroups)
	}
}

func TestRemoveSelection_MaterializesSelectAll(t *testing.T) {
	cfg := &config.SkillsConfig{SelectAll: true}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "group-a"}, {Identifier: "group-b"}},
		Optional: []Skill{
			{Identifier: "ungrouped-1"},
			{Identifier: "ungrouped-2"},
			{Identifier: "grouped", GroupIDs: []string{"group-a"}},
		},
	}

	result, err := RemoveSelection(cfg, fetched, []string{"group-a"}, nil)
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if !result.Materialized {
		t.Fatal("expected Materialized=true")
	}
	if cfg.SelectAll || cfg.SelectAllGroups || cfg.SelectAllUngrouped {
		t.Errorf("SelectAll flags should be cleared: %+v", cfg)
	}
	if !equalStrings(cfg.SelectedGroups, []string{"group-b"}) {
		t.Errorf("SelectedGroups: got %v", cfg.SelectedGroups)
	}
	if !equalStrings(cfg.SelectedSkills, []string{"ungrouped-1", "ungrouped-2"}) {
		t.Errorf("SelectedSkills: got %v", cfg.SelectedSkills)
	}
}

func TestRemoveSelection_RequiredSkippedNotRemoved(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectedGroups: []string{"group-a"},
	}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-req", Required: true},
		},
	}

	result, err := RemoveSelection(cfg, fetched, []string{"group-req"}, nil)
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if !equalStrings(result.SkippedGroups, []string{"group-req"}) {
		t.Errorf("SkippedGroups: got %v", result.SkippedGroups)
	}
	if len(result.RemovedGroups) != 0 {
		t.Errorf("expected no removed groups, got %v", result.RemovedGroups)
	}
	if result.Materialized {
		t.Error("nothing actionable; materialization should not happen")
	}
}

func TestRemoveSelection_UnknownIDErrors(t *testing.T) {
	cfg := &config.SkillsConfig{SelectedGroups: []string{"group-a"}}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "group-a"}},
	}

	_, err := RemoveSelection(cfg, fetched, []string{"not-a-group"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown selection")
	}
}

func TestRemoveSelection_NotInSelectionSkipped(t *testing.T) {
	cfg := &config.SkillsConfig{SelectedGroups: []string{"group-a"}}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-b"},
		},
	}

	result, err := RemoveSelection(cfg, fetched, []string{"group-b"}, nil)
	if err != nil {
		t.Fatalf("RemoveSelection: %v", err)
	}
	if len(result.RemovedGroups) != 0 {
		t.Errorf("expected no removed groups, got %v", result.RemovedGroups)
	}
	if !equalStrings(result.SkippedGroups, []string{"group-b"}) {
		t.Errorf("SkippedGroups: got %v", result.SkippedGroups)
	}
}

func TestRemovableGroups_ExpandsSelectAll(t *testing.T) {
	cfg := &config.SkillsConfig{SelectAllGroups: true}
	fetched := &FetchedSkills{
		Groups: []SkillGroup{
			{Identifier: "group-a"},
			{Identifier: "group-b"},
			{Identifier: "group-req", Required: true},
		},
	}

	got := RemovableGroups(cfg, fetched)
	if len(got) != 2 {
		t.Fatalf("expected 2 removable groups, got %d", len(got))
	}
	ids := []string{got[0].Identifier, got[1].Identifier}
	if !equalStrings(ids, []string{"group-a", "group-b"}) {
		t.Errorf("RemovableGroups: got %v", ids)
	}
}

func TestRemovableSkills_ExplicitAndSelectAllUngrouped(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectAllUngrouped: true,
		SelectedSkills:     []string{"grouped-explicit"},
	}
	fetched := &FetchedSkills{
		Optional: []Skill{
			{Identifier: "ungrouped-1"},
			{Identifier: "ungrouped-2"},
			{Identifier: "grouped-explicit", GroupIDs: []string{"group-a"}},
			{Identifier: "grouped-via-group", GroupIDs: []string{"group-a"}},
		},
	}

	got := RemovableSkills(cfg, fetched)
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.Identifier)
	}
	expected := []string{"ungrouped-1", "ungrouped-2", "grouped-explicit"}
	if !equalStrings(ids, expected) {
		t.Errorf("RemovableSkills: got %v, want %v", ids, expected)
	}
}

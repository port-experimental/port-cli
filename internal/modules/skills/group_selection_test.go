package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/config"
)

func TestGroupSelectionFromCatalog(t *testing.T) {
	groups := []aiservice.SkillGroupCatalogEntry{
		{Identifier: "g-team", MatchesUserTeams: true},
		{Identifier: "g-other", MatchesUserTeams: false},
		{Identifier: "g-team2", MatchesUserTeams: true},
	}
	include, exclude := GroupSelectionFromCatalog(groups, []string{"g-team", "g-other"})
	if len(include) != 1 || include[0] != "g-other" {
		t.Fatalf("include: %v", include)
	}
	if len(exclude) != 1 || exclude[0] != "g-team2" {
		t.Fatalf("exclude: %v", exclude)
	}
}

func TestInitialSelectedGroupIDs_TeamIncludeExclude(t *testing.T) {
	groups := []aiservice.SkillGroupCatalogEntry{
		{Identifier: "demo-engineering-optional", MatchesUserTeams: true},
		{Identifier: "demo-engineering-required", MatchesUserTeams: false},
		{Identifier: "demo-security-manual", MatchesUserTeams: false},
	}
	cfg := &config.SkillsConfig{
		TeamGroupDefaults: true,
		IncludeGroups:     []string{"demo-security-manual"},
		ExcludeGroups:     []string{"demo-engineering-optional"},
		Targets:           []string{"/tmp/.cursor"},
	}
	got := InitialSelectedGroupIDs(groups, cfg)
	want := []string{"demo-security-manual"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestInitialUngroupedSelection(t *testing.T) {
	cfg := &config.SkillsConfig{
		SelectAllUngrouped: false,
		SelectedSkills:     []string{"demo-standalone"},
	}
	all, ids := InitialUngroupedSelection(cfg)
	if all || len(ids) != 1 || ids[0] != "demo-standalone" {
		t.Fatalf("got all=%v ids=%v", all, ids)
	}
}

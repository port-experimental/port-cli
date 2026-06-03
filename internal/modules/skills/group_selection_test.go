package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
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

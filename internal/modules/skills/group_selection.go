package skills

import "github.com/port-experimental/port-cli/internal/api/aiservice"

// GroupSelectionFromCatalog computes include/exclude deltas vs team-owned groups.
func GroupSelectionFromCatalog(groups []aiservice.SkillGroupCatalogEntry, selected []string) (include, exclude []string) {
	teamBaseline := make(map[string]bool)
	for _, g := range groups {
		if g.MatchesUserTeams {
			teamBaseline[g.Identifier] = true
		}
	}

	selectedSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		selectedSet[id] = true
	}

	for id := range selectedSet {
		if !teamBaseline[id] {
			include = append(include, id)
		}
	}
	for id := range teamBaseline {
		if !selectedSet[id] {
			exclude = append(exclude, id)
		}
	}
	return include, exclude
}

// PreselectedGroupIDs returns group identifiers that match the user's teams.
func PreselectedGroupIDs(groups []aiservice.SkillGroupCatalogEntry) []string {
	var out []string
	for _, g := range groups {
		if g.MatchesUserTeams {
			out = append(out, g.Identifier)
		}
	}
	return out
}

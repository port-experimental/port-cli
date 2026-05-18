package skills

import (
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
)

// MergeSelectionResult reports what was merged into the skills config.
type MergeSelectionResult struct {
	AddedGroups   []string
	AddedSkills   []string
	SkippedGroups []string
	SkippedSkills []string
}

// HasChanges reports whether any new groups or skills were added.
func (r MergeSelectionResult) HasChanges() bool {
	return len(r.AddedGroups) > 0 || len(r.AddedSkills) > 0
}

// MergeSelection appends group and skill identifiers to cfg without replacing
// the existing selection. Unknown identifiers return an error. Items already
// covered by the current selection are listed in Skipped*.
func MergeSelection(cfg *config.SkillsConfig, fetched *FetchedSkills, addGroups, addSkills []string) (MergeSelectionResult, error) {
	groupSet := make(map[string]SkillGroup, len(fetched.Groups))
	for _, g := range fetched.Groups {
		if g.Required {
			continue
		}
		groupSet[g.Identifier] = g
	}

	skillByID := make(map[string]Skill, len(fetched.Optional))
	for _, s := range fetched.Optional {
		skillByID[s.Identifier] = s
	}

	var result MergeSelectionResult
	var invalid []string

	for _, id := range addGroups {
		if _, ok := groupSet[id]; !ok {
			invalid = append(invalid, "group:"+id)
			continue
		}
		if isGroupSelected(cfg, id) {
			result.SkippedGroups = append(result.SkippedGroups, id)
			continue
		}
		cfg.SelectedGroups = appendUniqueString(cfg.SelectedGroups, id)
		result.AddedGroups = append(result.AddedGroups, id)
	}

	for _, id := range addSkills {
		s, ok := skillByID[id]
		if !ok {
			invalid = append(invalid, "skill:"+id)
			continue
		}
		if isSkillSelected(cfg, s) {
			result.SkippedSkills = append(result.SkippedSkills, id)
			continue
		}
		cfg.SelectedSkills = appendUniqueString(cfg.SelectedSkills, id)
		result.AddedSkills = append(result.AddedSkills, id)
	}

	if len(invalid) > 0 {
		return result, fmt.Errorf("unknown selection: %s", strings.Join(invalid, ", "))
	}
	return result, nil
}

func isGroupSelected(cfg *config.SkillsConfig, groupID string) bool {
	if cfg.SelectAll || cfg.SelectAllGroups {
		return true
	}
	for _, g := range cfg.SelectedGroups {
		if g == groupID {
			return true
		}
	}
	return false
}

func isSkillSelected(cfg *config.SkillsConfig, skill Skill) bool {
	if skill.Required || cfg.SelectAll {
		return true
	}
	if skill.GroupID == "" {
		if cfg.SelectAllUngrouped {
			return true
		}
		for _, id := range cfg.SelectedSkills {
			if id == skill.Identifier {
				return true
			}
		}
		return false
	}
	if cfg.SelectAllGroups {
		return true
	}
	for _, g := range cfg.SelectedGroups {
		if g == skill.GroupID {
			return true
		}
	}
	for _, id := range cfg.SelectedSkills {
		if id == skill.Identifier {
			return true
		}
	}
	return false
}

func appendUniqueString(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// AvailableGroupsToAdd returns optional groups not yet in the user's selection.
func AvailableGroupsToAdd(cfg *config.SkillsConfig, fetched *FetchedSkills) []SkillGroup {
	if cfg.SelectAll || cfg.SelectAllGroups {
		return nil
	}
	var out []SkillGroup
	for _, g := range fetched.Groups {
		if g.Required {
			continue
		}
		if !isGroupSelected(cfg, g.Identifier) {
			out = append(out, g)
		}
	}
	return out
}

// AvailableUngroupedSkillsToAdd returns optional ungrouped skills not yet selected.
func AvailableUngroupedSkillsToAdd(cfg *config.SkillsConfig, fetched *FetchedSkills) []Skill {
	if cfg.SelectAll || cfg.SelectAllUngrouped {
		return nil
	}
	var out []Skill
	for _, s := range fetched.Optional {
		if s.GroupID != "" {
			continue
		}
		if !isSkillSelected(cfg, s) {
			out = append(out, s)
		}
	}
	return out
}

// AvailableSkillsToAdd returns optional skills not yet covered by the selection.
func AvailableSkillsToAdd(cfg *config.SkillsConfig, fetched *FetchedSkills) []Skill {
	if cfg.SelectAll {
		return nil
	}
	var out []Skill
	for _, s := range fetched.Optional {
		if !isSkillSelected(cfg, s) {
			out = append(out, s)
		}
	}
	return out
}

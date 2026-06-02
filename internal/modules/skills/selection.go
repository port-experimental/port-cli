package skills

import (
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/config"
)

func applySelectionToConfig(cfg *config.SkillsConfig, opts LoadSkillsOptions) {
	if opts.ReplaceSelection {
		cfg.SelectAll = opts.SelectAll
		cfg.SelectAllGroups = opts.SelectAllGroups
		cfg.SelectAllUngrouped = opts.SelectAllUngrouped
		cfg.SelectedGroups = append([]string(nil), opts.SelectedGroups...)
		cfg.SelectedSkills = append([]string(nil), opts.SelectedSkills...)
		return
	}

	if opts.SelectAll || opts.SelectAllGroups || opts.SelectAllUngrouped ||
		len(opts.SelectedGroups) > 0 || len(opts.SelectedSkills) > 0 {
		cfg.SelectAll = opts.SelectAll
		cfg.SelectAllGroups = opts.SelectAllGroups
		cfg.SelectAllUngrouped = opts.SelectAllUngrouped
		cfg.SelectedGroups = opts.SelectedGroups
		cfg.SelectedSkills = opts.SelectedSkills
	}
}

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
	if len(skill.GroupIDs) == 0 {
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
		for _, gid := range skill.GroupIDs {
			if g == gid {
				return true
			}
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
		if len(s.GroupIDs) > 0 {
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

// RemoveSelectionResult reports what was removed from the skills config.
type RemoveSelectionResult struct {
	RemovedGroups []string
	RemovedSkills []string
	SkippedGroups []string
	SkippedSkills []string
	// Materialized is true when a SelectAll* flag was expanded into explicit
	// lists to enable removal. Callers should surface this so users know
	// future Port-side additions will no longer auto-sync.
	Materialized bool
}

// HasChanges reports whether any group or skill was removed.
func (r RemoveSelectionResult) HasChanges() bool {
	return len(r.RemovedGroups) > 0 || len(r.RemovedSkills) > 0
}

// RemoveSelection drops group and skill identifiers from cfg. Required items
// and items not currently in the selection are reported in Skipped*. Unknown
// identifiers return an error. If cfg uses any SelectAll* flag, the selection
// is materialized into explicit lists first so individual items can be removed.
func RemoveSelection(cfg *config.SkillsConfig, fetched *FetchedSkills, removeGroups, removeSkills []string) (RemoveSelectionResult, error) {
	groupByID := make(map[string]SkillGroup, len(fetched.Groups))
	for _, g := range fetched.Groups {
		groupByID[g.Identifier] = g
	}
	skillByID := make(map[string]Skill, len(fetched.Optional)+len(fetched.Required))
	for _, s := range fetched.Optional {
		skillByID[s.Identifier] = s
	}
	for _, s := range fetched.Required {
		skillByID[s.Identifier] = s
	}

	var invalid []string
	for _, id := range removeGroups {
		if _, ok := groupByID[id]; !ok {
			invalid = append(invalid, "group:"+id)
		}
	}
	for _, id := range removeSkills {
		if _, ok := skillByID[id]; !ok {
			invalid = append(invalid, "skill:"+id)
		}
	}
	if len(invalid) > 0 {
		return RemoveSelectionResult{}, fmt.Errorf("unknown selection: %s", strings.Join(invalid, ", "))
	}

	var result RemoveSelectionResult

	// Filter out required items — they cannot be removed.
	actionableGroups := make([]string, 0, len(removeGroups))
	for _, id := range removeGroups {
		if groupByID[id].Required {
			result.SkippedGroups = append(result.SkippedGroups, id)
			continue
		}
		actionableGroups = append(actionableGroups, id)
	}
	actionableSkills := make([]string, 0, len(removeSkills))
	for _, id := range removeSkills {
		if skillByID[id].Required {
			result.SkippedSkills = append(result.SkippedSkills, id)
			continue
		}
		actionableSkills = append(actionableSkills, id)
	}

	if len(actionableGroups) == 0 && len(actionableSkills) == 0 {
		return result, nil
	}

	result.Materialized = materializeSelection(cfg, fetched)

	for _, id := range actionableGroups {
		before := len(cfg.SelectedGroups)
		cfg.SelectedGroups = removeStringFromSlice(cfg.SelectedGroups, id)
		if len(cfg.SelectedGroups) < before {
			result.RemovedGroups = append(result.RemovedGroups, id)
		} else {
			result.SkippedGroups = append(result.SkippedGroups, id)
		}
	}

	for _, id := range actionableSkills {
		before := len(cfg.SelectedSkills)
		cfg.SelectedSkills = removeStringFromSlice(cfg.SelectedSkills, id)
		if len(cfg.SelectedSkills) < before {
			result.RemovedSkills = append(result.RemovedSkills, id)
		} else {
			result.SkippedSkills = append(result.SkippedSkills, id)
		}
	}

	return result, nil
}

// materializeSelection expands any SelectAll* flags on cfg into explicit
// SelectedGroups / SelectedSkills lists. Returns true if any flag was expanded.
func materializeSelection(cfg *config.SkillsConfig, fetched *FetchedSkills) bool {
	changed := false
	if cfg.SelectAll {
		cfg.SelectAll = false
		cfg.SelectAllGroups = true
		cfg.SelectAllUngrouped = true
		changed = true
	}
	if cfg.SelectAllGroups {
		for _, g := range fetched.Groups {
			if g.Required {
				continue
			}
			cfg.SelectedGroups = appendUniqueString(cfg.SelectedGroups, g.Identifier)
		}
		cfg.SelectAllGroups = false
		changed = true
	}
	if cfg.SelectAllUngrouped {
		for _, s := range fetched.Optional {
			if len(s.GroupIDs) > 0 {
				continue
			}
			cfg.SelectedSkills = appendUniqueString(cfg.SelectedSkills, s.Identifier)
		}
		cfg.SelectAllUngrouped = false
		changed = true
	}
	return changed
}

func removeStringFromSlice(slice []string, target string) []string {
	out := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}

// RemovableGroups returns optional groups currently in the user's selection,
// virtually expanding SelectAll* coverage so users can remove any group that
// is in effect.
func RemovableGroups(cfg *config.SkillsConfig, fetched *FetchedSkills) []SkillGroup {
	var out []SkillGroup
	if cfg.SelectAll || cfg.SelectAllGroups {
		for _, g := range fetched.Groups {
			if g.Required {
				continue
			}
			out = append(out, g)
		}
		return out
	}
	selected := make(map[string]bool, len(cfg.SelectedGroups))
	for _, id := range cfg.SelectedGroups {
		selected[id] = true
	}
	for _, g := range fetched.Groups {
		if g.Required {
			continue
		}
		if selected[g.Identifier] {
			out = append(out, g)
		}
	}
	return out
}

// RemovableSkills returns optional skills currently in the user's explicit
// selection (cfg.SelectedSkills), plus any ungrouped optional skills covered
// by SelectAll / SelectAllUngrouped. Skills selected only via their group
// cannot be removed individually — the group must be removed instead.
func RemovableSkills(cfg *config.SkillsConfig, fetched *FetchedSkills) []Skill {
	seen := make(map[string]bool)
	var out []Skill

	if cfg.SelectAll || cfg.SelectAllUngrouped {
		for _, s := range fetched.Optional {
			if len(s.GroupIDs) > 0 {
				continue
			}
			if !seen[s.Identifier] {
				seen[s.Identifier] = true
				out = append(out, s)
			}
		}
	}

	explicit := make(map[string]bool, len(cfg.SelectedSkills))
	for _, id := range cfg.SelectedSkills {
		explicit[id] = true
	}
	for _, s := range fetched.Optional {
		if explicit[s.Identifier] && !seen[s.Identifier] {
			seen[s.Identifier] = true
			out = append(out, s)
		}
	}
	return out
}

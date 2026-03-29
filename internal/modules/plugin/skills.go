package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// SkillFile represents a reference or asset file attached to a skill.
type SkillFile struct {
	Path    string
	Content string
}

// SkillLocation controls where a skill is written on disk.
type SkillLocation string

const (
	SkillLocationGlobal  SkillLocation = "global"
	SkillLocationProject SkillLocation = "project"
)

// Skill holds the data for a single skill entity fetched from Port.
type Skill struct {
	Identifier   string
	Title        string
	Description  string
	Instructions string
	GroupID      string
	Required     bool
	Location     SkillLocation
	References   []SkillFile
	Assets       []SkillFile
}

// SkillGroup holds the data for a single skill_group entity fetched from Port.
type SkillGroup struct {
	Identifier string
	Title      string
	Required   bool
	SkillIDs   []string
}

// FetchedSkills contains skills split by whether they are required.
type FetchedSkills struct {
	Required []Skill
	Optional []Skill
	Groups   []SkillGroup
}

// FetchSkills retrieves all skill groups and skills from the Port API and
// partitions them into required vs optional.
func FetchSkills(ctx context.Context, client *api.Client) (*FetchedSkills, error) {
	groupEntities, err := client.GetSkillGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill groups: %w", err)
	}

	skillEntities, err := client.GetSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills: %w", err)
	}

	return ParseFetchedSkills(groupEntities, skillEntities), nil
}

// ParseFetchedSkills builds a FetchedSkills from raw API entities.
// Exported so tests can exercise parsing without hitting the network.
func ParseFetchedSkills(groupEntities, skillEntities []api.Entity) *FetchedSkills {
	groups := make([]SkillGroup, 0, len(groupEntities))
	requiredSkillIDs := make(map[string]bool)
	skillGroupMap := make(map[string]string)

	for _, e := range groupEntities {
		props, _ := e["properties"].(map[string]interface{})
		relations, _ := e["relations"].(map[string]interface{})

		groupID := stringProp(e, "identifier")
		enforcement := stringFromMap(props, "enforcement")
		isRequired := enforcement == "required"

		var skillIDs []string
		if rel, ok := relations["skills"]; ok {
			if items, ok := rel.([]interface{}); ok {
				for _, item := range items {
					if sid, ok := item.(string); ok {
						skillIDs = append(skillIDs, sid)
						skillGroupMap[sid] = groupID
						if isRequired {
							requiredSkillIDs[sid] = true
						}
					}
				}
			}
		}

		groups = append(groups, SkillGroup{
			Identifier: groupID,
			Title:      stringProp(e, "title"),
			Required:   isRequired,
			SkillIDs:   skillIDs,
		})
	}

	result := &FetchedSkills{Groups: groups}
	for _, e := range skillEntities {
		props, _ := e["properties"].(map[string]interface{})
		skillID := stringProp(e, "identifier")

		skill := Skill{
			Identifier:   skillID,
			Title:        stringProp(e, "title"),
			Description:  stringFromMap(props, "description"),
			Instructions: stringFromMap(props, "instructions"),
			GroupID:      skillGroupMap[skillID],
			Required:     requiredSkillIDs[skillID],
			Location:     parseSkillLocation(stringFromMap(props, "location")),
			References:   parseSkillFiles(props, "references"),
			Assets:       parseSkillFiles(props, "assets"),
		}

		if skill.Required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}

	return result
}

func parseSkillLocation(raw string) SkillLocation {
	if raw == string(SkillLocationProject) {
		return SkillLocationProject
	}
	return SkillLocationGlobal
}

func parseSkillFiles(props map[string]interface{}, key string) []SkillFile {
	if props == nil {
		return nil
	}
	raw, ok := props[key].([]interface{})
	if !ok {
		return nil
	}
	var files []SkillFile
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		path := stringFromMap(m, "path")
		content := stringFromMap(m, "content")
		if path != "" && content != "" {
			files = append(files, SkillFile{Path: path, Content: content})
		}
	}
	return files
}

// FilterSkills returns the union of all required skills plus the optional
// skills matching the provided selection criteria.
func FilterSkills(fetched *FetchedSkills, selectAll, selectAllGroups, selectAllUngrouped bool, selectedGroups, selectedSkills []string) []Skill {
	var result []Skill
	result = append(result, fetched.Required...)

	if selectAll {
		result = append(result, fetched.Optional...)
		return result
	}

	selectedGroupSet := toSet(selectedGroups)
	selectedSkillSet := toSet(selectedSkills)

	for _, s := range fetched.Optional {
		ungrouped := s.GroupID == ""
		switch {
		case ungrouped && selectAllUngrouped:
			result = append(result, s)
		case ungrouped && selectedSkillSet[s.Identifier]:
			result = append(result, s)
		case !ungrouped && selectAllGroups:
			result = append(result, s)
		case !ungrouped && selectedGroupSet[s.GroupID]:
			result = append(result, s)
		case selectedSkillSet[s.Identifier]:
			result = append(result, s)
		}
	}
	return result
}

// GroupName resolves the display name for a group, falling back to its identifier.
func GroupName(groups []SkillGroup, groupID string) string {
	for _, g := range groups {
		if g.Identifier == groupID {
			if g.Title != "" {
				return g.Title
			}
			return g.Identifier
		}
	}
	if groupID != "" {
		return groupID
	}
	return NoGroupDir
}

const (
	NoGroupDir    = "_skills_without_group"
	PortSkillsDir = "port"
)

type skillKey struct{ group, skill string }

// WriteSkills writes SKILL.md files (plus references and assets) for each skill,
// routing each one based on its Location property:
//   - SkillLocationGlobal  → written into every dir in globalTargets
//   - SkillLocationProject → written into the matching tool sub-directory
//     inside every projectDir (e.g. <projectDir>/.agents/skills/port/…)
func WriteSkills(skills []Skill, groups []SkillGroup, globalTargets []string, projectDirs []string) error {
	globalSkills := make([]Skill, 0, len(skills))
	projectSkills := make([]Skill, 0)
	for _, s := range skills {
		if s.Location == SkillLocationProject {
			projectSkills = append(projectSkills, s)
		} else {
			globalSkills = append(globalSkills, s)
		}
	}

	if err := writeSkillsToTargets(globalSkills, globalTargets); err != nil {
		return err
	}

	if len(projectDirs) > 0 && len(projectSkills) > 0 {
		projectTargets := buildProjectTargets(globalTargets, projectDirs)
		if err := writeSkillsToTargets(projectSkills, projectTargets); err != nil {
			return err
		}
	}

	return nil
}

// buildProjectTargets creates project-level target paths by combining each
// project directory with the tool sub-directory extracted from the global
// targets. For example, if globalTargets contains "/home/user/.agents" and
// projectDirs contains "/repo", this produces "/repo/.agents".
func buildProjectTargets(globalTargets []string, projectDirs []string) []string {
	toolDirs := extractToolDirs(globalTargets)
	seen := make(map[string]bool)
	var result []string
	for _, pd := range projectDirs {
		for _, td := range toolDirs {
			p := filepath.Join(pd, td)
			if !seen[p] {
				result = append(result, p)
				seen[p] = true
			}
		}
	}
	return result
}

// extractToolDirs returns the relative tool directory names from absolute
// global target paths. It matches against known hook targets; unrecognized
// paths are included as-is (using the base name).
func extractToolDirs(globalTargets []string) []string {
	knownTargets := DefaultHookTargets()
	seen := make(map[string]bool)
	var dirs []string
	for _, gt := range globalTargets {
		expanded := expandHome(gt)
		matched := false
		for _, kt := range knownTargets {
			if strings.HasSuffix(expanded, string(filepath.Separator)+kt.Dir) || gt == kt.Dir {
				if !seen[kt.Dir] {
					dirs = append(dirs, kt.Dir)
					seen[kt.Dir] = true
				}
				matched = true
				break
			}
		}
		if !matched {
			base := filepath.Base(expanded)
			if !seen[base] {
				dirs = append(dirs, base)
				seen[base] = true
			}
		}
	}
	return dirs
}

func writeSkillsToTargets(skills []Skill, targets []string) error {
	expected := make(map[skillKey]bool, len(skills))
	for _, s := range skills {
		groupDir := s.GroupID
		if groupDir == "" {
			groupDir = NoGroupDir
		}
		expected[skillKey{groupDir, s.Identifier}] = true
	}

	for _, target := range targets {
		expanded := expandHome(target)
		portDir := filepath.Join(expanded, "skills", PortSkillsDir)

		for _, s := range skills {
			groupDir := s.GroupID
			if groupDir == "" {
				groupDir = NoGroupDir
			}

			skillDir := filepath.Join(portDir, groupDir, s.Identifier)
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				return fmt.Errorf("failed to create skill directory %s: %w", skillDir, err)
			}

			content := buildSkillMD(s)
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write SKILL.md for %s: %w", s.Identifier, err)
			}

			for _, f := range s.References {
				if err := writeSkillFile(skillDir, f); err != nil {
					return fmt.Errorf("failed to write reference file %s for skill %s: %w", f.Path, s.Identifier, err)
				}
			}
			for _, f := range s.Assets {
				if err := writeSkillFile(skillDir, f); err != nil {
					return fmt.Errorf("failed to write asset file %s for skill %s: %w", f.Path, s.Identifier, err)
				}
			}
		}

		if err := reconcileSkills(portDir, expected); err != nil {
			return fmt.Errorf("reconciliation failed for %s: %w", target, err)
		}
	}
	return nil
}

func reconcileSkills(portDir string, expected map[skillKey]bool) error {
	groupEntries, err := os.ReadDir(portDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, groupEntry := range groupEntries {
		if !groupEntry.IsDir() {
			continue
		}
		groupName := groupEntry.Name()
		groupPath := filepath.Join(portDir, groupName)

		skillEntries, err := os.ReadDir(groupPath)
		if err != nil {
			continue
		}

		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			key := skillKey{groupName, skillEntry.Name()}
			if !expected[key] {
				if err := os.RemoveAll(filepath.Join(groupPath, skillEntry.Name())); err != nil {
					return fmt.Errorf("failed to remove stale skill %s/%s: %w", groupName, skillEntry.Name(), err)
				}
			}
		}

		remaining, _ := os.ReadDir(groupPath)
		if len(remaining) == 0 {
			_ = os.Remove(groupPath)
		}
	}
	return nil
}

func writeSkillFile(skillDir string, f SkillFile) error {
	dest := filepath.Join(skillDir, filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dest, err)
	}
	return os.WriteFile(dest, []byte(f.Content), 0o644)
}

func buildSkillMD(s Skill) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", s.Identifier))
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	}
	sb.WriteString("---\n\n")

	if s.Instructions != "" {
		sb.WriteString(s.Instructions)
		if !strings.HasSuffix(s.Instructions, "\n") {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("# %s\n\n_No instructions provided._\n", s.Title))
	}

	return sb.String()
}

func stringProp(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func stringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func toSet(slice []string) map[string]bool {
	s := make(map[string]bool, len(slice))
	for _, v := range slice {
		s[v] = true
	}
	return s
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

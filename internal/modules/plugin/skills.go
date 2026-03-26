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
// "global" targets the user's home-directory AI tool dirs; "project" targets
// the current working directory. Missing or unrecognised values default to "global".
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
// Required is true when enforcement == "required"; all skills in this group are
// always synced. SkillIDs lists the identifiers of related skills via the
// skill_group.relations.skills many-relation.
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
//
// The skill_group blueprint owns the relation: skill_group.relations.skills (many → skill).
// Required enforcement is determined by skill_group.properties.enforcement == "required".
func FetchSkills(ctx context.Context, client *api.Client) (*FetchedSkills, error) {
	groupEntities, err := client.GetSkillGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill groups: %w", err)
	}

	skillEntities, err := client.GetSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills: %w", err)
	}

	// Parse skill groups, recording which skill IDs belong to each group
	// and whether that group is required.
	groups := make([]SkillGroup, 0, len(groupEntities))
	// requiredSkillIDs collects skill IDs that belong to a required group.
	requiredSkillIDs := make(map[string]bool)
	// skillGroupMap maps a skill ID → group identifier.
	skillGroupMap := make(map[string]string)

	for _, e := range groupEntities {
		props, _ := e["properties"].(map[string]interface{})
		relations, _ := e["relations"].(map[string]interface{})

		groupID := stringProp(e, "identifier")
		enforcement := stringFromMap(props, "enforcement")
		isRequired := enforcement == "required"

		// The many-relation "skills" is a []interface{} of skill identifiers.
		var skillIDs []string
		if rel, ok := relations["skills"]; ok {
			switch v := rel.(type) {
			case []interface{}:
				for _, item := range v {
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

	// Parse skills.
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

	return result, nil
}

// parseSkillLocation converts the raw location string from the API into a
// SkillLocation. Any value other than "project" is treated as "global".
func parseSkillLocation(raw string) SkillLocation {
	if raw == string(SkillLocationProject) {
		return SkillLocationProject
	}
	return SkillLocationGlobal
}

// parseSkillFiles extracts references or assets from a skill's properties map.
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

// FilterSkills returns the union of all required skills plus the optional skills
// matching the provided selection options.
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

// GroupName resolves the display name (or falls back to the identifier) for a group.
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
	// NoGroupDir is the folder name used for skills that have no group.
	NoGroupDir = "_skills_without_group"
	// PortSkillsDir is the subdirectory under {target}/skills/ that holds all Port skills.
	PortSkillsDir = "port"
)

// skillKey identifies a skill by its group directory and skill identifier.
type skillKey struct{ group, skill string }

// WriteSkills writes SKILL.md files (plus references and assets) for each skill,
// routing each one based on its Location property:
//   - SkillLocationGlobal  → written into every dir in globalTargets
//   - SkillLocationProject → written into projectDir (the cwd where the CLI ran)
//
// Pass an empty projectDir to skip project-scoped skills entirely.
// Stale skill directories are removed from every target (reconciliation).
// Layout: {target}/skills/port/{group-identifier}/{skill-identifier}/SKILL.md
func WriteSkills(skills []Skill, groups []SkillGroup, globalTargets []string, projectDir string) error {
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

	if projectDir != "" && len(projectSkills) > 0 {
		if err := writeSkillsToTargets(projectSkills, []string{projectDir}); err != nil {
			return err
		}
	}

	return nil
}

// writeSkillsToTargets writes a slice of skills into every provided target
// directory and reconciles stale skill dirs.
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

// reconcileSkills walks portDir and removes any {group}/{skill} directories
// that are not in the expected set. Empty group directories are also cleaned up.
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

		// Remove the group directory if it is now empty.
		remaining, _ := os.ReadDir(groupPath)
		if len(remaining) == 0 {
			_ = os.Remove(groupPath)
		}
	}
	return nil
}

// writeSkillFile writes a single reference or asset file relative to the skill directory.
func writeSkillFile(skillDir string, f SkillFile) error {
	dest := filepath.Join(skillDir, filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dest, err)
	}
	return os.WriteFile(dest, []byte(f.Content), 0o644)
}

// buildSkillMD produces the SKILL.md content for a skill.
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

// --- helpers ---

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

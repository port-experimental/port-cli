package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// Skill holds the data for a single skill entity fetched from Port.
type Skill struct {
	Identifier  string
	Title       string
	Description string
	Instructions string
	GroupID     string
	Required    bool
}

// SkillGroup holds the data for a single skill_group entity fetched from Port.
type SkillGroup struct {
	Identifier string
	Title      string
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

	groups := make([]SkillGroup, 0, len(groupEntities))
	for _, e := range groupEntities {
		groups = append(groups, SkillGroup{
			Identifier: stringProp(e, "identifier"),
			Title:      stringProp(e, "title"),
		})
	}

	result := &FetchedSkills{Groups: groups}
	for _, e := range skillEntities {
		props, _ := e["properties"].(map[string]interface{})
		relations, _ := e["relations"].(map[string]interface{})

		groupID := ""
		if rel, ok := relations["skill_group"]; ok {
			switch v := rel.(type) {
			case string:
				groupID = v
			case map[string]interface{}:
				groupID = stringFromMap(v, "identifier")
			}
		}

		required := false
		if props != nil {
			if v, ok := props["required"].(bool); ok {
				required = v
			}
		}

		skill := Skill{
			Identifier:   stringProp(e, "identifier"),
			Title:        stringProp(e, "title"),
			Description:  stringFromMap(props, "description"),
			Instructions: stringFromMap(props, "instructions"),
			GroupID:      groupID,
			Required:     required,
		}

		if required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}

	return result, nil
}

// FilterSkills returns the union of all required skills plus the optional skills
// whose identifier or group identifier matches the provided selections.
func FilterSkills(fetched *FetchedSkills, selectedGroups, selectedSkills []string) []Skill {
	selectedGroupSet := toSet(selectedGroups)
	selectedSkillSet := toSet(selectedSkills)

	var result []Skill
	result = append(result, fetched.Required...)

	for _, s := range fetched.Optional {
		if selectedSkillSet[s.Identifier] || selectedGroupSet[s.GroupID] {
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
	return "other"
}

// WriteSkills writes SKILL.md files for each skill into every target directory.
// Layout: {target}/skills/{group-identifier}/{skill-identifier}/SKILL.md
func WriteSkills(skills []Skill, groups []SkillGroup, targets []string) error {
	for _, target := range targets {
		expanded := expandHome(target)
		for _, s := range skills {
			groupDir := s.GroupID
			if groupDir == "" {
				groupDir = "other"
			}

			dir := filepath.Join(expanded, "skills", groupDir, s.Identifier)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create skill directory %s: %w", dir, err)
			}

			content := buildSkillMD(s)
			path := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write skill file %s: %w", path, err)
			}
		}
	}
	return nil
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

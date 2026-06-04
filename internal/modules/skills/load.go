// Package skills syncs Port catalog skills to local AI tool directories.
// Catalog reads and writes go through internal/api/aiservice only (not port-api blueprints).
package skills

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/auth"
)

// CatalogFromAIService maps the grouped ai-service response to FetchedSkills.
func CatalogFromAIService(resp *aiservice.GroupedSkillsResponse) *FetchedSkills {
	if resp == nil {
		return &FetchedSkills{}
	}
	result := &FetchedSkills{}
	seen := make(map[string]bool)
	for _, g := range resp.Groups {
		group := SkillGroup{
			Identifier: g.Identifier,
			Title:      g.Title,
			SkillIDs:   make([]string, 0, len(g.Skills)),
		}
		for _, s := range g.Skills {
			group.SkillIDs = append(group.SkillIDs, s.Identifier)
			if !seen[s.Identifier] {
				seen[s.Identifier] = true
				result.Skills = append(result.Skills, skillFromAIService(s, []string{g.Identifier}))
			}
		}
		result.Groups = append(result.Groups, group)
	}
	for _, s := range resp.UngroupedSkills {
		if seen[s.Identifier] {
			continue
		}
		seen[s.Identifier] = true
		result.Skills = append(result.Skills, skillFromAIService(s, nil))
	}
	return result
}

func coalesceGroupIDs(fromGroup, fromSkill []string) []string {
	if len(fromGroup) > 0 {
		return append([]string(nil), fromGroup...)
	}
	if len(fromSkill) == 0 {
		return nil
	}
	return append([]string(nil), fromSkill...)
}

func skillFromAIService(s aiservice.SkillAtLatestVersion, groupIDs []string) Skill {
	groupIDs = coalesceGroupIDs(groupIDs, s.GroupIdentifiers)
	files := make([]SkillFile, 0, len(s.Files))
	for _, f := range s.Files {
		path, _ := f.Properties["path"].(string)
		content, _ := f.Properties["content"].(string)
		if path == "" {
			continue
		}
		files = append(files, SkillFile{Path: path, Content: content})
	}
	return Skill{
		Identifier:  s.Identifier,
		Title:       s.Title,
		Description: s.Description,
		Version:     s.Version,
		CreatedBy:   s.CreatedBy,
		GroupIDs:    append([]string(nil), groupIDs...),
		Location:    parseSkillLocation(s.Location),
		Files:       files,
	}
}

// FetchSkillsQuery optional filters for loading the sync catalog from ai-service.
type FetchSkillsQuery struct {
	SkillIdentifiers []string
	IncludeGroups    []string
	ExcludeGroups    []string
	// TeamsDefault when set is sent as teams_default query param (ai-service defaults to true if omitted).
	TeamsDefault *bool
	// Exclude lists response parts to omit (e.g. files, legacy, internal).
	Exclude []string
	// ExcludeFiles requests GET /v1/skills?exclude=files (metadata only, for init prompts).
	ExcludeFiles bool
}

// ExcludeSkillFiles is the ai-service exclude query value for omitting file content.
const ExcludeSkillFiles = "files"

// FetchSkillGroupsFromAIService loads all skill groups for init selection.
func FetchSkillGroupsFromAIService(ctx context.Context, aiClient *aiservice.Client, token *auth.Token) ([]aiservice.SkillGroupCatalogEntry, error) {
	if aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := aiClient.GetSkillGroups(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill groups from ai-service: %w", err)
	}
	return resp.Groups, nil
}

// FetchSkillsFromAIService loads the skill catalog from ai-service.
func FetchSkillsFromAIService(ctx context.Context, aiClient *aiservice.Client, token *auth.Token, query FetchSkillsQuery) (*FetchedSkills, error) {
	if aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	skillQuery := aiservice.GetSkillsQuery{
		SkillIdentifiers: query.SkillIdentifiers,
		IncludeGroups:    query.IncludeGroups,
		ExcludeGroups:    query.ExcludeGroups,
		TeamsDefault:     query.TeamsDefault,
		Exclude:          append([]string(nil), query.Exclude...),
	}
	if query.ExcludeFiles {
		skillQuery.Exclude = append(skillQuery.Exclude, ExcludeSkillFiles)
	}
	resp, err := aiClient.GetSkillsGrouped(ctx, token, skillQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills from ai-service: %w", err)
	}
	return CatalogFromAIService(resp), nil
}

// UngroupedSkills returns skills that are not members of any group in the catalog.
// Membership is determined from each group's SkillIDs (authoritative), not the
// per-skill GroupIDs field — the API may list grouped skills under ungroupedSkills
// when team filters are applied.
// Pass a catalog from GET /v1/skills without include_groups/exclude_groups/teams_default.
func UngroupedSkills(fetched *FetchedSkills) []Skill {
	if fetched == nil {
		return nil
	}
	inGroup := groupedSkillIDSet(fetched.Groups)
	var out []Skill
	seen := make(map[string]bool)
	for _, s := range fetched.Skills {
		if inGroup[s.Identifier] || seen[s.Identifier] {
			continue
		}
		seen[s.Identifier] = true
		out = append(out, s)
	}
	return out
}

func groupedSkillIDSet(groups []SkillGroup) map[string]bool {
	set := make(map[string]bool)
	for _, g := range groups {
		for _, id := range g.SkillIDs {
			set[id] = true
		}
	}
	return set
}

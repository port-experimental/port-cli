// Package skills syncs Port catalog skills to local AI tool directories.
// Catalog reads and writes go through internal/api skills routes.
package skills

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
)

// CatalogFromAPI maps the grouped skills API response to FetchedSkills.
func CatalogFromAPI(resp *api.GroupedSkillsResponse) *FetchedSkills {
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
				result.Skills = append(result.Skills, skillFromAPI(s, []string{g.Identifier}))
			}
		}
		result.Groups = append(result.Groups, group)
	}
	for _, s := range resp.UngroupedSkills {
		if seen[s.Identifier] {
			continue
		}
		seen[s.Identifier] = true
		result.Skills = append(result.Skills, skillFromAPI(s, nil))
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

func skillFromAPI(s api.SkillAtLatestVersion, groupIDs []string) Skill {
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
		GroupIDs:    append([]string(nil), groupIDs...),
		Location:    parseSkillLocation(s.Location),
		Files:       files,
	}
}

// FetchSkillsQuery optional filters for loading the sync catalog.
type FetchSkillsQuery struct {
	SkillIdentifiers []string
	IncludeGroups    []string
	ExcludeGroups    []string
	TeamsDefault     *bool
	Exclude          []string
	ExcludeFiles     bool
}

// ExcludeSkillFiles is the exclude query value for omitting file content.
const ExcludeSkillFiles = "files"

// FetchSkillGroupsFromAPI loads all skill groups for init selection.
func FetchSkillGroupsFromAPI(ctx context.Context, client *api.Client) ([]api.SkillGroupCatalogEntry, error) {
	if client == nil {
		return nil, fmt.Errorf("API client is not configured")
	}
	resp, err := client.GetSkillGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill groups: %w", err)
	}
	return resp.Groups, nil
}

// FetchSkillsFromAPI loads the skill catalog.
func FetchSkillsFromAPI(ctx context.Context, client *api.Client, query FetchSkillsQuery) (*FetchedSkills, error) {
	if client == nil {
		return nil, fmt.Errorf("API client is not configured")
	}
	skillQuery := api.GetSkillsQuery{
		SkillIdentifiers: query.SkillIdentifiers,
		IncludeGroups:    query.IncludeGroups,
		ExcludeGroups:    query.ExcludeGroups,
		TeamsDefault:     query.TeamsDefault,
		Exclude:          append([]string(nil), query.Exclude...),
	}
	if query.ExcludeFiles {
		skillQuery.Exclude = append(skillQuery.Exclude, ExcludeSkillFiles)
	}
	resp, err := client.GetSkillsGrouped(ctx, skillQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills: %w", err)
	}
	return CatalogFromAPI(resp), nil
}

// UngroupedSkills returns skills that are not members of any group in the catalog.
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

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
	for _, g := range resp.Groups {
		group := SkillGroup{
			Identifier: g.Identifier,
			Title:      g.Title,
			Required:   g.Required,
			AutoSync:   g.AutoSync,
			SkillIDs:   make([]string, 0, len(g.Skills)),
		}
		for _, s := range g.Skills {
			group.SkillIDs = append(group.SkillIDs, s.Identifier)
			skill := skillFromAIService(s, []string{g.Identifier}, g.Required, g.AutoSync)
			if skill.Required {
				result.Required = append(result.Required, skill)
			} else {
				result.Optional = append(result.Optional, skill)
			}
		}
		result.Groups = append(result.Groups, group)
	}
	for _, s := range resp.UngroupedSkills {
		skill := skillFromAIService(s, nil, false, false)
		if skill.Required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}
	return result
}

func skillFromAIService(s aiservice.SkillAtLatestVersion, groupIDs []string, groupRequired, groupAutoSync bool) Skill {
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
		GroupIDs:    append([]string(nil), groupIDs...),
		Required:    groupRequired,
		AutoSync:    groupAutoSync,
		Location:    parseSkillLocation(s.Location),
		Versioned:   true,
		Files:       files,
	}
}

// FetchSkillsFromAIService loads the skill catalog from ai-service.
func FetchSkillsFromAIService(ctx context.Context, aiClient *aiservice.Client, token *auth.Token, skillIDs []string) (*FetchedSkills, error) {
	if aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := aiClient.GetSkillsGrouped(ctx, token, aiservice.GetSkillsQuery{
		SkillIdentifiers: skillIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills from ai-service: %w", err)
	}
	return CatalogFromAIService(resp), nil
}

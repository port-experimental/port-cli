package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

func TestCatalogFromAIService_GroupedSkills(t *testing.T) {
	resp := &aiservice.GroupedSkillsResponse{
		Groups: []aiservice.SkillGroupAtLatestVersion{
			{
				Identifier: "g1",
				Title:      "Group 1",
				Skills: []aiservice.SkillAtLatestVersion{
					{
						Identifier: "skill-a",
						Title:      "Skill A",
						Location:   "global",
						Files: []aiservice.SkillFile{
							{Properties: map[string]interface{}{"path": "SKILL.md", "content": "# A"}},
						},
					},
				},
			},
		},
		UngroupedSkills: []aiservice.SkillAtLatestVersion{
			{
				Identifier: "solo",
				Title:      "Solo",
				Location:   "project",
				Files: []aiservice.SkillFile{
					{Properties: map[string]interface{}{"path": "SKILL.md", "content": "# Solo"}},
				},
			},
		},
	}
	fetched := CatalogFromAIService(resp)
	if len(fetched.Skills) != 2 {
		t.Fatalf("skills: %+v", fetched.Skills)
	}
	ids := []string{fetched.Skills[0].Identifier, fetched.Skills[1].Identifier}
	if !contains(ids, "skill-a") || !contains(ids, "solo") {
		t.Fatalf("expected skill-a and solo, got %v", ids)
	}
	if len(fetched.Groups) != 1 || fetched.Groups[0].Identifier != "g1" {
		t.Fatalf("groups: %+v", fetched.Groups)
	}
}

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
				Required:   true,
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
	if len(fetched.Required) != 1 || fetched.Required[0].Identifier != "skill-a" {
		t.Fatalf("required: %+v", fetched.Required)
	}
	if len(fetched.Optional) != 1 || fetched.Optional[0].Identifier != "solo" {
		t.Fatalf("optional: %+v", fetched.Optional)
	}
	if len(fetched.Groups) != 1 || fetched.Groups[0].Identifier != "g1" {
		t.Fatalf("groups: %+v", fetched.Groups)
	}
}

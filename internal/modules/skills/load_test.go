package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

func TestUngroupedSkills(t *testing.T) {
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "g1", SkillIDs: []string{"in-group", "also-grouped"}}},
		Skills: []Skill{
			{Identifier: "in-group", GroupIDs: []string{"g1"}},
			{Identifier: "standalone", GroupIDs: nil},
			{Identifier: "also-grouped", GroupIDs: nil}, // empty GroupIDs but listed in group
		},
	}
	got := UngroupedSkills(fetched)
	if len(got) != 1 || got[0].Identifier != "standalone" {
		t.Fatalf("got %+v", got)
	}
}

func TestUngroupedSkills_ExcludesGroupedListedAsUngrouped(t *testing.T) {
	catalog := CatalogFromAIService(&aiservice.GroupedSkillsResponse{
		Groups: []aiservice.SkillGroupAtLatestVersion{{
			Identifier: "platform-engineering",
			Skills: []aiservice.SkillAtLatestVersion{
				{Identifier: "local-dev-setup"},
				{Identifier: "port-api-client"},
			},
		}},
		UngroupedSkills: []aiservice.SkillAtLatestVersion{
			{Identifier: "local-dev-setup"},
			{Identifier: "port-api-client"},
			{Identifier: "integrations-overview"},
		},
	})
	got := UngroupedSkills(catalog)
	if len(got) != 1 || got[0].Identifier != "integrations-overview" {
		t.Fatalf("got %+v", got)
	}
}

func TestSkillFromAIService_MapsVersionAndCreatedBy(t *testing.T) {
	s := skillFromAIService(aiservice.SkillAtLatestVersion{
		Identifier: "demo-skill",
		Title:      "Demo",
		Version:    "2.0.0",
		CreatedBy:  "user@example.com",
		Location:   "global",
	}, nil)
	if s.Version != "2.0.0" || s.CreatedBy != "user@example.com" {
		t.Fatalf("got %+v", s)
	}
}

func TestCatalogFromAIService_UngroupedSeparateFromGroups(t *testing.T) {
	resp := &aiservice.GroupedSkillsResponse{
		Groups: []aiservice.SkillGroupAtLatestVersion{
			{
				Identifier: "g1",
				Skills: []aiservice.SkillAtLatestVersion{
					{Identifier: "grouped-skill"},
				},
			},
		},
		UngroupedSkills: []aiservice.SkillAtLatestVersion{
			{Identifier: "standalone"},
		},
	}
	catalog := CatalogFromAIService(resp)
	ungrouped := UngroupedSkills(catalog)
	if len(ungrouped) != 1 || ungrouped[0].Identifier != "standalone" {
		t.Fatalf("ungrouped: %+v", ungrouped)
	}
}

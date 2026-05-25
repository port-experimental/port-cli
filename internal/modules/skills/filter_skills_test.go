package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestFilterSkills(t *testing.T) {
	tests := []struct {
		name               string
		fetched            *FetchedSkills
		selectAll          bool
		selectAllGroups    bool
		selectAllUngrouped bool
		selectedGroups     []string
		selectedSkills     []string
		wantIDs            []string
	}{
		{
			name: "SelectAll includes everything",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-1", GroupIDs: []string{"group-a"}}, {Identifier: "opt-2"}},
			},
			selectAll: true,
			wantIDs:   []string{"req-1", "opt-1", "opt-2"},
		},
		{
			name: "required always included with no selection",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-1", GroupIDs: []string{"group-a"}}},
			},
			wantIDs: []string{"req-1"},
		},
		{
			name: "SelectAllGroups includes grouped only",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-grouped", GroupIDs: []string{"group-a"}}, {Identifier: "opt-ungrouped"}},
			},
			selectAllGroups: true,
			wantIDs:         []string{"req-1", "opt-grouped"},
		},
		{
			name: "SelectAllUngrouped includes ungrouped only",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "grouped", GroupIDs: []string{"group-a"}},
					{Identifier: "ungrouped-1"},
					{Identifier: "ungrouped-2"},
				},
			},
			selectAllUngrouped: true,
			wantIDs:            []string{"ungrouped-1", "ungrouped-2"},
		},
		{
			name: "specific groups",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "skill-a", GroupIDs: []string{"group-a"}},
					{Identifier: "skill-b", GroupIDs: []string{"group-b"}},
					{Identifier: "skill-c", GroupIDs: []string{"group-c"}},
				},
			},
			selectedGroups: []string{"group-a", "group-b"},
			wantIDs:        []string{"skill-a", "skill-b"},
		},
		{
			name: "specific skills",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "skill-1"},
					{Identifier: "skill-2"},
					{Identifier: "skill-3"},
				},
			},
			selectedSkills: []string{"skill-1", "skill-3"},
			wantIDs:        []string{"skill-1", "skill-3"},
		},
		{
			name: "auto sync optional skills are included without selection",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "auto", GroupIDs: []string{"group-a"}, AutoSync: true},
					{Identifier: "manual", GroupIDs: []string{"group-b"}},
				},
			},
			wantIDs: []string{"auto"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterSkills(tt.fetched, tt.selectAll, tt.selectAllGroups, tt.selectAllUngrouped, tt.selectedGroups, tt.selectedSkills)
			ids := identifiers(result)
			if len(ids) != len(tt.wantIDs) {
				t.Fatalf("want %d skills (%v), got %d (%v)", len(tt.wantIDs), tt.wantIDs, len(ids), ids)
			}
			for _, want := range tt.wantIDs {
				if !contains(ids, want) {
					t.Errorf("expected %q in result %v", want, ids)
				}
			}
		})
	}
}

func TestParseFetchedSkills_GroupRelationAndRequired(t *testing.T) {
	groupEntities := []api.Entity{
		{
			"identifier": "group-required",
			"title":      "Required Group",
			"properties": map[string]interface{}{"enforcement": "required"},
			"relations":  map[string]interface{}{"skills": []interface{}{"skill-a", "skill-b"}},
		},
		{
			"identifier": "group-optional",
			"title":      "Optional Group",
			"properties": map[string]interface{}{"enforcement": "optional"},
			"relations":  map[string]interface{}{"skills": []interface{}{"skill-c"}},
		},
	}
	skillEntities := []api.Entity{
		{"identifier": "skill-a", "title": "A", "properties": map[string]interface{}{"instructions": "do a"}},
		{"identifier": "skill-b", "title": "B", "properties": map[string]interface{}{"instructions": "do b"}},
		{"identifier": "skill-c", "title": "C", "properties": map[string]interface{}{"instructions": "do c"}},
	}
	fetched := ParseFetchedSkills(groupEntities, skillEntities)

	if len(fetched.Required) != 2 {
		t.Errorf("want 2 required, got %d", len(fetched.Required))
	}
	if len(fetched.Optional) != 1 || fetched.Optional[0].Identifier != "skill-c" {
		t.Errorf("want 1 optional (skill-c), got %v", fetched.Optional)
	}
	for _, s := range fetched.Required {
		if !contains(s.GroupIDs, "group-required") {
			t.Errorf("expected GroupIDs to contain group-required for %s, got %v", s.Identifier, s.GroupIDs)
		}
	}
}

func TestParseFetchedSkills_UngroupedAndFiles(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "skill-with-files",
			"title":      "Skill With Files",
			"properties": map[string]interface{}{
				"instructions":     "do it",
				"references":       []interface{}{map[string]interface{}{"path": "refs/guide.md", "content": "# Guide"}},
				"assets":           []interface{}{map[string]interface{}{"path": "assets/tpl.yaml", "content": "key: value"}},
				"scripts":          []interface{}{map[string]interface{}{"path": "scripts/run.sh", "content": "#!/bin/sh\n"}},
				"additional_files": []interface{}{map[string]interface{}{"path": "LICENSE", "content": "MIT"}},
			},
		},
	}
	fetched := ParseFetchedSkills(nil, skillEntities)
	s := fetched.Optional[0]
	if len(s.GroupIDs) != 0 {
		t.Errorf("expected empty GroupIDs, got %v", s.GroupIDs)
	}
	if len(s.References) != 1 || s.References[0].Path != "refs/guide.md" {
		t.Errorf("unexpected references: %+v", s.References)
	}
	if len(s.Assets) != 1 || s.Assets[0].Path != "assets/tpl.yaml" {
		t.Errorf("unexpected assets: %+v", s.Assets)
	}
	if len(s.Scripts) != 1 || s.Scripts[0].Path != "scripts/run.sh" {
		t.Errorf("unexpected scripts: %+v", s.Scripts)
	}
	if len(s.AdditionalFiles) != 1 || s.AdditionalFiles[0].Path != "LICENSE" {
		t.Errorf("unexpected additional_files: %+v", s.AdditionalFiles)
	}
}

func TestParseFetchedSkills_SkipsUnaddressableUngroupedPlaceholderEntities(t *testing.T) {
	skillEntities := []api.Entity{
		{"identifier": "via-demo/skills-team-bff/orphan-file", "title": "via-demo/skills-team-bff/orphan-file"},
		{"identifier": "real-skill", "title": "Real Skill"},
	}

	fetched := ParseFetchedSkills(nil, skillEntities)
	if len(fetched.Optional) != 1 {
		t.Fatalf("want only real skill, got %+v", fetched.Optional)
	}
	if fetched.Optional[0].Identifier != "real-skill" {
		t.Fatalf("unexpected skill: %+v", fetched.Optional[0])
	}
}

func TestFilterOrphanSkillFiles_IgnoresStandaloneFilesInAnySkillsFolder(t *testing.T) {
	files := filterOrphanSkillFiles(Skill{Identifier: "real-skill", Title: "real-skill"}, []SkillFile{
		{Path: ".cursor/skills/standalone-file", Content: "ignored"},
		{Path: ".cursor/skills/port/standalone-file", Content: "ignored"},
		{Path: ".cursor/skills/engineering/real-skill/SKILL.md", Content: "kept"},
		{Path: ".cursor/skills/real-skill/references/guide.md", Content: "also kept"},
	})

	if len(files) != 2 {
		t.Fatalf("want 2 files, got %+v", files)
	}
	if files[0].Path != ".cursor/skills/engineering/real-skill/SKILL.md" {
		t.Fatalf("unexpected first file: %+v", files[0])
	}
	if files[1].Path != ".cursor/skills/real-skill/references/guide.md" {
		t.Fatalf("unexpected second file: %+v", files[1])
	}
}

func TestParseFetchedSkillEntities_NewVersionedStructure(t *testing.T) {
	groupEntities := []api.Entity{
		{
			"identifier": "org/platform",
			"title":      "platform",
			"properties": map[string]interface{}{"enforcement": "required", "auto_sync": true},
		},
	}
	skillEntities := []api.Entity{
		{
			"identifier": "org/platform/deploy-helper",
			"title":      "deploy-helper",
			"properties": map[string]interface{}{"location": "project"},
			"relations": map[string]interface{}{
				"skill_to_skill_group": []interface{}{
					map[string]interface{}{"identifier": "org/platform", "title": "platform"},
				},
			},
		},
	}
	versionEntities := []api.Entity{
		{
			"identifier": "old-version",
			"properties": map[string]interface{}{"description": "old", "version": "0.0.1"},
			"relations": map[string]interface{}{
				"skill_version_to_skill": map[string]interface{}{"identifier": "org/platform/deploy-helper"},
			},
		},
		{
			"identifier": "new-version",
			"properties": map[string]interface{}{"description": "new", "version": "0.0.2"},
			"relations": map[string]interface{}{
				"skill_version_to_skill": map[string]interface{}{"identifier": "org/platform/deploy-helper"},
			},
		},
	}
	fileEntities := []api.Entity{
		{
			"identifier": "old-file",
			"properties": map[string]interface{}{"path": ".cursor/skills/port/deploy-helper/SKILL.md", "content": "old content"},
			"relations": map[string]interface{}{
				"skill_file_to_skill_version": map[string]interface{}{"identifier": "old-version"},
			},
		},
		{
			"identifier": "new-file",
			"properties": map[string]interface{}{"path": ".cursor/skills/port/deploy-helper/SKILL.md", "content": "new content"},
			"relations": map[string]interface{}{
				"skill_file_to_skill_version": map[string]interface{}{"identifier": "new-version"},
			},
		},
	}

	fetched := ParseFetchedSkillEntities(groupEntities, skillEntities, versionEntities, fileEntities)
	if len(fetched.Required) != 1 {
		t.Fatalf("want 1 required skill, got %d optional=%d", len(fetched.Required), len(fetched.Optional))
	}
	s := fetched.Required[0]
	if s.Identifier != "org/platform/deploy-helper" {
		t.Fatalf("unexpected skill identifier %q", s.Identifier)
	}
	if s.Description != "new" {
		t.Errorf("expected latest version description, got %q", s.Description)
	}
	if s.Location != SkillLocationProject {
		t.Errorf("expected project location, got %q", s.Location)
	}
	if len(s.GroupIDs) != 1 || s.GroupIDs[0] != "org/platform" {
		t.Errorf("expected group relation from skill, got %v", s.GroupIDs)
	}
	if len(s.Files) != 1 || s.Files[0].Content != "new content" {
		t.Errorf("expected latest version file only, got %+v", s.Files)
	}
}

func TestParseFetchedSkillEntities_UsesSemanticLatestVersion(t *testing.T) {
	skillEntities := []api.Entity{{"identifier": "skill-a", "title": "Skill A"}}
	versionEntities := []api.Entity{
		{
			"identifier": "v1.9.0",
			"properties": map[string]interface{}{"version": "1.9.0"},
			"relations":  map[string]interface{}{"skill_version_to_skill": map[string]interface{}{"identifier": "skill-a"}},
		},
		{
			"identifier": "v1.10.0",
			"properties": map[string]interface{}{"version": "1.10.0"},
			"relations":  map[string]interface{}{"skill_version_to_skill": map[string]interface{}{"identifier": "skill-a"}},
		},
	}
	fileEntities := []api.Entity{
		{
			"properties": map[string]interface{}{"path": "SKILL.md", "content": "one nine"},
			"relations":  map[string]interface{}{"skill_file_to_skill_version": map[string]interface{}{"identifier": "v1.9.0"}},
		},
		{
			"properties": map[string]interface{}{"path": "SKILL.md", "content": "one ten"},
			"relations":  map[string]interface{}{"skill_file_to_skill_version": map[string]interface{}{"identifier": "v1.10.0"}},
		},
	}

	fetched := ParseFetchedSkillEntities(nil, skillEntities, versionEntities, fileEntities)
	if len(fetched.Optional) != 1 {
		t.Fatalf("want 1 optional skill, got %d", len(fetched.Optional))
	}
	if got := fetched.Optional[0].Files[0].Content; got != "one ten" {
		t.Errorf("expected semantic latest version file, got %q", got)
	}
}

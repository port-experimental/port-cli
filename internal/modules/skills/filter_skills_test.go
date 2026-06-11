package skills

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			name: "auto sync skills are NOT included without group selection",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "auto", GroupIDs: []string{"group-a"}, AutoSync: true},
					{Identifier: "manual", GroupIDs: []string{"group-b"}},
				},
			},
			wantIDs: []string{},
		},
		{
			name: "auto sync skills included when their group is selected",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "auto", GroupIDs: []string{"group-a"}, AutoSync: true},
					{Identifier: "manual", GroupIDs: []string{"group-b"}},
				},
			},
			selectedGroups: []string{"group-a"},
			wantIDs:        []string{"auto"},
		},
		{
			name: "auto sync skills NOT included when a different group is selected",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "auto-b", GroupIDs: []string{"group-b"}, AutoSync: true},
					{Identifier: "manual-a", GroupIDs: []string{"group-a"}},
				},
			},
			selectedGroups: []string{"group-a"},
			wantIDs:        []string{"manual-a"},
		},
		{
			name: "auto sync skills included with selectAllGroups",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "auto", GroupIDs: []string{"group-a"}, AutoSync: true},
					{Identifier: "manual", GroupIDs: []string{"group-b"}},
				},
			},
			selectAllGroups: true,
			wantIDs:         []string{"auto", "manual"},
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

func TestParseFetchedSkills_KeepsLegacyUngroupedEntities(t *testing.T) {
	skillEntities := []api.Entity{
		{"identifier": "via-demo/skills-team-bff/orphan-file", "title": "via-demo/skills-team-bff/orphan-file"},
		{"identifier": "real-skill", "title": "Real Skill"},
	}

	fetched := ParseFetchedSkills(nil, skillEntities)
	if len(fetched.Optional) != 2 {
		t.Fatalf("want legacy parser to keep both skills, got %+v", fetched.Optional)
	}
	if fetched.Optional[0].Identifier != "via-demo/skills-team-bff/orphan-file" {
		t.Fatalf("unexpected first skill: %+v", fetched.Optional[0])
	}
}

func TestFilterOrphanSkillFiles_IgnoresStandaloneFilesInAnySkillsFolder(t *testing.T) {
	files := filterOrphanSkillFiles(Skill{Identifier: "real-skill", Title: "real-skill"}, []SkillFile{
		{Path: ".cursor/skills/standalone-file", Content: "ignored"},
		{Path: ".cursor/skills/port/standalone-file", Content: "ignored"},
		{Path: ".cursor/skills/engineering/real-skill/SKILL.md", Content: "kept"},
		{Path: ".cursor/skills/real-skill/references/guide.md", Content: "also kept"},
		{Path: "scripts/run.sh", Content: "relative path kept"},
	})

	if len(files) != 3 {
		t.Fatalf("want 3 files, got %+v", files)
	}
	if files[0].Path != ".cursor/skills/engineering/real-skill/SKILL.md" {
		t.Fatalf("unexpected first file: %+v", files[0])
	}
	if files[1].Path != ".cursor/skills/real-skill/references/guide.md" {
		t.Fatalf("unexpected second file: %+v", files[1])
	}
	if files[2].Path != "scripts/run.sh" {
		t.Fatalf("unexpected third file: %+v", files[2])
	}
}

func TestLoadLatestVersionFiles_FiltersSourcePathsAsVersioned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		switch r.URL.Path {
		case "/blueprints/skill_version/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{
						"identifier": "deploy-helper@v1",
						"relations": map[string]interface{}{
							"skill_version_to_skill": map[string]interface{}{"identifier": "org/platform/deploy-helper"},
						},
						"properties": map[string]interface{}{"version": "1"},
					},
				},
			})
		case "/blueprints/skill_file/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{
						"relations": map[string]interface{}{
							"skill_file_to_skill_version": map[string]interface{}{"identifier": "deploy-helper@v1"},
						},
						"properties": map[string]interface{}{
							"path":    ".cursor/skills/engineering/Deploy Helper/SKILL.md",
							"content": "versioned content",
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	skills, err := LoadLatestVersionFiles(context.Background(), client, []Skill{
		{Identifier: "org/platform/deploy-helper", Title: "Deploy Helper"},
	})
	if err != nil {
		t.Fatalf("LoadLatestVersionFiles: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 enriched skill, got %d", len(skills))
	}
	if !skills[0].Versioned {
		t.Fatal("expected skill to be marked versioned")
	}
	if len(skills[0].Files) != 1 {
		t.Fatalf("expected source-style file to be kept, got %+v", skills[0].Files)
	}
}

func TestLoadLatestVersionFiles_RefetchesLegacyContentWhenVersionBlueprintMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		switch r.URL.Path {
		case "/blueprints/skill_version/entities/search":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"error":   "blueprint_not_found",
				"message": "Blueprint skill_version does not exist",
			})
		case "/blueprints/skill/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{
						"identifier": "legacy",
						"title":      "Legacy",
						"properties": map[string]interface{}{
							"instructions": "legacy instructions",
							"references": []interface{}{
								map[string]interface{}{"path": "references/guide.md", "content": "guide"},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	skills, err := LoadLatestVersionFiles(context.Background(), client, []Skill{
		{Identifier: "legacy", Title: "Legacy", GroupIDs: []string{"group"}},
	})
	if err != nil {
		t.Fatalf("LoadLatestVersionFiles: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 legacy skill, got %+v", skills)
	}
	if skills[0].Instructions != "legacy instructions" {
		t.Fatalf("expected legacy instructions to be restored, got %+v", skills[0])
	}
	if len(skills[0].References) != 1 {
		t.Fatalf("expected legacy references to be restored, got %+v", skills[0])
	}
}

func TestLoadSyncableFetchedSkills_DropsVersionedSkillsWithoutContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		switch r.URL.Path {
		case "/blueprints/skill_version/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{
						"identifier": "required@v1",
						"relations": map[string]interface{}{
							"skill_version_to_skill": map[string]interface{}{"identifier": "required"},
						},
						"properties": map[string]interface{}{"version": "1"},
					},
					{
						"identifier": "placeholder@v1",
						"relations": map[string]interface{}{
							"skill_version_to_skill": map[string]interface{}{"identifier": "placeholder"},
						},
						"properties": map[string]interface{}{"version": "1"},
					},
				},
			})
		case "/blueprints/skill_file/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{
						"relations": map[string]interface{}{
							"skill_file_to_skill_version": map[string]interface{}{"identifier": "required@v1"},
						},
						"properties": map[string]interface{}{"path": "SKILL.md", "content": "content"},
					},
					{
						"relations": map[string]interface{}{
							"skill_file_to_skill_version": map[string]interface{}{"identifier": "placeholder@v1"},
						},
						"properties": map[string]interface{}{"path": ".cursor/skills/port/.gitkeep", "content": ""},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "group", Required: true, SkillIDs: []string{"required"}}},
		Required: []Skill{
			{Identifier: "required", Title: "Required", Required: true, GroupIDs: []string{"group"}},
			{Identifier: "placeholder", Title: "placeholder"},
		},
	}

	syncable, err := LoadSyncableFetchedSkills(context.Background(), client, fetched)
	if err != nil {
		t.Fatalf("LoadSyncableFetchedSkills: %v", err)
	}
	if len(syncable.Required) != 1 {
		t.Fatalf("expected only 1 syncable required skill, got %+v", syncable.Required)
	}
	if syncable.Required[0].Identifier != "required" {
		t.Fatalf("expected required skill to remain, got %+v", syncable.Required)
	}
	if len(syncable.Groups) != 1 || syncable.Groups[0].Identifier != "group" {
		t.Fatalf("expected only groups with syncable skills to remain, got %+v", syncable.Groups)
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

func TestCompareVersionStrings_PreReleaseIsLower(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3-alpha", "1.2.3", -1},      // pre-release < release
		{"1.2.3", "1.2.3-alpha", 1},       // release > pre-release
		{"1.2.3-alpha", "1.2.3-beta", -1}, // alpha < beta
		{"1.2.3-beta", "1.2.3-alpha", 1},  // beta > alpha
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.10.0", "1.9.0", 1},
	}
	for _, tc := range cases {
		got := compareVersionStrings(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("compareVersionStrings(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

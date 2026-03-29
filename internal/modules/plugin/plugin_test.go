package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
)

// --- helpers ---

func newTestModule(t *testing.T) (*Module, *config.ConfigManager, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cm := config.NewConfigManager(cfgPath)
	client := api.NewClient(api.ClientOpts{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "https://api.getport.io/v1",
	})
	mod := &Module{client: client, configManager: cm}
	return mod, cm, dir
}

func writeCfg(t *testing.T, cm *config.ConfigManager, cfg *config.PluginConfig) {
	t.Helper()
	if err := cm.SavePluginConfig(cfg); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}
}

func identifiers(skills []Skill) []string {
	ids := make([]string, len(skills))
	for i, s := range skills {
		ids[i] = s.Identifier
	}
	return ids
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsStr(body, substr string) bool {
	return strings.Contains(body, substr)
}

// --- DefaultHookTargets ---

func TestDefaultHookTargets_ReturnsExpectedTools(t *testing.T) {
	targets := DefaultHookTargets()
	if len(targets) == 0 {
		t.Fatal("expected at least one default hook target")
	}
	names := make([]string, len(targets))
	for i, tg := range targets {
		names[i] = tg.Name
	}
	for _, expected := range []string{"Cursor", "Claude Code", "Gemini CLI", "OpenAI Codex", "Windsurf", "Agents", "GitHub Copilot"} {
		if !contains(names, expected) {
			t.Errorf("expected %q in default targets", expected)
		}
	}

	for _, tg := range targets {
		if tg.Name == "GitHub Copilot" {
			if !tg.RepoScoped {
				t.Error("expected GitHub Copilot to be repo-scoped")
			}
			if tg.Note == "" {
				t.Error("expected GitHub Copilot to have a note")
			}
		}
	}
}

// --- TargetPaths ---

func TestTargetPaths_ResolvesPaths(t *testing.T) {
	targets := []HookTarget{
		{Name: "Cursor", Dir: ".cursor"},
		{Name: "Claude Code", Dir: ".claude"},
	}
	paths := TargetPaths(targets, "/home/user", "/home/user")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if !strings.HasSuffix(paths[0], ".cursor") {
		t.Errorf("expected path ending in .cursor, got %s", paths[0])
	}
	if !strings.HasSuffix(paths[1], ".claude") {
		t.Errorf("expected path ending in .claude, got %s", paths[1])
	}
}

func TestTargetPaths_RepoScopedUsesRepoRoot(t *testing.T) {
	targets := []HookTarget{
		{Name: "Global", Dir: ".cursor"},
		{Name: "Repo", Dir: ".github/hooks", RepoScoped: true},
	}
	paths := TargetPaths(targets, "/home/user", "/repo/root")

	if paths[0] != "/home/user/.cursor" {
		t.Errorf("global target: expected /home/user/.cursor, got %s", paths[0])
	}
	if paths[1] != "/repo/root/.github/hooks" {
		t.Errorf("repo target: expected /repo/root/.github/hooks, got %s", paths[1])
	}
}

// --- FilterSkills ---

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
				Optional: []Skill{
					{Identifier: "opt-1", GroupID: "group-a"},
					{Identifier: "opt-2"},
				},
			},
			selectAll: true,
			wantIDs:   []string{"req-1", "opt-1", "opt-2"},
		},
		{
			name: "required always included even with no selection",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{{Identifier: "opt-1", GroupID: "group-a"}},
			},
			wantIDs: []string{"req-1"},
		},
		{
			name: "SelectAllGroups includes grouped only",
			fetched: &FetchedSkills{
				Required: []Skill{{Identifier: "req-1", Required: true}},
				Optional: []Skill{
					{Identifier: "opt-grouped", GroupID: "group-a"},
					{Identifier: "opt-ungrouped"},
				},
			},
			selectAllGroups: true,
			wantIDs:         []string{"req-1", "opt-grouped"},
		},
		{
			name: "SelectAllUngrouped includes ungrouped only",
			fetched: &FetchedSkills{
				Optional: []Skill{
					{Identifier: "grouped", GroupID: "group-a"},
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
					{Identifier: "skill-a", GroupID: "group-a"},
					{Identifier: "skill-b", GroupID: "group-b"},
					{Identifier: "skill-c", GroupID: "group-c"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterSkills(tt.fetched, tt.selectAll, tt.selectAllGroups, tt.selectAllUngrouped, tt.selectedGroups, tt.selectedSkills)
			ids := identifiers(result)
			if len(ids) != len(tt.wantIDs) {
				t.Fatalf("expected %d skills, got %d: %v", len(tt.wantIDs), len(ids), ids)
			}
			for _, want := range tt.wantIDs {
				if !contains(ids, want) {
					t.Errorf("expected %q in result, got %v", want, ids)
				}
			}
		})
	}
}

// --- ParseFetchedSkills ---

func TestParseFetchedSkills_GroupRelationAndRequired(t *testing.T) {
	groupEntities := []api.Entity{
		{
			"identifier": "group-required",
			"title":      "Required Group",
			"properties": map[string]interface{}{
				"enforcement": "required",
			},
			"relations": map[string]interface{}{
				"skills": []interface{}{"skill-a", "skill-b"},
			},
		},
		{
			"identifier": "group-optional",
			"title":      "Optional Group",
			"properties": map[string]interface{}{
				"enforcement": "optional",
			},
			"relations": map[string]interface{}{
				"skills": []interface{}{"skill-c"},
			},
		},
	}

	skillEntities := []api.Entity{
		{"identifier": "skill-a", "title": "Skill A", "properties": map[string]interface{}{"instructions": "do a"}},
		{"identifier": "skill-b", "title": "Skill B", "properties": map[string]interface{}{"instructions": "do b"}},
		{"identifier": "skill-c", "title": "Skill C", "properties": map[string]interface{}{"instructions": "do c"}},
	}

	fetched := ParseFetchedSkills(groupEntities, skillEntities)

	if len(fetched.Required) != 2 {
		t.Errorf("expected 2 required skills, got %d", len(fetched.Required))
	}
	if len(fetched.Optional) != 1 {
		t.Errorf("expected 1 optional skill, got %d", len(fetched.Optional))
	}

	reqIDs := identifiers(fetched.Required)
	if !contains(reqIDs, "skill-a") || !contains(reqIDs, "skill-b") {
		t.Error("expected skill-a and skill-b as required")
	}
	if fetched.Optional[0].Identifier != "skill-c" {
		t.Errorf("expected skill-c as optional, got %s", fetched.Optional[0].Identifier)
	}

	for _, s := range fetched.Required {
		if s.GroupID != "group-required" {
			t.Errorf("expected group-required for %s, got %s", s.Identifier, s.GroupID)
		}
	}
}

func TestParseFetchedSkills_UngroupedSkill(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "ungrouped-skill",
			"title":      "Ungrouped",
			"properties": map[string]interface{}{"instructions": "do it"},
		},
	}

	fetched := ParseFetchedSkills(nil, skillEntities)

	if len(fetched.Optional) != 1 {
		t.Fatalf("expected 1 optional skill, got %d", len(fetched.Optional))
	}
	if fetched.Optional[0].GroupID != "" {
		t.Errorf("expected empty GroupID, got %s", fetched.Optional[0].GroupID)
	}
}

func TestParseFetchedSkills_ReferencesAndAssets(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "skill-with-files",
			"title":      "Skill With Files",
			"properties": map[string]interface{}{
				"instructions": "do it",
				"references":   []interface{}{map[string]interface{}{"path": "refs/guide.md", "content": "# Guide"}},
				"assets":       []interface{}{map[string]interface{}{"path": "assets/tpl.yaml", "content": "key: value"}},
			},
		},
	}

	fetched := ParseFetchedSkills(nil, skillEntities)

	s := fetched.Optional[0]
	if len(s.References) != 1 || s.References[0].Path != "refs/guide.md" {
		t.Errorf("unexpected references: %+v", s.References)
	}
	if len(s.Assets) != 1 || s.Assets[0].Path != "assets/tpl.yaml" {
		t.Errorf("unexpected assets: %+v", s.Assets)
	}
}

// --- InstallHooks ---

func TestInstallHooks_WritesJSONHook(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Test", Dir: "tooldir", Format: hookFormatJSON}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "tooldir", "hooks.json"))
	body := string(data)
	if !containsStr(body, "sessionStart") {
		t.Error("hooks.json missing sessionStart key")
	}
	if !containsStr(body, hookCommand) {
		t.Errorf("hooks.json should contain command %q", hookCommand)
	}
}

func TestInstallHooks_WritesClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Claude", Dir: "claudedir", Format: hookFormatClaude}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "claudedir", "settings.json"))
	body := string(data)
	if !containsStr(body, "UserPromptSubmit") {
		t.Error("settings.json missing UserPromptSubmit key")
	}
	if !containsStr(body, hookCommand) {
		t.Errorf("settings.json should contain command %q", hookCommand)
	}
}

func TestInstallHooks_WritesGeminiSettings(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Gemini", Dir: "geminidir", Format: hookFormatGemini}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "geminidir", "settings.json"))
	body := string(data)
	if !containsStr(body, "SessionStart") {
		t.Error("settings.json missing SessionStart key")
	}
	if !containsStr(body, hookCommand) {
		t.Errorf("settings.json should contain command %q", hookCommand)
	}
}

func TestInstallHooks_WritesWindsurfHook(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Windsurf", Dir: "windsurfdir", Format: hookFormatWindsurf}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "windsurfdir", "hooks.json"))
	body := string(data)
	if !containsStr(body, "pre_user_prompt") {
		t.Error("hooks.json missing pre_user_prompt key")
	}
	if !containsStr(body, hookCommand) {
		t.Errorf("hooks.json should contain command %q", hookCommand)
	}
}

func TestInstallHooks_RepoScopedTarget(t *testing.T) {
	homeDir := t.TempDir()
	repoDir := t.TempDir()

	targets := []HookTarget{
		{Name: "Copilot", Dir: ".github/hooks", Format: hookFormatJSON, RepoScoped: true},
	}

	if err := InstallHooks(targets, homeDir, repoDir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	repoPath := filepath.Join(repoDir, ".github", "hooks", "hooks.json")
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("expected hooks.json in repoDir at %s", repoPath)
	}

	homePath := filepath.Join(homeDir, ".github", "hooks", "hooks.json")
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Error("repo-scoped hook should not be written to home directory")
	}
}

func TestInstallHooks_MergesExistingJSONHook(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := `{"version":1,"hooks":{"preCommit":[{"command":"lint"}]}}`
	if err := os.WriteFile(filepath.Join(toolDir, "hooks.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Tool", Dir: "tool", Format: hookFormatJSON}}
	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(toolDir, "hooks.json"))
	body := string(data)
	if !containsStr(body, "preCommit") {
		t.Error("existing preCommit hook was lost after merge")
	}
	if !containsStr(body, "sessionStart") {
		t.Error("sessionStart hook not added")
	}
}

// --- RemoveHooks ---

func TestRemoveHooks_RemovesJSONHookEntry(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Tool", Dir: "tooldir", Format: hookFormatJSON}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	hookFile := filepath.Join(dir, "tooldir", "hooks.json")
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Error("hooks.json should have been deleted when empty")
	}
}

func TestRemoveHooks_PreservesOtherJSONHooks(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "tooldir")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Tool", Dir: "tooldir", Format: hookFormatJSON}}
	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatal(err)
	}

	hookFile := filepath.Join(toolDir, "hooks.json")
	raw := map[string]interface{}{}
	data, _ := os.ReadFile(hookFile)
	_ = json.Unmarshal(data, &raw)
	hooks := raw["hooks"].(map[string]interface{})
	hooks["preCommit"] = []map[string]string{{"command": "./lint.sh"}}
	hooks["sessionStart"] = append(
		hooks["sessionStart"].([]interface{}),
		map[string]string{"command": "./other-session.sh"},
	)
	out, _ := json.Marshal(raw)
	_ = os.WriteFile(hookFile, out, 0o644)

	_, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}

	result, _ := os.ReadFile(hookFile)
	body := string(result)

	if !containsStr(body, "preCommit") {
		t.Error("preCommit hook should have been preserved")
	}
	if !containsStr(body, "other-session.sh") {
		t.Error("other sessionStart entry should have been preserved")
	}
	if containsStr(body, hookCommand) {
		t.Error("Port hook command should have been removed")
	}
}

func TestRemoveHooks_RemovesClaudeHookEntry(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Claude", Dir: "claudedir", Format: hookFormatClaude}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	if _, err := os.Stat(filepath.Join(dir, "claudedir", "settings.json")); !os.IsNotExist(err) {
		t.Error("settings.json should have been deleted when empty")
	}
}

func TestRemoveHooks_PreservesOtherClaudeHooks(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, "claudedir")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := `{"hooks": [{"matcher": "PreToolUse", "hooks": [{"type": "command", "command": "./lint.sh"}]}]}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Claude", Dir: "claudedir", Format: hookFormatClaude}}
	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatal(err)
	}

	_, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	body := string(data)

	if !containsStr(body, "PreToolUse") {
		t.Error("unrelated PreToolUse hook should have been preserved")
	}
	if containsStr(body, hookCommand) {
		t.Error("Port hook command should have been removed")
	}
}

func TestRemoveHooks_RemovesGeminiHookEntry(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Gemini", Dir: "geminidir", Format: hookFormatGemini}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	if _, err := os.Stat(filepath.Join(dir, "geminidir", "settings.json")); !os.IsNotExist(err) {
		t.Error("settings.json should have been deleted when empty")
	}
}

func TestRemoveHooks_RemovesWindsurfHookEntry(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Windsurf", Dir: "wsdir", Format: hookFormatWindsurf}}

	if err := InstallHooks(targets, dir, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	if _, err := os.Stat(filepath.Join(dir, "wsdir", "hooks.json")); !os.IsNotExist(err) {
		t.Error("hooks.json should have been deleted when empty")
	}
}

func TestRemoveHooks_SkipsWhenNoHookFile(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Tool", Dir: "nonexistent", Format: hookFormatJSON}}

	result, err := RemoveHooks(targets, dir, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

// --- WriteSkills ---

func TestWriteSkills_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	skills := []Skill{
		{
			Identifier:   "my-skill",
			Title:        "My Skill",
			Description:  "does stuff",
			Instructions: "step 1\nstep 2\n",
			GroupID:      "my-group",
		},
	}

	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	skillMD := filepath.Join(dir, "skills", PortSkillsDir, "my-group", "my-skill", "SKILL.md")
	content, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	body := string(content)
	if !containsStr(body, "name: my-skill") {
		t.Error("SKILL.md missing name frontmatter")
	}
	if !containsStr(body, "description: does stuff") {
		t.Error("SKILL.md missing description frontmatter")
	}
	if !containsStr(body, "step 1") {
		t.Error("SKILL.md missing instructions")
	}
}

func TestWriteSkills_UngroupedUsesNoGroupDir(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{{Identifier: "solo-skill", Title: "Solo"}}

	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	expected := filepath.Join(dir, "skills", PortSkillsDir, NoGroupDir, "solo-skill", "SKILL.md")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected SKILL.md at %s", expected)
	}
}

func TestWriteSkills_WritesReferencesAndAssets(t *testing.T) {
	dir := t.TempDir()

	skills := []Skill{
		{
			Identifier:   "skill-files",
			GroupID:      "grp",
			Instructions: "do it",
			References:   []SkillFile{{Path: "references/guide.md", Content: "# Guide"}},
			Assets:       []SkillFile{{Path: "assets/config.yaml", Content: "key: value"}},
		},
	}

	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	refPath := filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files", "references", "guide.md")
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		t.Errorf("reference file not created at %s", refPath)
	}

	assetPath := filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files", "assets", "config.yaml")
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		t.Errorf("asset file not created at %s", assetPath)
	}
}

func TestWriteSkills_MultipleTargets(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	skills := []Skill{{Identifier: "sk", GroupID: "g", Instructions: "x"}}

	if err := WriteSkills(skills, nil, []string{dir1, dir2}, nil); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	for _, base := range []string{dir1, dir2} {
		p := filepath.Join(base, "skills", PortSkillsDir, "g", "sk", "SKILL.md")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SKILL.md not created in target %s", base)
		}
	}
}

func TestWriteSkills_ReconcileRemovesStaleSkill(t *testing.T) {
	dir := t.TempDir()

	initial := []Skill{
		{Identifier: "keep", GroupID: "grp", Instructions: "x"},
		{Identifier: "stale", GroupID: "grp", Instructions: "y"},
	}
	if err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills error: %v", err)
	}

	staleDir := filepath.Join(dir, "skills", PortSkillsDir, "grp", "stale")
	if _, err := os.Stat(staleDir); os.IsNotExist(err) {
		t.Fatal("stale skill directory should exist before reconcile")
	}

	updated := []Skill{{Identifier: "keep", GroupID: "grp", Instructions: "x"}}
	if err := WriteSkills(updated, nil, []string{dir}, nil); err != nil {
		t.Fatalf("second WriteSkills error: %v", err)
	}

	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("stale skill directory should have been removed")
	}

	keepFile := filepath.Join(dir, "skills", PortSkillsDir, "grp", "keep", "SKILL.md")
	if _, err := os.Stat(keepFile); os.IsNotExist(err) {
		t.Error("kept skill SKILL.md should still exist")
	}
}

func TestWriteSkills_ReconcileRemovesEmptyGroup(t *testing.T) {
	dir := t.TempDir()

	initial := []Skill{{Identifier: "sk", GroupID: "gone-group", Instructions: "x"}}
	if err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills error: %v", err)
	}

	if err := WriteSkills(nil, nil, []string{dir}, nil); err != nil {
		t.Fatalf("second WriteSkills error: %v", err)
	}

	groupDir := filepath.Join(dir, "skills", PortSkillsDir, "gone-group")
	if _, err := os.Stat(groupDir); !os.IsNotExist(err) {
		t.Error("empty group directory should have been removed")
	}
}

// --- WriteSkills location routing ---

func TestWriteSkills_ProjectSkillGoesToProjectDir(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	skills := []Skill{
		{Identifier: "proj-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationProject},
	}

	if err := WriteSkills(skills, nil, []string{globalDir}, []string{projectDir}); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	projPath := filepath.Join(projectDir, "skills", PortSkillsDir, "grp", "proj-skill", "SKILL.md")
	if _, err := os.Stat(projPath); os.IsNotExist(err) {
		t.Error("project-scoped skill not written to projectDir")
	}

	globalPath := filepath.Join(globalDir, "skills", PortSkillsDir, "grp", "proj-skill", "SKILL.md")
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Error("project-scoped skill should not be written to globalDir")
	}
}

func TestWriteSkills_GlobalSkillGoesToGlobalTargets(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	skills := []Skill{
		{Identifier: "global-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationGlobal},
	}

	if err := WriteSkills(skills, nil, []string{globalDir}, []string{projectDir}); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	globalPath := filepath.Join(globalDir, "skills", PortSkillsDir, "grp", "global-skill", "SKILL.md")
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		t.Error("global-scoped skill not written to globalDir")
	}

	projPath := filepath.Join(projectDir, "skills", PortSkillsDir, "grp", "global-skill", "SKILL.md")
	if _, err := os.Stat(projPath); !os.IsNotExist(err) {
		t.Error("global-scoped skill should not be written to projectDir")
	}
}

func TestWriteSkills_DefaultLocationIsGlobal(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	skills := []Skill{{Identifier: "default-skill", GroupID: "grp", Instructions: "x"}}

	if err := WriteSkills(skills, nil, []string{globalDir}, []string{projectDir}); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	globalPath := filepath.Join(globalDir, "skills", PortSkillsDir, "grp", "default-skill", "SKILL.md")
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		t.Error("skill with default location should be written to globalDir")
	}
}

func TestWriteSkills_ProjectSkillSkippedWhenNoCwd(t *testing.T) {
	globalDir := t.TempDir()

	skills := []Skill{
		{Identifier: "proj-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationProject},
	}

	if err := WriteSkills(skills, nil, []string{globalDir}, nil); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	globalPath := filepath.Join(globalDir, "skills", PortSkillsDir, "grp", "proj-skill", "SKILL.md")
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Error("project-scoped skill should not be written when projectDir is empty")
	}
}

// --- parseSkillLocation ---

func TestParseSkillLocation(t *testing.T) {
	tests := []struct {
		input string
		want  SkillLocation
	}{
		{"project", SkillLocationProject},
		{"global", SkillLocationGlobal},
		{"", SkillLocationGlobal},
		{"something-else", SkillLocationGlobal},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseSkillLocation(tt.input); got != tt.want {
				t.Errorf("parseSkillLocation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- buildSkillMD ---

func TestBuildSkillMD_WithInstructions(t *testing.T) {
	s := Skill{
		Identifier:   "my-skill",
		Description:  "does things",
		Instructions: "step 1\nstep 2",
	}
	md := buildSkillMD(s)

	if !containsStr(md, "name: my-skill") {
		t.Error("missing name in frontmatter")
	}
	if !containsStr(md, "description: does things") {
		t.Error("missing description in frontmatter")
	}
	if !containsStr(md, "step 1") {
		t.Error("missing instructions")
	}
}

func TestBuildSkillMD_NoInstructionsFallback(t *testing.T) {
	s := Skill{Identifier: "empty", Title: "Empty Skill"}
	md := buildSkillMD(s)

	if !containsStr(md, "_No instructions provided._") {
		t.Error("expected fallback text for missing instructions")
	}
}

// --- GroupName ---

func TestGroupName(t *testing.T) {
	groups := []SkillGroup{
		{Identifier: "grp-1", Title: "My Group"},
		{Identifier: "grp-2", Title: ""},
	}

	tests := []struct {
		name    string
		groupID string
		want    string
	}{
		{"returns title", "grp-1", "My Group"},
		{"falls back to identifier", "grp-2", "grp-2"},
		{"unknown group returns ID", "unknown", "unknown"},
		{"empty ID returns NoGroupDir", "", NoGroupDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupName(groups, tt.groupID); got != tt.want {
				t.Errorf("GroupName(%q) = %q, want %q", tt.groupID, got, tt.want)
			}
		})
	}
}

// --- Module integration ---

func TestModule_Init_InstallsHooksAndSavesConfig(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)

	targets := []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON},
		{Name: "Agents", Dir: ".agents", Format: hookFormatJSON},
	}

	if err := InstallHooks(targets, tmpDir, tmpDir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}
	targetPaths := TargetPaths(targets, tmpDir, tmpDir)
	writeCfg(t, cm, &config.PluginConfig{Targets: targetPaths})

	for _, dir := range []string{".cursor", ".agents"} {
		p := filepath.Join(tmpDir, dir, "hooks.json")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("hooks.json missing in %s", dir)
		}
	}

	cfg, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig after Init: %v", err)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets in config, got %d", len(cfg.Targets))
	}
}

func TestModule_Remove_ClearsEverything(t *testing.T) {
	mod, cm, baseDir := newTestModule(t)

	cursorDir := filepath.Join(baseDir, ".cursor")
	targets := []HookTarget{{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON}}
	if err := InstallHooks(targets, baseDir, baseDir); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(cursorDir, "skills", PortSkillsDir, "grp", "sk")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# skill"), 0o644)

	writeCfg(t, cm, &config.PluginConfig{Targets: []string{cursorDir}})

	hooksResult, err := RemoveHooks(targets, baseDir, baseDir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(hooksResult.RemovedFrom) != 1 {
		t.Errorf("expected 1 hook removal, got %d", len(hooksResult.RemovedFrom))
	}

	skillsResult, err := mod.ClearSkills()
	if err != nil {
		t.Fatalf("ClearSkills error: %v", err)
	}
	if len(skillsResult.DeletedTargets) != 1 {
		t.Errorf("expected 1 skills target cleared, got %d", len(skillsResult.DeletedTargets))
	}

	if err := cm.SavePluginConfig(&config.PluginConfig{}); err != nil {
		t.Fatalf("SavePluginConfig error: %v", err)
	}
	loaded, _ := cm.LoadPluginConfig()
	if len(loaded.Targets) != 0 {
		t.Error("plugin config should be empty after remove")
	}
}

func TestModule_ClearSkills_RemovesPortDir(t *testing.T) {
	mod, cm, tmpDir := newTestModule(t)

	portDir := filepath.Join(tmpDir, "skills", PortSkillsDir)
	if err := os.MkdirAll(filepath.Join(portDir, "group-a", "skill-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeCfg(t, cm, &config.PluginConfig{Targets: []string{tmpDir}})

	result, err := mod.ClearSkills()
	if err != nil {
		t.Fatalf("ClearSkills error: %v", err)
	}
	if len(result.DeletedTargets) != 1 {
		t.Errorf("expected 1 deleted target, got %d", len(result.DeletedTargets))
	}
	if _, err := os.Stat(portDir); !os.IsNotExist(err) {
		t.Error("expected skills/port/ to be deleted")
	}
}

func TestModule_ClearSkills_AlsoClearsProjectDirs(t *testing.T) {
	mod, cm, _ := newTestModule(t)

	targetDir := t.TempDir()
	projectDir := t.TempDir()

	targetPortDir := filepath.Join(targetDir, "skills", PortSkillsDir)
	if err := os.MkdirAll(filepath.Join(targetPortDir, "grp", "sk"), 0o755); err != nil {
		t.Fatal(err)
	}

	projectPortDir := filepath.Join(projectDir, "skills", PortSkillsDir)
	if err := os.MkdirAll(filepath.Join(projectPortDir, "grp", "sk"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeCfg(t, cm, &config.PluginConfig{
		Targets:     []string{targetDir},
		ProjectDirs: []string{projectDir},
	})

	result, err := mod.ClearSkills()
	if err != nil {
		t.Fatalf("ClearSkills error: %v", err)
	}
	if len(result.DeletedTargets) != 2 {
		t.Errorf("expected 2 deleted targets (1 AI tool + 1 project), got %d", len(result.DeletedTargets))
	}
	if _, err := os.Stat(targetPortDir); !os.IsNotExist(err) {
		t.Error("expected skills/port/ deleted from AI tool target")
	}
	if _, err := os.Stat(projectPortDir); !os.IsNotExist(err) {
		t.Error("expected skills/port/ deleted from project dir")
	}
}

func TestModule_ClearSkills_SkipsMissingDir(t *testing.T) {
	mod, cm, tmpDir := newTestModule(t)
	writeCfg(t, cm, &config.PluginConfig{Targets: []string{tmpDir}})

	result, err := mod.ClearSkills()
	if err != nil {
		t.Fatalf("ClearSkills error: %v", err)
	}
	if len(result.DeletedTargets) != 0 {
		t.Errorf("expected 0 deleted targets, got %d", len(result.DeletedTargets))
	}
	if len(result.SkippedTargets) != 1 {
		t.Errorf("expected 1 skipped target, got %d", len(result.SkippedTargets))
	}
}

func TestModule_Status_ReturnsConfigValues(t *testing.T) {
	mod, cm, _ := newTestModule(t)

	writeCfg(t, cm, &config.PluginConfig{
		Targets:         []string{"/home/user/.cursor"},
		SelectAllGroups: true,
		LastSyncedAt:    "2026-03-25T10:00:00Z",
	})

	status, err := mod.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}

	if len(status.Targets) != 1 || status.Targets[0] != "/home/user/.cursor" {
		t.Errorf("Targets: got %v", status.Targets)
	}
	if !status.SelectAllGroups {
		t.Error("SelectAllGroups should be true")
	}
	if status.LastSyncedAt != "2026-03-25T10:00:00Z" {
		t.Errorf("LastSyncedAt: got %s", status.LastSyncedAt)
	}
}

// --- SavePluginConfig / LoadPluginConfig ---

func TestSaveAndLoadPluginConfig(t *testing.T) {
	_, cm, _ := newTestModule(t)

	cfg := &config.PluginConfig{
		Targets:            []string{"/home/user/.cursor", "/home/user/.claude"},
		SelectAllGroups:    true,
		SelectAllUngrouped: false,
		SelectedSkills:     []string{"skill-x"},
		LastSyncedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	writeCfg(t, cm, cfg)

	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig error: %v", err)
	}

	if len(loaded.Targets) != 2 {
		t.Errorf("Targets mismatch: got %d", len(loaded.Targets))
	}
	if !loaded.SelectAllGroups {
		t.Error("SelectAllGroups should be true")
	}
	if loaded.SelectAllUngrouped {
		t.Error("SelectAllUngrouped should be false")
	}
	if len(loaded.SelectedSkills) != 1 || loaded.SelectedSkills[0] != "skill-x" {
		t.Errorf("SelectedSkills mismatch: got %v", loaded.SelectedSkills)
	}
}

func TestSavePluginConfig_PreservesOtherFields(t *testing.T) {
	_, cm, _ := newTestModule(t)

	if err := cm.SavePluginConfig(&config.PluginConfig{}); err != nil {
		t.Fatal(err)
	}

	if err := cm.SavePluginConfig(&config.PluginConfig{SelectAll: true}); err != nil {
		t.Fatalf("second SavePluginConfig error: %v", err)
	}

	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig error: %v", err)
	}
	if !loaded.SelectAll {
		t.Error("expected SelectAll=true")
	}
}

// --- PluginConfig.HasSelection ---

func TestPluginConfig_HasSelection(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.PluginConfig
		want bool
	}{
		{"empty", config.PluginConfig{}, false},
		{"targets set", config.PluginConfig{Targets: []string{"/foo"}}, true},
		{"select all", config.PluginConfig{SelectAll: true}, true},
		{"select all groups", config.PluginConfig{SelectAllGroups: true}, true},
		{"selected skills", config.PluginConfig{SelectedSkills: []string{"s"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasSelection(); got != tt.want {
				t.Errorf("HasSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}

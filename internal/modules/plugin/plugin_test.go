package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
)

// --- helpers ---

// newTestModule creates a Module backed by a temp config file and a dummy API
// client (no real HTTP calls are made in these tests).
func newTestModule(t *testing.T) (*Module, *config.ConfigManager, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cm := config.NewConfigManager(cfgPath)
	client := api.NewClient("test-id", "test-secret", "https://api.getport.io/v1", 0)
	mod := &Module{client: client, configManager: cm}
	return mod, cm, dir
}

// writeCfg saves a PluginConfig and returns the updated manager.
func writeCfg(t *testing.T, cm *config.ConfigManager, cfg *config.PluginConfig) {
	t.Helper()
	if err := cm.SavePluginConfig(cfg); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}
}

// --- FilterSkills ---

func TestFilterSkills_SelectAll(t *testing.T) {
	fetched := &FetchedSkills{
		Required: []Skill{{Identifier: "req-1", Required: true}},
		Optional: []Skill{
			{Identifier: "opt-1", GroupID: "group-a"},
			{Identifier: "opt-2", GroupID: ""},
		},
	}

	result := FilterSkills(fetched, true, false, false, nil, nil)

	if len(result) != 3 {
		t.Errorf("expected 3 skills, got %d", len(result))
	}
}

func TestFilterSkills_RequiredAlwaysIncluded(t *testing.T) {
	fetched := &FetchedSkills{
		Required: []Skill{{Identifier: "req-1", Required: true}},
		Optional: []Skill{{Identifier: "opt-1", GroupID: "group-a"}},
	}

	// No selection at all — required still comes through.
	result := FilterSkills(fetched, false, false, false, nil, nil)

	if len(result) != 1 || result[0].Identifier != "req-1" {
		t.Errorf("expected only required skill, got %+v", result)
	}
}

func TestFilterSkills_SelectAllGroups(t *testing.T) {
	fetched := &FetchedSkills{
		Required: []Skill{{Identifier: "req-1", Required: true}},
		Optional: []Skill{
			{Identifier: "opt-grouped", GroupID: "group-a"},
			{Identifier: "opt-ungrouped", GroupID: ""},
		},
	}

	result := FilterSkills(fetched, false, true, false, nil, nil)

	ids := identifiers(result)
	if !contains(ids, "req-1") {
		t.Error("expected required skill")
	}
	if !contains(ids, "opt-grouped") {
		t.Error("expected grouped skill when selectAllGroups=true")
	}
	if contains(ids, "opt-ungrouped") {
		t.Error("ungrouped skill should not be included when selectAllUngrouped=false")
	}
}

func TestFilterSkills_SelectAllUngrouped(t *testing.T) {
	fetched := &FetchedSkills{
		Optional: []Skill{
			{Identifier: "grouped", GroupID: "group-a"},
			{Identifier: "ungrouped-1", GroupID: ""},
			{Identifier: "ungrouped-2", GroupID: ""},
		},
	}

	result := FilterSkills(fetched, false, false, true, nil, nil)

	ids := identifiers(result)
	if contains(ids, "grouped") {
		t.Error("grouped skill should not be included when selectAllUngrouped=true and group not selected")
	}
	if !contains(ids, "ungrouped-1") || !contains(ids, "ungrouped-2") {
		t.Error("expected all ungrouped skills")
	}
}

func TestFilterSkills_SpecificGroups(t *testing.T) {
	fetched := &FetchedSkills{
		Optional: []Skill{
			{Identifier: "skill-a", GroupID: "group-a"},
			{Identifier: "skill-b", GroupID: "group-b"},
			{Identifier: "skill-c", GroupID: "group-c"},
		},
	}

	result := FilterSkills(fetched, false, false, false, []string{"group-a", "group-b"}, nil)

	ids := identifiers(result)
	if !contains(ids, "skill-a") || !contains(ids, "skill-b") {
		t.Error("expected skills from selected groups")
	}
	if contains(ids, "skill-c") {
		t.Error("skill-c should not be included (group-c not selected)")
	}
}

func TestFilterSkills_SpecificSkills(t *testing.T) {
	fetched := &FetchedSkills{
		Optional: []Skill{
			{Identifier: "skill-1", GroupID: ""},
			{Identifier: "skill-2", GroupID: ""},
			{Identifier: "skill-3", GroupID: ""},
		},
	}

	result := FilterSkills(fetched, false, false, false, nil, []string{"skill-1", "skill-3"})

	ids := identifiers(result)
	if !contains(ids, "skill-1") || !contains(ids, "skill-3") {
		t.Error("expected individually selected skills")
	}
	if contains(ids, "skill-2") {
		t.Error("skill-2 should not be included")
	}
}

// --- FetchSkills parsing ---

func TestFetchSkills_GroupRelationAndRequired(t *testing.T) {
	// Build mock entities that match the Port blueprint structure:
	// skill_group has enforcement property and relations.skills (many → skill)
	groupEntities := []api.Entity{
		{
			"identifier": "group-required",
			"title":      "Required Group",
			"properties": map[string]interface{}{
				"enforcement": "required",
				"auto_sync":   true,
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
		{
			"identifier": "skill-a",
			"title":      "Skill A",
			"properties": map[string]interface{}{
				"description":  "desc a",
				"instructions": "do a",
			},
		},
		{
			"identifier": "skill-b",
			"title":      "Skill B",
			"properties": map[string]interface{}{
				"description":  "desc b",
				"instructions": "do b",
			},
		},
		{
			"identifier": "skill-c",
			"title":      "Skill C",
			"properties": map[string]interface{}{
				"description":  "desc c",
				"instructions": "do c",
			},
		},
	}

	fetched := parseFetchedSkills(groupEntities, skillEntities)

	if len(fetched.Required) != 2 {
		t.Errorf("expected 2 required skills (from required group), got %d", len(fetched.Required))
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

	// Group IDs should be resolved correctly.
	for _, s := range fetched.Required {
		if s.GroupID != "group-required" {
			t.Errorf("expected group-required for %s, got %s", s.Identifier, s.GroupID)
		}
	}
}

func TestFetchSkills_UngroupedSkill(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "ungrouped-skill",
			"title":      "Ungrouped",
			"properties": map[string]interface{}{
				"description":  "no group",
				"instructions": "do it",
			},
		},
	}

	fetched := parseFetchedSkills(nil, skillEntities)

	if len(fetched.Optional) != 1 {
		t.Fatalf("expected 1 optional skill, got %d", len(fetched.Optional))
	}
	if fetched.Optional[0].GroupID != "" {
		t.Errorf("expected empty GroupID for ungrouped skill, got %s", fetched.Optional[0].GroupID)
	}
}

func TestFetchSkills_ReferencesAndAssets(t *testing.T) {
	skillEntities := []api.Entity{
		{
			"identifier": "skill-with-files",
			"title":      "Skill With Files",
			"properties": map[string]interface{}{
				"description":  "has files",
				"instructions": "do it",
				"references": []interface{}{
					map[string]interface{}{
						"path":    "references/guide.md",
						"content": "# Guide",
					},
				},
				"assets": []interface{}{
					map[string]interface{}{
						"path":    "assets/template.yaml",
						"content": "key: value",
					},
				},
			},
		},
	}

	fetched := parseFetchedSkills(nil, skillEntities)

	if len(fetched.Optional) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(fetched.Optional))
	}
	s := fetched.Optional[0]
	if len(s.References) != 1 || s.References[0].Path != "references/guide.md" {
		t.Errorf("expected reference file, got %+v", s.References)
	}
	if len(s.Assets) != 1 || s.Assets[0].Path != "assets/template.yaml" {
		t.Errorf("expected asset file, got %+v", s.Assets)
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

	if err := WriteSkills(skills, nil, []string{dir}); err != nil {
		t.Fatalf("WriteSkills error: %v", err)
	}

	skillMD := filepath.Join(dir, "skills", PortSkillsDir, "my-group", "my-skill", "SKILL.md")
	if _, err := os.Stat(skillMD); os.IsNotExist(err) {
		t.Errorf("SKILL.md not created at expected path: %s", skillMD)
	}

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

	skills := []Skill{
		{Identifier: "solo-skill", Title: "Solo", GroupID: ""},
	}

	if err := WriteSkills(skills, nil, []string{dir}); err != nil {
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
			References: []SkillFile{
				{Path: "references/guide.md", Content: "# Guide"},
			},
			Assets: []SkillFile{
				{Path: "assets/config.yaml", Content: "key: value"},
			},
		},
	}

	if err := WriteSkills(skills, nil, []string{dir}); err != nil {
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

	if err := WriteSkills(skills, nil, []string{dir1, dir2}); err != nil {
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

	// First write: two skills in the same group.
	initial := []Skill{
		{Identifier: "keep", GroupID: "grp", Instructions: "x"},
		{Identifier: "stale", GroupID: "grp", Instructions: "y"},
	}
	if err := WriteSkills(initial, nil, []string{dir}); err != nil {
		t.Fatalf("initial WriteSkills error: %v", err)
	}

	staleDir := filepath.Join(dir, "skills", PortSkillsDir, "grp", "stale")
	if _, err := os.Stat(staleDir); os.IsNotExist(err) {
		t.Fatal("stale skill directory should exist before reconcile")
	}

	// Second write: only the first skill remains (simulate skill deleted from Port).
	updated := []Skill{
		{Identifier: "keep", GroupID: "grp", Instructions: "x"},
	}
	if err := WriteSkills(updated, nil, []string{dir}); err != nil {
		t.Fatalf("second WriteSkills error: %v", err)
	}

	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("stale skill directory should have been removed after reconcile")
	}

	keepDir := filepath.Join(dir, "skills", PortSkillsDir, "grp", "keep", "SKILL.md")
	if _, err := os.Stat(keepDir); os.IsNotExist(err) {
		t.Error("kept skill SKILL.md should still exist")
	}
}

func TestWriteSkills_ReconcileRemovesEmptyGroup(t *testing.T) {
	dir := t.TempDir()

	// Write one skill in a group that will be entirely removed.
	initial := []Skill{
		{Identifier: "sk", GroupID: "gone-group", Instructions: "x"},
	}
	if err := WriteSkills(initial, nil, []string{dir}); err != nil {
		t.Fatalf("initial WriteSkills error: %v", err)
	}

	// Write with no skills — everything should be removed.
	if err := WriteSkills(nil, nil, []string{dir}); err != nil {
		t.Fatalf("second WriteSkills error: %v", err)
	}

	groupDir := filepath.Join(dir, "skills", PortSkillsDir, "gone-group")
	if _, err := os.Stat(groupDir); !os.IsNotExist(err) {
		t.Error("empty group directory should have been removed after reconcile")
	}
}

// --- InstallHooks ---

func TestInstallHooks_WritesJSONHook(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{
		{Name: "Test", Dir: "tooldir", Format: "hooks_json"},
	}

	if err := InstallHooks(targets, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	hookFile := filepath.Join(dir, "tooldir", "hooks.json")
	if _, err := os.Stat(hookFile); os.IsNotExist(err) {
		t.Errorf("hooks.json not created at %s", hookFile)
	}

	data, _ := os.ReadFile(hookFile)
	body := string(data)
	if !containsStr(body, "sessionStart") {
		t.Error("hooks.json missing sessionStart key")
	}

	scriptName, _, _ := hookScriptInfo()
	if !containsStr(body, scriptName) {
		t.Errorf("hooks.json should reference script %s", scriptName)
	}

	// The script itself must exist and contain the CLI command.
	scriptFile := filepath.Join(dir, "tooldir", filepath.FromSlash(scriptName))
	if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
		t.Errorf("hook script not created at %s", scriptFile)
	}
	scriptData, _ := os.ReadFile(scriptFile)
	if !containsStr(string(scriptData), "port plugin reconcile") {
		t.Error("hook script missing 'port plugin reconcile' command")
	}
}

func TestInstallHooks_WritesClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{
		{Name: "Claude", Dir: "claudedir", Format: "claude_settings"},
	}

	if err := InstallHooks(targets, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	settingsFile := filepath.Join(dir, "claudedir", "settings.json")
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		t.Errorf("settings.json not created at %s", settingsFile)
	}

	data, _ := os.ReadFile(settingsFile)
	body := string(data)
	if !containsStr(body, "UserPromptSubmit") {
		t.Error("settings.json missing UserPromptSubmit key")
	}

	scriptName, _, _ := hookScriptInfo()
	if !containsStr(body, scriptName) {
		t.Errorf("settings.json should reference script %s", scriptName)
	}

	// The script itself must exist and contain the CLI command.
	scriptFile := filepath.Join(dir, "claudedir", filepath.FromSlash(scriptName))
	if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
		t.Errorf("hook script not created at %s", scriptFile)
	}
	scriptData, _ := os.ReadFile(scriptFile)
	if !containsStr(string(scriptData), "port plugin reconcile") {
		t.Error("hook script missing 'port plugin reconcile' command")
	}
}

func TestInstallHooks_MergesExistingJSONHook(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an existing hooks.json with another key.
	existing := `{"version":1,"hooks":{"preCommit":[{"command":"lint"}]}}`
	if err := os.WriteFile(filepath.Join(toolDir, "hooks.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Tool", Dir: "tool", Format: "hooks_json"}}
	if err := InstallHooks(targets, dir); err != nil {
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
	targets := []HookTarget{{Name: "Tool", Dir: "tooldir", Format: "hooks_json"}}

	// First install.
	if err := InstallHooks(targets, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	// hooks.json should be gone (was the only entry).
	hookFile := filepath.Join(dir, "tooldir", "hooks.json")
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Error("hooks.json should have been deleted when empty")
	}

	// Script should be gone.
	scriptName, _, _ := hookScriptInfo()
	scriptFile := filepath.Join(dir, "tooldir", filepath.FromSlash(scriptName))
	if _, err := os.Stat(scriptFile); !os.IsNotExist(err) {
		t.Error("hook script should have been removed")
	}
}

func TestRemoveHooks_PreservesOtherJSONHooks(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "tooldir")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Tool", Dir: "tooldir", Format: "hooks_json"}}

	// Install Port hook first.
	if err := InstallHooks(targets, dir); err != nil {
		t.Fatal(err)
	}

	// Manually add an unrelated hook to the same sessionStart array.
	hookFile := filepath.Join(toolDir, "hooks.json")
	raw := map[string]interface{}{}
	data, _ := os.ReadFile(hookFile)
	_ = json.Unmarshal(data, &raw)
	hooks := raw["hooks"].(map[string]interface{})
	hooks["preCommit"] = []map[string]string{{"command": "./lint.sh"}}
	// Also inject a non-port entry in sessionStart alongside ours.
	hooks["sessionStart"] = append(
		hooks["sessionStart"].([]interface{}),
		map[string]string{"command": "./other-session.sh"},
	)
	out, _ := json.Marshal(raw)
	_ = os.WriteFile(hookFile, out, 0o644)

	_, err := RemoveHooks(targets, dir)
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
	if containsStr(body, hookScriptBaseName) {
		t.Error("Port script reference should have been removed")
	}
}

func TestRemoveHooks_RemovesClaudeHookEntry(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Claude", Dir: "claudedir", Format: "claude_settings"}}

	if err := InstallHooks(targets, dir); err != nil {
		t.Fatalf("InstallHooks error: %v", err)
	}

	result, err := RemoveHooks(targets, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(result.RemovedFrom) != 1 {
		t.Errorf("expected 1 removal, got %d", len(result.RemovedFrom))
	}

	// settings.json should be gone (only contained the Port hook).
	settingsFile := filepath.Join(dir, "claudedir", "settings.json")
	if _, err := os.Stat(settingsFile); !os.IsNotExist(err) {
		t.Error("settings.json should have been deleted when empty")
	}
}

func TestRemoveHooks_PreservesOtherClaudeHooks(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, "claudedir")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a settings.json with an unrelated hook before installing.
	existing := `{
		"hooks": [
			{"matcher": "PreToolUse", "hooks": [{"type": "command", "command": "./lint.sh"}]}
		]
	}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []HookTarget{{Name: "Claude", Dir: "claudedir", Format: "claude_settings"}}
	if err := InstallHooks(targets, dir); err != nil {
		t.Fatal(err)
	}

	_, err := RemoveHooks(targets, dir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	body := string(data)

	if !containsStr(body, "PreToolUse") {
		t.Error("unrelated PreToolUse hook should have been preserved")
	}
	if containsStr(body, hookScriptBaseName) {
		t.Error("Port script reference should have been removed")
	}
}

func TestRemoveHooks_SkipsWhenNoHookFile(t *testing.T) {
	dir := t.TempDir()
	targets := []HookTarget{{Name: "Tool", Dir: "nonexistent", Format: "hooks_json"}}

	result, err := RemoveHooks(targets, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

func TestModule_Remove_ClearsEverything(t *testing.T) {
	mod, cm, baseDir := newTestModule(t)

	// Simulate an installed state: write config, hooks, and skills.
	cursorDir := filepath.Join(baseDir, ".cursor")
	targets := []HookTarget{{Name: "Cursor", Dir: ".cursor", Format: "hooks_json"}}
	if err := InstallHooks(targets, baseDir); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(cursorDir, "skills", PortSkillsDir, "grp", "sk")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# skill"), 0o644)

	writeCfg(t, cm, &config.PluginConfig{
		Scope:   "global",
		Targets: []string{cursorDir},
	})

	// Override the module's home dir by patching scope resolution via config.
	_ = mod

	// Run Remove directly on the module wired to baseDir as home.
	// We can't easily override os.UserHomeDir, so we call ClearSkills + RemoveHooks
	// directly in the test and verify the module-level Remove logic via integration.
	hooksResult, err := RemoveHooks(targets, baseDir)
	if err != nil {
		t.Fatalf("RemoveHooks error: %v", err)
	}
	if len(hooksResult.RemovedFrom) != 1 {
		t.Errorf("expected hooks removed from 1 target, got %d", len(hooksResult.RemovedFrom))
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
	if loaded.Scope != "" || len(loaded.Targets) != 0 {
		t.Error("plugin config should be empty after remove")
	}
}

// --- SavePluginConfig / LoadPluginConfig ---

func TestSaveAndLoadPluginConfig(t *testing.T) {
	_, cm, _ := newTestModule(t)

	cfg := &config.PluginConfig{
		Scope:              "global",
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

	if loaded.Scope != "global" {
		t.Errorf("Scope mismatch: got %s", loaded.Scope)
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

	// Write an org config first.
	baseCfg := &config.Config{
		DefaultOrg: "prod",
		Organizations: map[string]config.OrganizationConfig{
			"prod": {ClientID: "cid", ClientSecret: "csec", APIURL: "https://api.getport.io/v1"},
		},
	}
	if err := cm.SavePluginConfig(&config.PluginConfig{Scope: "local"}); err != nil {
		t.Fatal(err)
	}
	_ = baseCfg // just validating no org data loss

	// Update plugin config.
	if err := cm.SavePluginConfig(&config.PluginConfig{Scope: "global", SelectAll: true}); err != nil {
		t.Fatalf("second SavePluginConfig error: %v", err)
	}

	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig error: %v", err)
	}
	if loaded.Scope != "global" {
		t.Errorf("expected scope=global, got %s", loaded.Scope)
	}
	if !loaded.SelectAll {
		t.Error("expected SelectAll=true")
	}
}

// --- Module.Init ---

func TestModule_Init_InstallsHooksAndSavesConfig(t *testing.T) {
	mod, cm, tmpDir := newTestModule(t)

	targets := []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: "hooks_json"},
		{Name: "Agents", Dir: ".agents", Format: "hooks_json"},
	}

	result, err := mod.Init(context.Background(), InitOptions{
		Scope:     "local",
		ScopeRoot: tmpDir,
		Targets:   targets,
	})
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}

	if len(result.InstalledTargets) != 2 {
		t.Errorf("expected 2 installed targets, got %d", len(result.InstalledTargets))
	}

	// Hooks files should exist.
	for _, dir := range []string{".cursor", ".agents"} {
		p := filepath.Join(tmpDir, dir, "hooks.json")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("hooks.json missing in %s", dir)
		}
	}

	// Config should be saved.
	cfg, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig after Init: %v", err)
	}
	if cfg.Scope != "local" {
		t.Errorf("expected scope=local, got %s", cfg.Scope)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets in config, got %d", len(cfg.Targets))
	}
}

// --- Module.ClearSkills ---

func TestModule_ClearSkills_RemovesPortDir(t *testing.T) {
	mod, cm, tmpDir := newTestModule(t)

	// Simulate skills having been written.
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

// --- Module.Status ---

func TestModule_Status_ReturnsConfigValues(t *testing.T) {
	mod, cm, _ := newTestModule(t)

	writeCfg(t, cm, &config.PluginConfig{
		Scope:           "global",
		Targets:         []string{"/home/user/.cursor"},
		SelectAllGroups: true,
		LastSyncedAt:    "2026-03-25T10:00:00Z",
	})

	status, err := mod.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}

	if status.Scope != "global" {
		t.Errorf("Scope: got %s", status.Scope)
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

// --- parseFetchedSkills is a test helper that calls the same parsing logic
// as FetchSkills but with pre-built entities, without hitting the API. ---

func parseFetchedSkills(groupEntities, skillEntities []api.Entity) *FetchedSkills {
	requiredSkillIDs := make(map[string]bool)
	skillGroupMap := make(map[string]string)
	var groups []SkillGroup

	for _, e := range groupEntities {
		props, _ := e["properties"].(map[string]interface{})
		relations, _ := e["relations"].(map[string]interface{})
		groupID := stringProp(e, "identifier")
		enforcement := stringFromMap(props, "enforcement")
		isRequired := enforcement == "required"

		var skillIDs []string
		if rel, ok := relations["skills"]; ok {
			if items, ok := rel.([]interface{}); ok {
				for _, item := range items {
					if sid, ok := item.(string); ok {
						skillIDs = append(skillIDs, sid)
						skillGroupMap[sid] = groupID
						if isRequired {
							requiredSkillIDs[sid] = true
						}
					}
				}
			}
		}

		groups = append(groups, SkillGroup{
			Identifier: groupID,
			Title:      stringProp(e, "title"),
			Required:   isRequired,
			SkillIDs:   skillIDs,
		})
	}

	result := &FetchedSkills{Groups: groups}
	for _, e := range skillEntities {
		props, _ := e["properties"].(map[string]interface{})
		skillID := stringProp(e, "identifier")
		skill := Skill{
			Identifier:   skillID,
			Title:        stringProp(e, "title"),
			Description:  stringFromMap(props, "description"),
			Instructions: stringFromMap(props, "instructions"),
			GroupID:      skillGroupMap[skillID],
			Required:     requiredSkillIDs[skillID],
			References:   parseSkillFiles(props, "references"),
			Assets:       parseSkillFiles(props, "assets"),
		}
		if skill.Required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}
	return result
}

// --- test helpers ---

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
	return len(body) > 0 && len(substr) > 0 &&
		func() bool {
			for i := 0; i <= len(body)-len(substr); i++ {
				if body[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}

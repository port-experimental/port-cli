package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestModule_Init_InstallsHooksAndSavesConfig(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	targets := []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON},
		{Name: "GitHub Copilot", Dir: ".github", RepoScoped: true, HookSubDir: "hooks", Format: hookFormatCopilotJSON},
	}
	if err := InstallHooks(targets, tmpDir, tmpDir); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}
	writeCfg(t, cm, &config.SkillsConfig{Targets: TargetPaths(targets, tmpDir, tmpDir)})

	assertFileExists(t, filepath.Join(tmpDir, ".cursor", "hooks.json"))
	assertFileExists(t, filepath.Join(tmpDir, ".github", "hooks", "hooks.json"))
	cfg, err := cm.LoadSkillsConfig()
	if err != nil {
		t.Fatalf("LoadSkillsConfig: %v", err)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Targets))
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
	writeCfg(t, cm, &config.SkillsConfig{Targets: []string{cursorDir}})

	hooksResult, err := RemoveHooks(targets, baseDir, baseDir, nil)
	if err != nil {
		t.Fatalf("RemoveHooks: %v", err)
	}
	if len(hooksResult.RemovedFrom) != 1 {
		t.Errorf("expected 1 hook removal, got %d", len(hooksResult.RemovedFrom))
	}

	skillsResult, err := mod.ClearSkills()
	if err != nil {
		t.Fatalf("ClearSkills: %v", err)
	}
	if len(skillsResult.DeletedTargets) != 1 {
		t.Errorf("expected 1 skills target cleared, got %d", len(skillsResult.DeletedTargets))
	}

	if err := cm.SaveSkillsConfig(&config.SkillsConfig{}); err != nil {
		t.Fatalf("SaveSkillsConfig: %v", err)
	}
	loaded, _ := cm.LoadSkillsConfig()
	if len(loaded.Targets) != 0 {
		t.Error("config should be empty after remove")
	}
}

func TestModule_ClearSkills(t *testing.T) {
	t.Run("removes port dir", func(t *testing.T) {
		mod, cm, tmpDir := newTestModule(t)
		portDir := filepath.Join(tmpDir, "skills", PortSkillsDir)
		if err := os.MkdirAll(filepath.Join(portDir, "group-a", "skill-1"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeCfg(t, cm, &config.SkillsConfig{Targets: []string{tmpDir}})

		result, err := mod.ClearSkills()
		if err != nil {
			t.Fatalf("ClearSkills: %v", err)
		}
		if len(result.DeletedTargets) != 1 {
			t.Errorf("expected 1 deleted, got %d", len(result.DeletedTargets))
		}
		assertFileAbsent(t, portDir)
	})

	t.Run("also clears project dirs", func(t *testing.T) {
		mod, cm, _ := newTestModule(t)
		homeDir, projectDir := t.TempDir(), t.TempDir()
		globalTarget := filepath.Join(homeDir, ".cursor")

		targetPortDir := filepath.Join(globalTarget, "skills", PortSkillsDir)
		projectPortDir := filepath.Join(projectDir, ".cursor", "skills", PortSkillsDir)
		for _, d := range []string{filepath.Join(targetPortDir, "grp", "sk"), filepath.Join(projectPortDir, "grp", "sk")} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeCfg(t, cm, &config.SkillsConfig{Targets: []string{globalTarget}, ProjectDirs: []string{projectDir}})

		result, err := mod.ClearSkills()
		if err != nil {
			t.Fatalf("ClearSkills: %v", err)
		}
		if len(result.DeletedTargets) != 2 {
			t.Errorf("expected 2 deleted, got %d (deleted=%v skipped=%v)", len(result.DeletedTargets), result.DeletedTargets, result.SkippedTargets)
		}
		assertFileAbsent(t, targetPortDir)
		assertFileAbsent(t, projectPortDir)
	})

	t.Run("skips missing dir", func(t *testing.T) {
		mod, cm, tmpDir := newTestModule(t)
		writeCfg(t, cm, &config.SkillsConfig{Targets: []string{tmpDir}})

		result, err := mod.ClearSkills()
		if err != nil {
			t.Fatalf("ClearSkills: %v", err)
		}
		if len(result.DeletedTargets) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(result.DeletedTargets))
		}
		if len(result.SkippedTargets) != 1 {
			t.Errorf("expected 1 skipped, got %d", len(result.SkippedTargets))
		}
	})
}

func TestLoadSkills_RuntimeOptionsDoNotPersist(t *testing.T) {
	mod, cm, tmpDir := newTestModule(t)

	runtimeTarget := filepath.Join(tmpDir, ".cursor")
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "platform", SkillIDs: []string{"grouped"}}},
		Skills: []Skill{
			{
				Identifier: "grouped",
				GroupIDs:   []string{"platform"},
				Location:   SkillLocationGlobal,
				Files:      []SkillFile{{Path: "SKILL.md", Content: "# grouped"}},
			},
		},
	}

	if _, err := mod.LoadSkills(context.Background(), LoadSkillsOptions{
		SelectedGroups:      []string{"platform"},
		TargetOverrides:     []string{runtimeTarget},
		ProjectDirOverrides: []string{tmpDir},
		Fetched:             fetched,
		NoSave:              true,
	}); err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	assertFileExists(t, filepath.Join(runtimeTarget, "skills", PortSkillsDir, "platform", "grouped", "SKILL.md"))

	loaded, err := cm.LoadSkillsConfig()
	if err != nil {
		t.Fatalf("LoadSkillsConfig: %v", err)
	}
	if len(loaded.Targets) != 0 {
		t.Fatalf("targets persisted: %v", loaded.Targets)
	}
	if len(loaded.ProjectDirs) != 0 {
		t.Fatalf("project_dirs persisted: %v", loaded.ProjectDirs)
	}
	if len(loaded.SelectedGroups) != 0 || loaded.SelectAllGroups || loaded.SelectAllUngrouped {
		t.Fatalf("selection persisted: %+v", loaded)
	}
	if loaded.LastSyncedAt != "" {
		t.Fatalf("last_synced_at persisted: %s", loaded.LastSyncedAt)
	}
}

func TestLoadSkills_GlobalOnlySelectionOmitsProjectTargetSummary(t *testing.T) {
	mod, _, tmpDir := newTestModule(t)

	globalTarget := filepath.Join(tmpDir, ".cursor")
	projectDir := filepath.Join(tmpDir, "repo")
	fetched := &FetchedSkills{
		Groups: []SkillGroup{{Identifier: "platform", SkillIDs: []string{"local-dev-setup", "port-api-client"}}},
		Skills: []Skill{
			{
				Identifier: "local-dev-setup",
				GroupIDs:   []string{"platform"},
				Location:   SkillLocationGlobal,
				Files:      []SkillFile{{Path: "SKILL.md", Content: "# local dev"}},
			},
			{
				Identifier: "port-api-client",
				GroupIDs:   []string{"platform"},
				Location:   SkillLocationGlobal,
				Files:      []SkillFile{{Path: "SKILL.md", Content: "# api client"}},
			},
		},
	}

	result, err := mod.LoadSkills(context.Background(), LoadSkillsOptions{
		SelectedGroups:      []string{"platform"},
		TargetOverrides:     []string{globalTarget},
		ProjectDirOverrides: []string{projectDir},
		Fetched:             fetched,
		NoSave:              true,
	})
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if result.SkillCount != 2 {
		t.Fatalf("SkillCount = %d, want 2", result.SkillCount)
	}
	if result.GroupCount != 1 {
		t.Fatalf("GroupCount = %d, want 1", result.GroupCount)
	}
	if len(result.TargetResults) != 1 {
		t.Fatalf("TargetResults = %+v, want only global target", result.TargetResults)
	}
	if result.TargetResults[0].Path != globalTarget || result.TargetResults[0].IsProject {
		t.Fatalf("TargetResults = %+v, want global target %q", result.TargetResults, globalTarget)
	}
	if result.TargetResults[0].SkillCount != 2 {
		t.Fatalf("global SkillCount = %d, want 2", result.TargetResults[0].SkillCount)
	}
	if result.TargetResults[0].GroupCount != 1 {
		t.Fatalf("global GroupCount = %d, want 1", result.TargetResults[0].GroupCount)
	}

	assertFileExists(t, skillMDPath(globalTarget, "platform", "local-dev-setup"))
	assertFileExists(t, skillMDPath(globalTarget, "platform", "port-api-client"))
	assertFileAbsent(t, skillMDPath(filepath.Join(projectDir, ".cursor"), "platform", "local-dev-setup"))
	assertFileAbsent(t, skillMDPath(filepath.Join(projectDir, ".cursor"), "platform", "port-api-client"))
}

func TestBuildFetchSkillsQuery_TeamDefaultsIncludesSelectedUngroupedSkills(t *testing.T) {
	query := buildFetchSkillsQuery(&config.SkillsConfig{
		TeamGroupDefaults: true,
		IncludeGroups:     []string{"platform-engineering"},
		SelectedSkills:    []string{"incident-triage"},
		Targets:           []string{"/tmp/.cursor"},
	}, nil)

	if query.TeamsDefault == nil || !*query.TeamsDefault {
		t.Fatalf("TeamsDefault = %v, want true", query.TeamsDefault)
	}
	if len(query.SkillIdentifiers) != 1 || query.SkillIdentifiers[0] != "incident-triage" {
		t.Fatalf("SkillIdentifiers = %v, want incident-triage", query.SkillIdentifiers)
	}
	if len(query.IncludeGroups) != 1 || query.IncludeGroups[0] != "platform-engineering" {
		t.Fatalf("IncludeGroups = %v, want platform-engineering", query.IncludeGroups)
	}
}

func TestBuildFetchSkillsQuery_SelectAllUngroupedRequestsUngroupedCatalog(t *testing.T) {
	query := buildFetchSkillsQuery(&config.SkillsConfig{
		TeamGroupDefaults:  true,
		IncludeGroups:      []string{"platform-engineering"},
		SelectAllUngrouped: true,
		Targets:            []string{"/tmp/.cursor"},
	}, nil)

	if !query.IncludeUngrouped {
		t.Fatal("IncludeUngrouped = false, want true")
	}
	if len(query.SkillIdentifiers) != 0 {
		t.Fatalf("SkillIdentifiers = %v, want none when selecting all ungrouped", query.SkillIdentifiers)
	}
}

func TestModule_Status_ReturnsConfigValues(t *testing.T) {
	mod, cm, _ := newTestModule(t)
	writeCfg(t, cm, &config.SkillsConfig{
		Targets:         []string{"/home/user/.cursor"},
		SelectAllGroups: true,
		LastSyncedAt:    "2026-03-25T10:00:00Z",
	})
	status, err := mod.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
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

// TestInit_ReconcilesTargets covers how init reconciles the saved target set: re-running
// init keeps targets the user re-selects, adds newly selected ones, and preserves targets
// it does not manage for the current scope (another repo's repo-scoped dir). Deselection is
// covered by TestReplaceManagedTargets_DropsDeselectedKeepsForeign.
func TestInit_ReconcilesTargets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CURSOR_CONFIG_DIR", "")
	home := "/home/user"
	cwd := "/repo"

	managed := TargetPaths(DefaultHookTargets(), home, cwd)
	cursor := managed[1]
	claude := managed[2]
	foreign := "/otherrepo/.github"

	// Previously saved: Cursor + another repo's Copilot dir. Re-run init selecting Cursor + Claude.
	got := replaceManagedTargets([]string{cursor, foreign}, []string{cursor, claude}, home, cwd)

	for _, want := range []string{cursor, claude, foreign} {
		if !contains(got, want) {
			t.Errorf("expected target %q to be present, got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 reconciled targets, got %d: %v", len(got), got)
	}
}

func TestInit_AccumulatesDuplicateTargetsOnce(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	target := filepath.Join(tmpDir, ".github")
	writeCfg(t, cm, &config.SkillsConfig{Targets: []string{target}})

	skillsCfg, _ := cm.LoadSkillsConfig()
	skillsCfg.Targets = mergeUnique(skillsCfg.Targets, []string{target})
	if len(skillsCfg.Targets) != 1 {
		t.Errorf("duplicate should not be added, got %d: %v", len(skillsCfg.Targets), skillsCfg.Targets)
	}
}

func TestUniqCopilotSkillRoots_DedupesAndFilters(t *testing.T) {
	repo := t.TempDir()
	gh := filepath.Join(repo, ".github")
	other := filepath.Join(repo, ".cursor")

	got := uniqCopilotSkillRoots([]string{gh, gh, other, filepath.Join(repo, ".github")})
	if len(got) != 1 {
		t.Fatalf("want 1 copilot root, got %d: %v", len(got), got)
	}
	if filepath.Clean(got[0]) != filepath.Clean(gh) {
		t.Errorf("want path %q, got %q", gh, got[0])
	}
}

func TestIsGitHubCopilotSkillRoot(t *testing.T) {
	repo := t.TempDir()
	gh := filepath.Join(repo, ".github")
	if !isGitHubCopilotSkillRoot(gh) {
		t.Error("expected .github under repo to be Copilot skill root")
	}
	if isGitHubCopilotSkillRoot(filepath.Join(repo, ".cursor")) {
		t.Error(".cursor should not be Copilot skill root")
	}
}

func TestAddSkills_RejectsWhenNoPriorInit(t *testing.T) {
	mod, _, tmpDir := newTestModule(t)

	_, err := mod.AddSkills(t.Context(), AddSkillsOptions{
		Targets: []HookTarget{{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON}},
	})
	if err == nil {
		t.Fatal("expected error when no prior init")
	}
	if !strings.Contains(err.Error(), "port skills init") {
		t.Errorf("error should mention init, got: %v", err)
	}
	assertFileAbsent(t, filepath.Join(tmpDir, ".cursor", "hooks.json"))
}

func TestInit_AccumulatesProjectDirs(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	writeCfg(t, cm, &config.SkillsConfig{
		Targets:     []string{filepath.Join(tmpDir, ".github")},
		ProjectDirs: []string{"/repo/one"},
	})

	skillsCfg, _ := cm.LoadSkillsConfig()
	skillsCfg.ProjectDirs = appendUnique(skillsCfg.ProjectDirs, "/repo/two")
	if err := cm.SaveSkillsConfig(skillsCfg); err != nil {
		t.Fatalf("SaveSkillsConfig: %v", err)
	}

	loaded, _ := cm.LoadSkillsConfig()
	if len(loaded.ProjectDirs) != 2 {
		t.Fatalf("expected 2 project dirs, got %d: %v", len(loaded.ProjectDirs), loaded.ProjectDirs)
	}
	if !contains(loaded.ProjectDirs, "/repo/one") || !contains(loaded.ProjectDirs, "/repo/two") {
		t.Errorf("project dirs not accumulated: %v", loaded.ProjectDirs)
	}
}

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestModule_Init_InstallsHooksAndSavesConfig(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	targets := []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON},
		{Name: "GitHub Copilot", Dir: ".copilot", ProjectDir: ".github", Format: hookFormatJSON},
	}
	if err := InstallHooks(targets, tmpDir, tmpDir); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}
	writeCfg(t, cm, &config.PluginConfig{Targets: TargetPaths(targets, tmpDir, tmpDir)})

	for _, dir := range []string{".cursor", ".copilot"} {
		assertFileExists(t, filepath.Join(tmpDir, dir, "hooks.json"))
	}
	cfg, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig: %v", err)
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
	writeCfg(t, cm, &config.PluginConfig{Targets: []string{cursorDir}})

	hooksResult, err := RemoveHooks(targets, baseDir, baseDir)
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

	if err := cm.SavePluginConfig(&config.PluginConfig{}); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}
	loaded, _ := cm.LoadPluginConfig()
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
		writeCfg(t, cm, &config.PluginConfig{Targets: []string{tmpDir}})

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
		writeCfg(t, cm, &config.PluginConfig{Targets: []string{globalTarget}, ProjectDirs: []string{projectDir}})

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
		writeCfg(t, cm, &config.PluginConfig{Targets: []string{tmpDir}})

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

func TestModule_Status_ReturnsConfigValues(t *testing.T) {
	mod, cm, _ := newTestModule(t)
	writeCfg(t, cm, &config.PluginConfig{
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

func TestInit_AccumulatesTargets(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	cursorTarget := filepath.Join(tmpDir, ".cursor")
	copilotTarget := filepath.Join(tmpDir, ".copilot")

	writeCfg(t, cm, &config.PluginConfig{Targets: []string{cursorTarget}})

	targets := []HookTarget{{Name: "GitHub Copilot", Dir: ".copilot", ProjectDir: ".github", Format: hookFormatJSON}}
	if err := InstallHooks(targets, tmpDir, tmpDir); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}

	pluginCfg, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig: %v", err)
	}
	pluginCfg.Targets = mergeUnique(pluginCfg.Targets, TargetPaths(targets, tmpDir, tmpDir))
	if err := cm.SavePluginConfig(pluginCfg); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}

	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig after merge: %v", err)
	}
	if len(loaded.Targets) != 2 {
		t.Fatalf("expected 2 accumulated targets, got %d: %v", len(loaded.Targets), loaded.Targets)
	}
	if !contains(loaded.Targets, cursorTarget) || !contains(loaded.Targets, copilotTarget) {
		t.Errorf("targets not accumulated correctly: %v", loaded.Targets)
	}
}

func TestInit_AccumulatesDuplicateTargetsOnce(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	target := filepath.Join(tmpDir, ".copilot")
	writeCfg(t, cm, &config.PluginConfig{Targets: []string{target}})

	pluginCfg, _ := cm.LoadPluginConfig()
	pluginCfg.Targets = mergeUnique(pluginCfg.Targets, []string{target})
	if len(pluginCfg.Targets) != 1 {
		t.Errorf("duplicate should not be added, got %d: %v", len(pluginCfg.Targets), pluginCfg.Targets)
	}
}

func TestInit_AccumulatesProjectDirs(t *testing.T) {
	_, cm, tmpDir := newTestModule(t)
	writeCfg(t, cm, &config.PluginConfig{
		Targets:     []string{filepath.Join(tmpDir, ".copilot")},
		ProjectDirs: []string{"/repo/one"},
	})

	pluginCfg, _ := cm.LoadPluginConfig()
	pluginCfg.ProjectDirs = appendUnique(pluginCfg.ProjectDirs, "/repo/two")
	if err := cm.SavePluginConfig(pluginCfg); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}

	loaded, _ := cm.LoadPluginConfig()
	if len(loaded.ProjectDirs) != 2 {
		t.Fatalf("expected 2 project dirs, got %d: %v", len(loaded.ProjectDirs), loaded.ProjectDirs)
	}
	if !contains(loaded.ProjectDirs, "/repo/one") || !contains(loaded.ProjectDirs, "/repo/two") {
		t.Errorf("project dirs not accumulated: %v", loaded.ProjectDirs)
	}
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestDefaultSyncTargets_AgentsAndClaude(t *testing.T) {
	targets := DefaultSyncTargets()
	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(targets))
	}
	if targets[0].Name != "Agents (cross-platform)" || targets[0].Dir != ".agents" || !targets[0].SkillsOnly {
		t.Fatalf("first target: %+v", targets[0])
	}
	if targets[1].Name != "Claude Code" || targets[1].Dir != ".claude" || !targets[1].SkillsOnly {
		t.Fatalf("second target: %+v", targets[1])
	}
}

func TestApplySyncDefaults_EmptyConfig(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	cfg := &config.SkillsConfig{}
	ApplySyncDefaults(cfg)

	if len(cfg.Targets) != 2 {
		t.Fatalf("targets: %v", cfg.Targets)
	}
	for _, wantSuffix := range []string{".agents", ".claude"} {
		found := false
		for _, p := range cfg.Targets {
			if strings.HasSuffix(p, wantSuffix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing target suffix %s in %v", wantSuffix, cfg.Targets)
		}
	}
	gotProj, _ := filepath.EvalSymlinks(cfg.ProjectDirs[0])
	wantProj, _ := filepath.EvalSymlinks(cwd)
	if len(cfg.ProjectDirs) != 1 || gotProj != wantProj {
		t.Fatalf("project_dirs: %v want %s", cfg.ProjectDirs, cwd)
	}
	if !cfg.TeamGroupDefaults || !cfg.SelectAllUngrouped {
		t.Fatalf("selection defaults: team=%v ungrouped=%v", cfg.TeamGroupDefaults, cfg.SelectAllUngrouped)
	}
}

func TestApplySyncDefaults_PreservesExisting(t *testing.T) {
	cfg := &config.SkillsConfig{
		Targets:           []string{"/custom/.cursor"},
		ProjectDirs:       []string{"/proj"},
		TeamGroupDefaults: false,
		SelectedGroups:    []string{"g1"},
	}
	ApplySyncDefaults(cfg)
	if len(cfg.Targets) != 1 || cfg.Targets[0] != "/custom/.cursor" {
		t.Fatalf("targets changed: %v", cfg.Targets)
	}
	if cfg.TeamGroupDefaults {
		t.Fatal("should not enable team defaults when selection exists")
	}
}

func TestInstallHooks_SkipsSkillsOnly(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallHooks(DefaultSyncTargets(), home, repo); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hooks.json", "settings.json"} {
		if err := filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && d.Name() == name {
				t.Errorf("unexpected hook file %s under skills-only home", path)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

package skills

import (
	"os"

	"github.com/port-experimental/port-cli/internal/config"
)

// ApplySyncDefaults fills targets, project dirs, and skill selection when the user
// has not run 'port skills init'. Sync writes to ~/.agents and ~/.claude (and the
// current project’s .agents/.claude trees) and selects all customer skill groups.
func ApplySyncDefaults(cfg *config.SkillsConfig) {
	if cfg == nil {
		return
	}
	home := userHomeDir()
	cwd, _ := os.Getwd()
	if len(cfg.Targets) == 0 {
		cfg.Targets = TargetPaths(DefaultSyncTargets(), home, cwd)
	}
	if len(cfg.ProjectDirs) == 0 && cwd != "" {
		cfg.ProjectDirs = appendUnique(cfg.ProjectDirs, cwd)
	}
	if !cfg.HasSkillContentSelection() {
		cfg.SelectAllGroups = true
		cfg.SelectAllUngrouped = true
	}
}

package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookFormat string

const (
	hookFormatJSON        hookFormat = "hooks_json"
	hookFormatCopilotJSON hookFormat = "copilot_hooks_json"
	hookFormatClaude      hookFormat = "claude_settings"
	hookFormatGemini      hookFormat = "gemini_settings"
	hookFormatWindsurf    hookFormat = "windsurf_hooks"
)

const hookCommand = "port skills sync --quiet"

// HookTarget describes one AI tool directory and how to write its hook.
// When RepoScoped is true the hook is installed relative to the repository
// root (cwd) rather than the user's home directory.
//
// ProjectDir, when set, overrides Dir for project-scoped skill placement when
// mapping global hook target paths to per-repo tool directories (see
// extractProjectDirs). Most tools leave this empty so Dir is used.
//
// HookSubDir, when set, is appended to the resolved base directory so hooks
// are written to {base}/{HookSubDir}/ (e.g. GitHub Copilot uses base
// <repo>/.github and HookSubDir "hooks" for <repo>/.github/hooks/hooks.json).
// Skills are always written under {base}/skills/port/ (not under HookSubDir).
//
// LegacyHookDirs lists extra directories under the user's home directory
// where older CLI versions may have installed hooks for this tool. RemoveHooks
// cleans those paths in addition to the primary hook directory.
//
// EnvOverride names an environment variable (e.g. CURSOR_CONFIG_DIR) that,
// when set, is used as the absolute directory instead of the default.
// XDGDir names the directory under $XDG_CONFIG_HOME (e.g. "cursor") used
// on Linux/BSD when XDG_CONFIG_HOME is set and EnvOverride is not.
type HookTarget struct {
	Name           string
	Dir            string
	ProjectDir     string
	Format         hookFormat
	RepoScoped     bool
	Note           string
	EnvOverride    string
	XDGDir         string
	HookSubDir     string
	LegacyHookDirs []string
}

// DefaultHookTargets returns the list of supported AI tool directories.
func DefaultHookTargets() []HookTarget {
	return []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON, EnvOverride: "CURSOR_CONFIG_DIR", XDGDir: "cursor"},
		{Name: "Claude Code", Dir: ".claude", Format: hookFormatClaude},
		{Name: "Gemini CLI", Dir: ".gemini", Format: hookFormatGemini},
		{Name: "OpenAI Codex", Dir: ".codex", Format: hookFormatJSON},
		{Name: "Windsurf", Dir: ".codeium/windsurf", Format: hookFormatWindsurf},
		{
			Name:           "GitHub Copilot",
			Dir:            ".github",
			Format:         hookFormatCopilotJSON,
			RepoScoped:     true,
			HookSubDir:     "hooks",
			LegacyHookDirs: []string{".copilot"},
			Note:           "repo only — run init from the repository root",
		},
	}
}

// TargetPaths resolves the absolute paths for all hook targets.
// Global targets are rooted at globalRoot (home dir); repo-scoped targets
// are rooted at repoRoot (cwd).
func TargetPaths(targets []HookTarget, globalRoot, repoRoot string) []string {
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		paths = append(paths, resolveTargetDir(t, globalRoot, repoRoot))
	}
	return paths
}

// resolveTargetDir returns the absolute directory for a target.
// For repo-scoped targets it uses repoRoot. For global targets the resolution
// order is: tool-specific env var (EnvOverride) > XDG_CONFIG_HOME+XDGDir > homeDir+Dir.
func resolveTargetDir(t HookTarget, homeDir, repoRoot string) string {
	if t.RepoScoped {
		return filepath.Join(repoRoot, t.Dir)
	}
	if t.EnvOverride != "" {
		if v := os.Getenv(t.EnvOverride); v != "" {
			return v
		}
	}
	if t.XDGDir != "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, t.XDGDir)
		}
	}
	return filepath.Join(homeDir, t.Dir)
}

// ResolveTargetNames maps saved target paths back to their HookTarget names.
// It matches by checking suffixes (Dir, XDGDir) and exact env-override values.
func ResolveTargetNames(savedPaths []string, targets []HookTarget) []string {
	var names []string
	seen := make(map[string]bool)
	for _, sp := range savedPaths {
		for _, t := range targets {
			if seen[t.Name] {
				continue
			}
			if matchesTarget(sp, t) {
				names = append(names, t.Name)
				seen[t.Name] = true
				break
			}
		}
	}
	return names
}

func matchesTarget(savedPath string, t HookTarget) bool {
	if t.Name == "GitHub Copilot" && hasDirSuffix(savedPath, ".copilot") {
		// Legacy installs used ~/.copilot as the hook + skill root.
		return true
	}
	if t.EnvOverride != "" {
		if v := os.Getenv(t.EnvOverride); v != "" && savedPath == v {
			return true
		}
	}
	if hasDirSuffix(savedPath, t.Dir) {
		return true
	}
	if t.XDGDir != "" && hasDirSuffix(savedPath, t.XDGDir) {
		return true
	}
	return false
}

// hookInstallDir returns the directory that contains hooks.json (JSON tools),
// settings.json, etc. When HookSubDir is set it is joined after the resolved
// base directory from resolveTargetDir.
func hookInstallDir(t HookTarget, homeDir, repoRoot string) string {
	base := resolveTargetDir(t, homeDir, repoRoot)
	if t.HookSubDir != "" {
		return filepath.Join(base, filepath.FromSlash(t.HookSubDir))
	}
	return base
}

// hookRemoveDirs returns all directories where RemoveHooks should strip Port
// entries for this target (primary install dir plus legacy home dirs).
func hookRemoveDirs(t HookTarget, homeDir, repoRoot string) []string {
	primary := hookInstallDir(t, homeDir, repoRoot)
	out := []string{primary}
	seen := map[string]bool{primary: true}
	for _, leg := range t.LegacyHookDirs {
		p := filepath.Join(homeDir, filepath.FromSlash(leg))
		if !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	return out
}

// primaryHookDirForSkillRoot maps a saved skill sync root (e.g. <repo>/.github)
// to the directory that contains hooks.json.
func primaryHookDirForSkillRoot(t HookTarget, skillRoot string) string {
	if t.HookSubDir != "" {
		return filepath.Join(skillRoot, filepath.FromSlash(t.HookSubDir))
	}
	return skillRoot
}

// hookDirsToClean lists hook directories to strip for target t.
// savedSkillRoots comes from ~/.port/config.yaml skills.targets (absolute paths).
// For repo-scoped tools, each saved path that matches t is cleaned, then a
// fallback uses repoRoot (typically the process cwd) so `port cache clear`
// still works when run from the repository root.
func hookDirsToClean(t HookTarget, homeDir, repoRoot string, savedSkillRoots []string) []string {
	if !t.RepoScoped {
		return hookRemoveDirs(t, homeDir, repoRoot)
	}
	var out []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	for _, sp := range savedSkillRoots {
		ex := expandHome(sp)
		if matchesTarget(ex, t) {
			add(primaryHookDirForSkillRoot(t, ex))
		}
	}
	add(primaryHookDirForSkillRoot(t, resolveTargetDir(t, homeDir, repoRoot)))
	for _, leg := range t.LegacyHookDirs {
		add(filepath.Join(homeDir, filepath.FromSlash(leg)))
	}
	return out
}

func hasDirSuffix(path, dir string) bool {
	return strings.HasSuffix(path, string(filepath.Separator)+dir) || path == dir
}

// hookWriter is implemented by each format to install/remove hooks.
type hookWriter interface {
	Write(dir string) error
	Remove(dir string) (changed bool, err error)
}

func writerFor(f hookFormat) hookWriter {
	switch f {
	case hookFormatJSON:
		return jsonHookWriter{}
	case hookFormatCopilotJSON:
		return copilotJSONHookWriter{}
	case hookFormatClaude:
		return settingsHookWriter{eventKey: "UserPromptSubmit", topLevelArray: false}
	case hookFormatGemini:
		return settingsHookWriter{eventKey: "SessionStart", topLevelArray: false}
	case hookFormatWindsurf:
		return windsurfHookWriter{}
	default:
		return jsonHookWriter{}
	}
}

// InstallHooks writes (or merges) the hook configuration for each target.
func InstallHooks(targets []HookTarget, globalRoot, repoRoot string) error {
	for _, t := range targets {
		dir := hookInstallDir(t, globalRoot, repoRoot)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if err := writerFor(t.Format).Write(dir); err != nil {
			return fmt.Errorf("failed to write hook for %s: %w", t.Name, err)
		}
	}
	return nil
}

// RemoveHooksResult reports what was changed per target.
type RemoveHooksResult struct {
	RemovedFrom []string
	Skipped     []string
}

// RemoveHooks removes only the Port hook entries from each target,
// preserving any other hooks. Empty hook files are deleted entirely.
// savedSkillRoots should be config.SkillsConfig.Targets so repo-scoped hooks
// (GitHub Copilot) are found even when repoRoot is not the repository where
// hooks were installed. Pass nil to only use repoRoot for repo-scoped targets.
func RemoveHooks(targets []HookTarget, globalRoot, repoRoot string, savedSkillRoots []string) (*RemoveHooksResult, error) {
	result := &RemoveHooksResult{}
	for _, t := range targets {
		for _, dir := range hookDirsToClean(t, globalRoot, repoRoot, savedSkillRoots) {
			removed, err := writerFor(t.Format).Remove(dir)
			if err != nil {
				return nil, fmt.Errorf("failed to remove hook for %s: %w", t.Name, err)
			}
			if removed {
				result.RemovedFrom = append(result.RemovedFrom, dir)
			} else {
				result.Skipped = append(result.Skipped, dir)
			}
		}
	}
	return result, nil
}

// legacyHookCommands lists hook command strings written by older versions of
// the CLI so that init/uninit can remove them even after renames.
var legacyHookCommands = []string{
	"port skills sync",
	"port plugin sync",
	"port plugin sync --quiet",
}

func isPortCommand(cmd string) bool {
	if cmd == hookCommand {
		return true
	}
	for _, legacy := range legacyHookCommands {
		if cmd == legacy {
			return true
		}
	}
	return false
}

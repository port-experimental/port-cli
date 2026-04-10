package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookFormat string

const (
	hookFormatJSON     hookFormat = "hooks_json"
	hookFormatClaude   hookFormat = "claude_settings"
	hookFormatGemini   hookFormat = "gemini_settings"
	hookFormatWindsurf hookFormat = "windsurf_hooks"
)

const hookCommand = "port skills sync --quiet"

// HookTarget describes one AI tool directory and how to write its hook.
// When RepoScoped is true the hook is installed relative to the repository
// root (cwd) rather than the user's home directory.
//
// ProjectDir, when set, overrides Dir for project-scoped skill placement.
// For example, GitHub Copilot uses ~/.copilot globally but reads project
// skills from <repo>/.github/skills.
//
// EnvOverride names an environment variable (e.g. CURSOR_CONFIG_DIR) that,
// when set, is used as the absolute directory instead of the default.
// XDGDir names the directory under $XDG_CONFIG_HOME (e.g. "cursor") used
// on Linux/BSD when XDG_CONFIG_HOME is set and EnvOverride is not.
type HookTarget struct {
	Name        string
	Dir         string
	ProjectDir  string
	Format      hookFormat
	RepoScoped  bool
	Note        string
	EnvOverride string
	XDGDir      string
}

// DefaultHookTargets returns the list of supported AI tool directories.
func DefaultHookTargets() []HookTarget {
	return []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON, EnvOverride: "CURSOR_CONFIG_DIR", XDGDir: "cursor"},
		{Name: "Claude Code", Dir: ".claude", Format: hookFormatClaude},
		{Name: "Gemini CLI", Dir: ".gemini", Format: hookFormatGemini},
		{Name: "OpenAI Codex", Dir: ".codex", Format: hookFormatJSON},
		{Name: "Windsurf", Dir: ".codeium/windsurf", Format: hookFormatWindsurf},
		{Name: "GitHub Copilot", Dir: ".copilot", ProjectDir: ".github", Format: hookFormatJSON},
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
		dir := resolveTargetDir(t, globalRoot, repoRoot)
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
func RemoveHooks(targets []HookTarget, globalRoot, repoRoot string) (*RemoveHooksResult, error) {
	result := &RemoveHooksResult{}
	for _, t := range targets {
		dir := resolveTargetDir(t, globalRoot, repoRoot)
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

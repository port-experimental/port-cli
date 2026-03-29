package plugin

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

const hookCommand = "port plugin sync"

// HookTarget describes one AI tool directory and how to write its hook.
// When RepoScoped is true the hook is installed relative to the repository
// root (cwd) rather than the user's home directory.
type HookTarget struct {
	Name       string
	Dir        string
	Format     hookFormat
	RepoScoped bool
	Note       string
}

// DefaultHookTargets returns the list of supported AI tool directories.
func DefaultHookTargets() []HookTarget {
	return []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON},
		{Name: "Claude Code", Dir: ".claude", Format: hookFormatClaude},
		{Name: "Gemini CLI", Dir: ".gemini", Format: hookFormatGemini},
		{Name: "OpenAI Codex", Dir: ".codex", Format: hookFormatJSON},
		{Name: "Windsurf", Dir: ".codeium/windsurf", Format: hookFormatWindsurf},
		{Name: "GitHub Copilot", Dir: ".agents", Format: hookFormatJSON},
	}
}

// TargetPaths resolves the absolute paths for all hook targets.
// Global targets are rooted at globalRoot (home dir); repo-scoped targets
// are rooted at repoRoot (cwd).
func TargetPaths(targets []HookTarget, globalRoot, repoRoot string) []string {
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		paths = append(paths, filepath.Join(targetRoot(t, globalRoot, repoRoot), t.Dir))
	}
	return paths
}

func targetRoot(t HookTarget, globalRoot, repoRoot string) string {
	if t.RepoScoped {
		return repoRoot
	}
	return globalRoot
}

// ResolveTargetNames maps saved target paths back to their HookTarget names.
// It matches by checking if a saved path ends with the target's Dir component.
func ResolveTargetNames(savedPaths []string, targets []HookTarget) []string {
	var names []string
	seen := make(map[string]bool)
	for _, sp := range savedPaths {
		for _, t := range targets {
			if !seen[t.Name] && (strings.HasSuffix(sp, string(filepath.Separator)+t.Dir) || sp == t.Dir) {
				names = append(names, t.Name)
				seen[t.Name] = true
				break
			}
		}
	}
	return names
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
		return settingsHookWriter{eventKey: "UserPromptSubmit", topLevelArray: true}
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
		dir := filepath.Join(targetRoot(t, globalRoot, repoRoot), t.Dir)
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
		dir := filepath.Join(targetRoot(t, globalRoot, repoRoot), t.Dir)
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

func isPortCommand(cmd string) bool {
	return cmd == hookCommand
}

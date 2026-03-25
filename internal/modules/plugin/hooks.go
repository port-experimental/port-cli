package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookFormat describes how a specific AI tool expects its hook configuration.
type hookFormat string

const (
	hookFormatJSON   hookFormat = "hooks_json"    // .cursor/hooks.json, .agents/hooks.json
	hookFormatClaude hookFormat = "claude_settings" // .claude/settings.json (merged)
)

// HookTarget describes one AI tool directory and how to write its hook.
type HookTarget struct {
	// Name is a human-readable label used in output messages.
	Name string
	// Dir is the directory path relative to the scope root (e.g. ".cursor").
	Dir string
	// Format determines which hook file format to write.
	Format hookFormat
}

// DefaultHookTargets returns the list of supported AI tool directories.
// Add new tools here — no other code change required.
func DefaultHookTargets() []HookTarget {
	return []HookTarget{
		{Name: "Cursor", Dir: ".cursor", Format: hookFormatJSON},
		{Name: "Claude Code", Dir: ".claude", Format: hookFormatClaude},
		{Name: "Agents", Dir: ".agents", Format: hookFormatJSON},
	}
}

// TargetPaths resolves the absolute paths for all hook targets given the
// scope root directory (home dir for global, cwd for local).
func TargetPaths(targets []HookTarget, scopeRoot string) []string {
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		paths = append(paths, filepath.Join(scopeRoot, t.Dir))
	}
	return paths
}

// InstallHooks writes (or merges) the hook configuration for each target.
func InstallHooks(targets []HookTarget, scopeRoot string) error {
	for _, t := range targets {
		dir := filepath.Join(scopeRoot, t.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		switch t.Format {
		case hookFormatJSON:
			if err := writeJSONHook(dir); err != nil {
				return fmt.Errorf("failed to write hook for %s: %w", t.Name, err)
			}
		case hookFormatClaude:
			if err := writeClaudeHook(dir); err != nil {
				return fmt.Errorf("failed to write hook for %s: %w", t.Name, err)
			}
		}
	}
	return nil
}

// --- hooks.json (Cursor / Agents) ---

type hooksJSON struct {
	Version int                    `json:"version"`
	Hooks   map[string]interface{} `json:"hooks"`
}

func writeJSONHook(dir string) error {
	path := filepath.Join(dir, "hooks.json")

	existing := &hooksJSON{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, existing)
	}

	if existing.Hooks == nil {
		existing.Hooks = make(map[string]interface{})
	}
	existing.Version = 1

	existing.Hooks["sessionStart"] = []map[string]string{
		{"command": "port plugin load-skills"},
	}

	data, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// --- .claude/settings.json ---

type claudeSettings struct {
	Permissions map[string]interface{} `json:"permissions,omitempty"`
	Hooks       map[string]interface{} `json:"hooks,omitempty"`
}

func writeClaudeHook(dir string) error {
	path := filepath.Join(dir, "settings.json")

	settings := &claudeSettings{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, settings)
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string]interface{})
	}

	settings.Hooks["UserPromptSubmit"] = []map[string]interface{}{
		{
			"hooks": []map[string]interface{}{
				{
					"type":          "command",
					"command":       "port plugin load-skills",
					"timeout":       120,
					"statusMessage": "Fetching available skills from Port...",
				},
			},
		},
	}

	data, err := json.MarshalIndent(settings, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal claude settings.json: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

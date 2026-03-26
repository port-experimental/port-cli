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
	hookFormatJSON   hookFormat = "hooks_json"      // .cursor/hooks.json, .agents/hooks.json
	hookFormatClaude hookFormat = "claude_settings" // .claude/settings.json (merged)
)

// hookCommand is the CLI command written directly into every hook entry.
const hookCommand = "port plugin sync"

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

// RemoveHooksResult reports what was changed per target.
type RemoveHooksResult struct {
	// RemovedFrom are target directory paths where our hook entry was removed.
	RemovedFrom []string
	// Skipped are target directory paths where no hook file was found.
	Skipped []string
}

// RemoveHooks surgically removes only the Port hook entries from each target,
// preserving any other hooks that may exist. If a hooks file becomes empty
// after removal it is deleted entirely.
func RemoveHooks(targets []HookTarget, scopeRoot string) (*RemoveHooksResult, error) {
	result := &RemoveHooksResult{}
	for _, t := range targets {
		dir := filepath.Join(scopeRoot, t.Dir)
		var removed bool
		var err error

		switch t.Format {
		case hookFormatJSON:
			removed, err = removeJSONHook(dir)
		case hookFormatClaude:
			removed, err = removeClaudeHook(dir)
		}

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

// isPortCommand reports whether a command string is our hook command.
func isPortCommand(cmd string) bool {
	return cmd == hookCommand
}

// --- hooks.json (Cursor / Agents) ---

type hooksJSON struct {
	Version int                    `json:"version"`
	Hooks   map[string]interface{} `json:"hooks"`
}

func writeJSONHook(dir string) error {
	jsonPath := filepath.Join(dir, "hooks.json")
	existing := &hooksJSON{}
	if data, err := os.ReadFile(jsonPath); err == nil {
		_ = json.Unmarshal(data, existing)
	}
	if existing.Hooks == nil {
		existing.Hooks = make(map[string]interface{})
	}
	existing.Version = 1
	existing.Hooks["sessionStart"] = []map[string]string{
		{"command": hookCommand},
	}

	data, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}
	return os.WriteFile(jsonPath, data, 0o644)
}

// --- .claude/settings.json ---

func writeClaudeHook(dir string) error {
	path := filepath.Join(dir, "settings.json")

	// Read existing settings as a raw map to avoid clobbering unknown fields.
	raw := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	// Claude Code hooks format: array of matcher objects, each with a "hooks" array.
	// We use UserPromptSubmit so the skills are injected before every prompt.
	portHook := map[string]interface{}{
		"matcher": "UserPromptSubmit",
		"hooks": []map[string]interface{}{
			{
				"type":    "command",
				"command": hookCommand,
			},
		},
	}

	// Merge: keep existing hook entries that aren't ours, then append ours.
	existing, _ := raw["hooks"].([]interface{})
	merged := make([]interface{}, 0, len(existing)+1)
	for _, entry := range existing {
		m, ok := entry.(map[string]interface{})
		if !ok || m["matcher"] == "UserPromptSubmit" {
			continue
		}
		merged = append(merged, entry)
	}
	merged = append(merged, portHook)
	raw["hooks"] = merged

	data, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal claude settings.json: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// --- removal helpers ---

// removeJSONHook removes the Port sessionStart entry from hooks.json.
// Returns true if anything was changed. If the hooks map is empty afterwards,
// hooks.json is deleted entirely.
func removeJSONHook(dir string) (bool, error) {
	jsonPath := filepath.Join(dir, "hooks.json")
	data, err := os.ReadFile(jsonPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read hooks.json: %w", err)
	}

	existing := &hooksJSON{}
	if err := json.Unmarshal(data, existing); err != nil || existing.Hooks == nil {
		return false, nil
	}

	// Inspect the sessionStart array; keep only entries not referencing our command.
	entries, _ := existing.Hooks["sessionStart"].([]interface{})
	kept := make([]interface{}, 0, len(entries))
	for _, raw := range entries {
		m, ok := raw.(map[string]interface{})
		if !ok {
			kept = append(kept, raw)
			continue
		}
		cmd, _ := m["command"].(string)
		if !isPortCommand(cmd) {
			kept = append(kept, raw)
		}
	}

	changed := len(kept) != len(entries)
	if len(kept) == 0 {
		delete(existing.Hooks, "sessionStart")
	} else {
		existing.Hooks["sessionStart"] = kept
	}

	// If no hooks remain, remove the file entirely.
	if len(existing.Hooks) == 0 {
		_ = os.Remove(jsonPath)
		return changed, nil
	}

	out, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return false, fmt.Errorf("failed to marshal hooks.json: %w", err)
	}
	return changed, os.WriteFile(jsonPath, out, 0o644)
}

// removeClaudeHook removes the Port UserPromptSubmit entry from
// .claude/settings.json. Returns true if anything was changed.
func removeClaudeHook(dir string) (bool, error) {
	settingsPath := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read settings.json: %w", err)
	}

	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, nil
	}

	hooks, _ := raw["hooks"].([]interface{})
	kept := make([]interface{}, 0, len(hooks))
	for _, entry := range hooks {
		m, ok := entry.(map[string]interface{})
		if !ok {
			kept = append(kept, entry)
			continue
		}
		// Only remove entries whose matcher is UserPromptSubmit AND whose
		// inner command is ours — leave unrelated entries alone.
		if m["matcher"] != "UserPromptSubmit" {
			kept = append(kept, entry)
			continue
		}
		innerHooks, _ := m["hooks"].([]interface{})
		isOurs := false
		for _, ih := range innerHooks {
			ih2, ok := ih.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := ih2["command"].(string)
			if isPortCommand(cmd) {
				isOurs = true
				break
			}
		}
		if !isOurs {
			kept = append(kept, entry)
		}
	}

	changed := len(kept) != len(hooks)

	if len(kept) == 0 {
		delete(raw, "hooks")
	} else {
		raw["hooks"] = kept
	}

	// If settings.json is now effectively empty, remove it.
	if len(raw) == 0 {
		_ = os.Remove(settingsPath)
		return changed, nil
	}

	out, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return false, fmt.Errorf("failed to marshal settings.json: %w", err)
	}
	return changed, os.WriteFile(settingsPath, out, 0o644)
}

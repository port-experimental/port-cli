package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
// When RepoScoped is true the hook must be installed relative to the current
// working directory (repository root) rather than the user's home directory.
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
		{Name: "Agents", Dir: ".agents", Format: hookFormatJSON},
		{
			Name:       "GitHub Copilot",
			Dir:        ".github/hooks",
			Format:     hookFormatJSON,
			RepoScoped: true,
			Note:       "repo-scoped: installs in the current directory only",
		},
	}
}

// TargetPaths resolves the absolute paths for all hook targets.
// Global targets are rooted at globalRoot (the user's home directory).
// Repo-scoped targets are rooted at repoRoot (the current working directory).
func TargetPaths(targets []HookTarget, globalRoot, repoRoot string) []string {
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		root := globalRoot
		if t.RepoScoped {
			root = repoRoot
		}
		paths = append(paths, filepath.Join(root, t.Dir))
	}
	return paths
}

// InstallHooks writes (or merges) the hook configuration for each target.
// Global targets are installed under globalRoot; repo-scoped targets under repoRoot.
func InstallHooks(targets []HookTarget, globalRoot, repoRoot string) error {
	for _, t := range targets {
		root := globalRoot
		if t.RepoScoped {
			root = repoRoot
		}
		dir := filepath.Join(root, t.Dir)
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
		case hookFormatGemini:
			if err := writeGeminiHook(dir); err != nil {
				return fmt.Errorf("failed to write hook for %s: %w", t.Name, err)
			}
		case hookFormatWindsurf:
			if err := writeWindsurfHook(dir); err != nil {
				return fmt.Errorf("failed to write hook for %s: %w", t.Name, err)
			}
		}
	}
	return nil
}

// RemoveHooksResult reports what was changed per target.
type RemoveHooksResult struct {
	RemovedFrom []string
	Skipped     []string
}

// RemoveHooks surgically removes only the Port hook entries from each target,
// preserving any other hooks that may exist. If a hooks file becomes empty
// after removal it is deleted entirely.
// Global targets are resolved under globalRoot; repo-scoped targets under repoRoot.
func RemoveHooks(targets []HookTarget, globalRoot, repoRoot string) (*RemoveHooksResult, error) {
	result := &RemoveHooksResult{}
	for _, t := range targets {
		root := globalRoot
		if t.RepoScoped {
			root = repoRoot
		}
		dir := filepath.Join(root, t.Dir)
		var removed bool
		var err error

		switch t.Format {
		case hookFormatJSON:
			removed, err = removeJSONHook(dir)
		case hookFormatClaude:
			removed, err = removeClaudeHook(dir)
		case hookFormatGemini:
			removed, err = removeGeminiHook(dir)
		case hookFormatWindsurf:
			removed, err = removeWindsurfHook(dir)
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

func isPortCommand(cmd string) bool {
	return cmd == hookCommand
}

// --- hooks.json (Cursor / OpenAI Codex / Agents) ---

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

	raw := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	portHook := map[string]interface{}{
		"matcher": "UserPromptSubmit",
		"hooks": []map[string]interface{}{
			{
				"type":    "command",
				"command": hookCommand,
			},
		},
	}

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

// --- .gemini/settings.json ---

func writeGeminiHook(dir string) error {
	path := filepath.Join(dir, "settings.json")

	raw := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	portHook := map[string]interface{}{
		"hooks": []map[string]interface{}{
			{
				"type":    "command",
				"command": hookCommand,
			},
		},
	}

	existing, _ := raw["hooks"].(map[string]interface{})
	if existing == nil {
		existing = make(map[string]interface{})
	}

	sessionStartEntries, _ := existing["SessionStart"].([]interface{})
	merged := make([]interface{}, 0, len(sessionStartEntries)+1)
	for _, entry := range sessionStartEntries {
		m, ok := entry.(map[string]interface{})
		if !ok {
			merged = append(merged, entry)
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
			merged = append(merged, entry)
		}
	}
	merged = append(merged, portHook)
	existing["SessionStart"] = merged
	raw["hooks"] = existing

	data, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal gemini settings.json: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// --- .codeium/windsurf/hooks.json (Windsurf) ---

func writeWindsurfHook(dir string) error {
	jsonPath := filepath.Join(dir, "hooks.json")

	raw := map[string]interface{}{}
	if data, err := os.ReadFile(jsonPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	hooksMap, _ := raw["hooks"].(map[string]interface{})
	if hooksMap == nil {
		hooksMap = make(map[string]interface{})
	}

	existing, _ := hooksMap["pre_user_prompt"].([]interface{})
	merged := make([]interface{}, 0, len(existing)+1)
	for _, entry := range existing {
		m, ok := entry.(map[string]interface{})
		if !ok {
			merged = append(merged, entry)
			continue
		}
		cmd, _ := m["command"].(string)
		if !isPortCommand(cmd) {
			merged = append(merged, entry)
		}
	}
	merged = append(merged, map[string]interface{}{"command": hookCommand})
	hooksMap["pre_user_prompt"] = merged
	raw["hooks"] = hooksMap

	data, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal windsurf hooks.json: %w", err)
	}
	return os.WriteFile(jsonPath, data, 0o644)
}

// --- removal helpers ---

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

func removeGeminiHook(dir string) (bool, error) {
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

	hooksMap, _ := raw["hooks"].(map[string]interface{})
	if hooksMap == nil {
		return false, nil
	}

	sessionStartEntries, _ := hooksMap["SessionStart"].([]interface{})
	kept := make([]interface{}, 0, len(sessionStartEntries))
	for _, entry := range sessionStartEntries {
		m, ok := entry.(map[string]interface{})
		if !ok {
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

	changed := len(kept) != len(sessionStartEntries)

	if len(kept) == 0 {
		delete(hooksMap, "SessionStart")
	} else {
		hooksMap["SessionStart"] = kept
	}

	if len(hooksMap) == 0 {
		delete(raw, "hooks")
	} else {
		raw["hooks"] = hooksMap
	}

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

func removeWindsurfHook(dir string) (bool, error) {
	jsonPath := filepath.Join(dir, "hooks.json")
	data, err := os.ReadFile(jsonPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read hooks.json: %w", err)
	}

	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, nil
	}

	hooksMap, _ := raw["hooks"].(map[string]interface{})
	if hooksMap == nil {
		return false, nil
	}

	entries, _ := hooksMap["pre_user_prompt"].([]interface{})
	kept := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		m, ok := entry.(map[string]interface{})
		if !ok {
			kept = append(kept, entry)
			continue
		}
		cmd, _ := m["command"].(string)
		if !isPortCommand(cmd) {
			kept = append(kept, entry)
		}
	}

	changed := len(kept) != len(entries)

	if len(kept) == 0 {
		delete(hooksMap, "pre_user_prompt")
	} else {
		hooksMap["pre_user_prompt"] = kept
	}

	if len(hooksMap) == 0 {
		delete(raw, "hooks")
	}

	if len(raw) == 0 {
		_ = os.Remove(jsonPath)
		return changed, nil
	}

	out, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return false, fmt.Errorf("failed to marshal hooks.json: %w", err)
	}
	return changed, os.WriteFile(jsonPath, out, 0o644)
}

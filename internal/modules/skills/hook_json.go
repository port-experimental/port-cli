package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// jsonHookWriter handles the hooks.json format used by Cursor, OpenAI Codex,
// and GitHub Copilot.
type jsonHookWriter struct{}

// hooksJSON models the hooks.json file structure.
// The Hooks map uses string keys (event names like "sessionStart") and
// each value is a slice of sessionHookEntry. Unknown event keys and their
// values are preserved during round-trips via the raw map.
type hooksJSON struct {
	Version int                    `json:"version"`
	Hooks   map[string]interface{} `json:"hooks"`
}

// sessionHookEntry represents a single hook entry with a command string.
type sessionHookEntry struct {
	Command string `json:"command"`
}

func (jsonHookWriter) Write(dir string) error {
	jsonPath := filepath.Join(dir, "hooks.json")
	existing := &hooksJSON{}
	if data, err := os.ReadFile(jsonPath); err == nil {
		_ = json.Unmarshal(data, existing)
	}
	if existing.Hooks == nil {
		existing.Hooks = make(map[string]interface{})
	}
	existing.Version = 1
	existing.Hooks["sessionStart"] = []sessionHookEntry{
		{Command: hookCommand},
	}

	data, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}
	return os.WriteFile(jsonPath, data, 0o644)
}

func (jsonHookWriter) Remove(dir string) (bool, error) {
	jsonPath := filepath.Join(dir, "hooks.json")

	data, err := os.ReadFile(jsonPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", jsonPath, err)
	}

	existing := &hooksJSON{}
	if err := json.Unmarshal(data, existing); err != nil || existing.Hooks == nil {
		return false, nil
	}

	entries, _ := existing.Hooks["sessionStart"].([]interface{})
	kept := filterCommands(entries)

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

	return changed, writeJSONFile(jsonPath, existing)
}

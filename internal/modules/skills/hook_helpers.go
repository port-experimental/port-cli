package skills

import (
	"encoding/json"
	"fmt"
	"os"
)

// readJSONFile loads a JSON value from disk. Returns (nil, nil) when the file
// does not exist, and (nil, err) on other I/O or parse errors.
func readJSONFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return v, nil
}

// writeJSONFile marshals v as indented JSON and writes it to path.
func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0o644)
}

// filterCommands returns entries that are not the Port hook command.
// Works for flat arrays of {command: string} entries (hooks.json, windsurf).
func filterCommands(entries []interface{}) []interface{} {
	kept := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		m, ok := entry.(map[string]interface{})
		if !ok {
			kept = append(kept, entry)
			continue
		}
		cmd, ok := m["command"].(string)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if !isPortCommand(cmd) {
			kept = append(kept, entry)
		}
	}
	return kept
}

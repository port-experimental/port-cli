package plugin

import (
	"os"
	"path/filepath"
)

// windsurfHookWriter handles the .codeium/windsurf/hooks.json format.
type windsurfHookWriter struct{}

func (windsurfHookWriter) Write(dir string) error {
	jsonPath := filepath.Join(dir, "hooks.json")

	raw, _ := readJSONFileMap(jsonPath)
	if raw == nil {
		raw = map[string]interface{}{}
	}

	hooksMap, _ := raw["hooks"].(map[string]interface{})
	if hooksMap == nil {
		hooksMap = make(map[string]interface{})
	}

	existing, _ := hooksMap["pre_user_prompt"].([]interface{})
	merged := filterCommands(existing)
	merged = append(merged, map[string]interface{}{"command": hookCommand})
	hooksMap["pre_user_prompt"] = merged
	raw["hooks"] = hooksMap

	return writeJSONFile(jsonPath, raw)
}

func (windsurfHookWriter) Remove(dir string) (bool, error) {
	jsonPath := filepath.Join(dir, "hooks.json")

	raw, err := readJSONFileMap(jsonPath)
	if raw == nil {
		return false, err
	}

	hooksMap, _ := raw["hooks"].(map[string]interface{})
	if hooksMap == nil {
		return false, nil
	}

	entries, _ := hooksMap["pre_user_prompt"].([]interface{})
	kept := filterCommands(entries)
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

	return changed, writeJSONFile(jsonPath, raw)
}

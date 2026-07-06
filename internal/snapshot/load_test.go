package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestLoadFromFile_JSON(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "export.json")
	content := `{"blueprints":[{"identifier":"service","title":"Service"}]}`
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	loadJSON := func(path string) (*export.Data, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		var raw map[string]interface{}
		if err := json.NewDecoder(file).Decode(&raw); err != nil {
			return nil, err
		}
		data := &export.Data{}
		if blueprints, ok := raw["blueprints"].([]interface{}); ok {
			for _, bp := range blueprints {
				if bpMap, ok := bp.(map[string]interface{}); ok {
					data.Blueprints = append(data.Blueprints, api.Blueprint(bpMap))
				}
			}
		}
		return data, nil
	}

	snap, err := LoadFromFile(inputPath, loadJSON)
	if err != nil {
		t.Fatalf("LoadFromFile error: %v", err)
	}
	if snap.OrgName != inputPath {
		t.Errorf("expected org name %q, got %q", inputPath, snap.OrgName)
	}
	if snap.Metadata.Source != "file" {
		t.Errorf("expected file source, got %q", snap.Metadata.Source)
	}
	if len(snap.Data.Blueprints) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(snap.Data.Blueprints))
	}
	if snap.Data.Blueprints[0]["identifier"] != "service" {
		t.Errorf("unexpected blueprint: %#v", snap.Data.Blueprints[0])
	}
}

package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr bool
	}{
		{
			name: "valid options",
			options: Options{
				OutputPath: "/tmp/test.json",
				Format:     "json",
			},
			wantErr: false,
		},
		{
			name: "missing output path",
			options: Options{
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			options: Options{
				OutputPath: "/tmp/test.json",
				Format:     "xml",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCollector(t *testing.T) {
	// This is a simple test to ensure NewCollector doesn't panic
	// We can't test collection without a real API client
	client := api.NewClient("test-id", "test-secret", "https://api.getport.io/v1", 0)
	_ = NewCollector(client)
}

func TestWriteJSON(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.json")

	data := &Data{
		Blueprints: []api.Blueprint{
			{"identifier": "test-bp", "title": "Test Blueprint"},
		},
		Entities: []api.Entity{},
	}

	if err := writeJSON(data, outputPath); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Verify file contains valid JSON
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Errorf("Output file is not valid JSON: %v", err)
	}
}

func TestWriteTar(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.tar.gz")

	data := &Data{
		Blueprints: []api.Blueprint{
			{"identifier": "test-bp", "title": "Test Blueprint"},
		},
		Entities: []api.Entity{},
	}

	if err := writeTar(data, outputPath); err != nil {
		t.Fatalf("writeTar() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}
}

func TestDataHasPermissionFields(t *testing.T) {
	d := &Data{
		BlueprintPermissions: map[string]api.Permissions{"bp1": {"view": []string{"$team"}}},
		ActionPermissions:    map[string]api.Permissions{"act1": {"execute": []string{"$admin"}}},
	}
	if d.BlueprintPermissions["bp1"] == nil {
		t.Error("expected blueprint permissions")
	}
	if d.ActionPermissions["act1"] == nil {
		t.Error("expected action permissions")
	}

	// Verify JSON serialisation uses PascalCase (consistent with other Data fields)
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, `"BlueprintPermissions"`) {
		t.Errorf("expected PascalCase key BlueprintPermissions in JSON, got: %s", outStr)
	}
	if !strings.Contains(outStr, `"ActionPermissions"`) {
		t.Errorf("expected PascalCase key ActionPermissions in JSON, got: %s", outStr)
	}
}

func TestWriteJSON_IncludesPermissions(t *testing.T) {
	d := &Data{
		Blueprints: []api.Blueprint{{"identifier": "service", "title": "Service"}},
		BlueprintPermissions: map[string]api.Permissions{
			"service": {"entities": map[string]interface{}{"view": []string{"$team"}}},
		},
		ActionPermissions: map[string]api.Permissions{
			"deploy": {"execute": map[string]interface{}{"users": []string{}}},
		},
	}
	var buf bytes.Buffer
	if err := d.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"BlueprintPermissions"`) {
		t.Errorf("expected BlueprintPermissions key in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"ActionPermissions"`) {
		t.Errorf("expected ActionPermissions key in JSON output, got: %s", output)
	}
	if !strings.Contains(output, "service") {
		t.Errorf("expected 'service' identifier in permissions JSON, got: %s", output)
	}
}

// Note: Integration tests for actual API calls would require:
// 1. A test Port organization
// 2. Valid credentials
// 3. Mock HTTP server or test fixtures

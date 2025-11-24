package export

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// Note: Integration tests for actual API calls would require:
// 1. A test Port organization
// 2. Valid credentials
// 3. Mock HTTP server or test fixtures


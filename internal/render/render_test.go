package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	exportmodule "github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
)

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	output.SetWriters(&buf, &buf)
	t.Cleanup(func() {
		output.SetWriters(nil, nil)
	})
	fn()
	return buf.String()
}

func TestExportRenderer_JSONGolden(t *testing.T) {
	result := &exportmodule.Result{
		Success:            true,
		Message:            "exported",
		OutputPath:         "backup.tar.gz",
		Format:             "tar",
		BlueprintsCount:    3,
		EntitiesCount:      10,
		ActionsCount:       2,
		UsersCount:         1,
		TeamsCount:         1,
		PagesCount:         4,
		IntegrationsCount:  2,
	}

	out := captureOutput(t, func() {
		if err := (ExportRenderer{}).Render(result, nil, ExportResultOptions{Format: FormatJSON}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if parsed["success"] != true {
		t.Fatalf("expected success true, got %#v", parsed["success"])
	}
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %#v", parsed["data"])
	}
	if data["output_path"] != "backup.tar.gz" {
		t.Fatalf("unexpected output_path: %#v", data["output_path"])
	}
}

func TestExportRenderer_TextGolden(t *testing.T) {
	result := &exportmodule.Result{
		Success:         true,
		Message:         "done",
		BlueprintsCount: 1,
	}

	out := captureOutput(t, func() {
		if err := (ExportRenderer{}).Render(result, nil, ExportResultOptions{Format: FormatText}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	for _, want := range []string{
		"Export completed successfully",
		"done",
		"Blueprints: 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestMigrateRenderer_JSONIncludesDryRunDetails(t *testing.T) {
	result := &migrate.Result{
		Success:            true,
		Message:            "dry-run ok",
		BlueprintsToCreate: []string{"service"},
	}

	out := captureOutput(t, func() {
		if err := (MigrateRenderer{}).Render(result, nil, MigrateResultOptions{Format: FormatJSON}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, ok := parsed["blueprints_to_create"]; !ok {
		t.Fatalf("expected blueprints_to_create in JSON: %s", out)
	}
}

func TestMigrateRenderer_TextVerboseDryRunDetails(t *testing.T) {
	result := &migrate.Result{
		Success:            true,
		Message:            "ok",
		BlueprintsToCreate: []string{"service"},
	}

	out := captureOutput(t, func() {
		if err := (MigrateRenderer{}).Render(result, nil, MigrateResultOptions{
			Format:  FormatText,
			Verbose: true,
		}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	if !strings.Contains(out, "Dry-run details:") {
		t.Fatalf("expected dry-run details, got:\n%s", out)
	}
	if !strings.Contains(out, "service") {
		t.Fatalf("expected blueprint id in output, got:\n%s", out)
	}
}

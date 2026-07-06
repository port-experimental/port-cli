package render

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	exportmodule "github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/compare"
	importmodule "github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/port-experimental/port-cli/internal/output"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
	})
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}

func TestImportRenderer_JSONGolden(t *testing.T) {
	result := &importmodule.Result{
		Success:           true,
		Message:           "imported",
		BlueprintsCreated: 2,
		ActionsUpdated:    1,
	}

	out := captureOutput(t, func() {
		if err := (ImportRenderer{}).Render(result, nil, ImportResultOptions{Format: FormatJSON}); err != nil {
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
	if parsed["blueprints_created"] != float64(2) {
		t.Fatalf("unexpected blueprints_created: %#v", parsed["blueprints_created"])
	}
}

func TestImportRenderer_TextGolden(t *testing.T) {
	result := &importmodule.Result{
		Success:           true,
		Message:           "done",
		BlueprintsCreated: 1,
	}

	out := captureOutput(t, func() {
		if err := (ImportRenderer{}).Render(result, nil, ImportResultOptions{Format: FormatText}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	for _, want := range []string{
		"Import completed successfully",
		"done",
		"Blueprints created: 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCompareRenderer_TextGolden(t *testing.T) {
	result := &compare.CompareResult{
		Identical: false,
		Actions: compare.ResourceDiff{
			Summary: compare.DiffSummary{Added: 1, Modified: 2},
		},
	}

	out := captureStdout(t, func() {
		if err := (CompareRenderer{}).Render(result, compare.Options{OutputFormat: "text"}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	for _, want := range []string{
		"Actions:",
		"1 added",
		"2 modified",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCompareRenderer_JSONGolden(t *testing.T) {
	result := &compare.CompareResult{
		Identical: true,
	}

	out := captureStdout(t, func() {
		if err := (CompareRenderer{}).Render(result, compare.Options{OutputFormat: "json"}); err != nil {
			t.Fatalf("Render failed: %v", err)
		}
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if parsed["identical"] != true {
		t.Fatalf("expected identical true, got %#v", parsed["identical"])
	}
}

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

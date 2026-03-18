package import_module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestLoader_LoadJSON_IncludesFolders(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "export.json")

	content := `{
  "blueprints": [{"identifier":"service","title":"Service"}],
  "_folders": [{"identifier":"quorum","title":"Quorum","after":"catalog_tables"}],
  "pages": [{"identifier":"service_overview","title":"Service Overview"}]
}`
	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	loader := NewLoader()
	data, err := loader.LoadData(inputPath)
	if err != nil {
		t.Fatalf("LoadData error: %v", err)
	}

	if len(data.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(data.Folders))
	}
	if data.Folders[0]["identifier"] != "quorum" {
		t.Fatalf("expected quorum folder identifier, got %v", data.Folders[0]["identifier"])
	}
	if len(data.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(data.Pages))
	}
}

func TestCleanFolderForCreate(t *testing.T) {
	folder := map[string]interface{}{
		"identifier":  "quorum",
		"title":       "Quorum",
		"after":       "catalog_tables",
		"parent":      "root_catalog",
		"sidebarType": "folder",
		"id":          "internal-id",
	}

	cleaned := CleanFolderForCreate(folder)

	if len(cleaned) != 4 {
		t.Fatalf("expected only identifier/title/after/parent, got %v", cleaned)
	}
	if cleaned["identifier"] != "quorum" || cleaned["title"] != "Quorum" || cleaned["after"] != "catalog_tables" || cleaned["parent"] != "root_catalog" {
		t.Fatalf("unexpected cleaned folder payload: %v", cleaned)
	}
	if _, exists := cleaned["sidebarType"]; exists {
		t.Fatalf("expected sidebarType to be stripped, got %v", cleaned)
	}
}

func TestSortFoldersByAfterLevels(t *testing.T) {
	levels := SortFoldersByAfterLevels([]api.Folder{
		{"identifier": "child", "parent": "root"},
		{"identifier": "sibling", "after": "child"},
		{"identifier": "root"},
		{"identifier": "leaf", "parent": "child", "after": "sibling"},
	})

	if len(levels) != 4 {
		t.Fatalf("expected 4 levels, got %d", len(levels))
	}

	order := []string{
		levels[0][0]["identifier"].(string),
		levels[1][0]["identifier"].(string),
		levels[2][0]["identifier"].(string),
		levels[3][0]["identifier"].(string),
	}
	if strings.Join(order, ",") != "root,child,sibling,leaf" {
		t.Fatalf("unexpected folder order: %v", order)
	}
}

func TestPlanSidebarPipeline_MixedFolderAndPageDependencies(t *testing.T) {
	pipeline := PlanSidebarPipeline(
		[]api.Folder{
			{"identifier": "catalog_root", "title": "Catalog Root"},
			{"identifier": "after_page_folder", "title": "After Page Folder", "after": "service_overview"},
		},
		[]api.Page{
			{"identifier": "service_overview", "title": "Service Overview", "parent": "catalog_root"},
			{"identifier": "service_details", "title": "Service Details", "parent": "after_page_folder"},
		},
	)

	lines := DescribeSidebarPipeline(pipeline)
	expected := []string{
		"Step 1: folders [catalog_root]",
		"Step 2: pages [service_overview]",
		"Step 3: folders [after_page_folder]",
		"Step 4: pages [service_details]",
	}

	if strings.Join(lines, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected pipeline:\n%s", strings.Join(lines, "\n"))
	}
}

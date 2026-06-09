package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestPrintSkillsCatalog(t *testing.T) {
	created := "2024-01-01T00:00:00.000Z"
	updated := "2024-06-01T12:00:00.000Z"
	version := "1.2.3"

	entries := []api.SkillCatalogEntry{
		{
			Skill: api.CatalogEntitySnapshot{
				Identifier: "demo-api-guide",
				Title:      "API Guide",
				Blueprint:  "_skill",
				Properties: map[string]interface{}{"location": "global"},
				Relations:  map[string]interface{}{"skill_active_version": "demo-api-guide_1.2.3"},
				CreatedAt:  &created,
				UpdatedAt:  &updated,
			},
			Version: &api.CatalogEntitySnapshot{
				Identifier: "demo-api-guide_1.2.3",
				Title:      "demo-api-guide 1.2.3",
				Blueprint:  "_skill_version",
				Properties: map[string]interface{}{
					"version":     version,
					"description": "How to use the API",
				},
				CreatedAt: &created,
				UpdatedAt: &updated,
			},
		},
	}

	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printSkillsCatalog(entries)
	w.Close()
	os.Stdout = orig
	io.Copy(&buf, r)

	out := buf.String()
	for _, want := range []string{
		"demo-api-guide",
		"API Guide",
		"global",
		"1.2.3",
		"(published)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDisplayCatalogTitle(t *testing.T) {
	if got := displayCatalogTitle("demo", "demo"); got != "" {
		t.Fatalf("expected empty when title equals identifier, got %q", got)
	}
	if got := displayCatalogTitle("My Skill", "demo"); got != "My Skill" {
		t.Fatalf("got %q", got)
	}
}

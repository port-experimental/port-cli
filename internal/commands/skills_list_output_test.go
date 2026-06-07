package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

func TestPrintSkillsCatalog(t *testing.T) {
	created := "2024-01-01T00:00:00.000Z"
	updated := "2024-06-01T12:00:00.000Z"
	version := "1.2.3"

	entries := []aiservice.SkillCatalogEntry{
		{
			Skill: aiservice.CatalogEntitySnapshot{
				Identifier: "demo-api-guide",
				Title:      "API Guide",
				Blueprint:  "_skill",
				Properties: map[string]interface{}{"location": "global"},
				Relations:  map[string]interface{}{"skill_active_version": "demo-api-guide_1.2.3"},
				CreatedAt:  &created,
				UpdatedAt:  &updated,
			},
			Version: &aiservice.CatalogEntitySnapshot{
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

func TestFormatSkillsListPaginationLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		p          aiservice.SkillsPagination
		shownCount int
		totalPages int
		want       string
	}{
		{
			name:       "first page full",
			p:          aiservice.SkillsPagination{Page: 1, PageSize: 20, Total: 45, TotalPages: 3},
			shownCount: 20,
			totalPages: 3,
			want:       "Showing skills 1–20 of 45 (page 1 of 3)",
		},
		{
			name:       "last page partial",
			p:          aiservice.SkillsPagination{Page: 3, PageSize: 20, Total: 45, TotalPages: 3},
			shownCount: 5,
			totalPages: 3,
			want:       "Showing skills 41–45 of 45 (page 3 of 3)",
		},
		{
			name:       "single skill",
			p:          aiservice.SkillsPagination{Page: 1, PageSize: 20, Total: 1, TotalPages: 1},
			shownCount: 1,
			totalPages: 1,
			want:       "Showing skill 1 of 1 (page 1 of 1)",
		},
		{
			name:       "empty page",
			p:          aiservice.SkillsPagination{Page: 4, PageSize: 20, Total: 45, TotalPages: 3},
			shownCount: 0,
			totalPages: 3,
			want:       "Page 4 of 3 (no skills on this page, 45 total)",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatSkillsListPaginationLine(tc.p, tc.shownCount, tc.totalPages)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
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

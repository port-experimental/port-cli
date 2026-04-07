package commands

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func TestClearPagesFlagParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterClear(rootCmd)

	clearCmd, _, err := rootCmd.Find([]string{"clear"})
	if err != nil || clearCmd == nil {
		t.Fatal("clear command not found")
	}

	if err := clearCmd.ParseFlags([]string{"--pages", "--force"}); err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	pages, err := clearCmd.Flags().GetBool("pages")
	if err != nil {
		t.Fatalf("could not get --pages: %v", err)
	}
	if !pages {
		t.Fatalf("expected --pages to be true")
	}
}

func TestDeleteProtectedFlagParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterClear(rootCmd)

	clearCmd, _, err := rootCmd.Find([]string{"clear"})
	if err != nil || clearCmd == nil {
		t.Fatal("clear command not found")
	}

	if err := clearCmd.ParseFlags([]string{"--pages", "--delete-protected-pages", "--force"}); err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	deleteProtected, err := clearCmd.Flags().GetBool("delete-protected-pages")
	if err != nil {
		t.Fatalf("could not get --delete-protected-pages: %v", err)
	}
	if !deleteProtected {
		t.Fatalf("expected --delete-protected-pages to be true")
	}
}

func TestRootFoldersForDeletionSelectsRootFoldersWithoutSidebarFilter(t *testing.T) {
	folders := []api.Folder{
		{"identifier": "root-a"},
		{"identifier": "hidden-root"},
		{"identifier": "child", "parent": "root-a"},
		{"identifier": "_system"},
	}

	roots := rootFoldersForDeletion(folders, false)
	if len(roots) != 2 {
		t.Fatalf("expected 2 deletable root folders, got %d", len(roots))
	}
	if roots[0]["identifier"] != "root-a" || roots[1]["identifier"] != "hidden-root" {
		t.Fatalf("unexpected root folder selection: %v", roots)
	}
}

func TestRootFoldersForDeletionProtectedAppendedLastWhenEnabled(t *testing.T) {
	folders := []api.Folder{
		{"identifier": "root-a"},
		{"identifier": "with_under_score"},
		{"identifier": "_system"},
		{"identifier": "child", "parent": "root-a"},
	}

	roots := rootFoldersForDeletion(folders, true)
	got := []string{
		roots[0]["identifier"].(string),
		roots[1]["identifier"].(string),
		roots[2]["identifier"].(string),
	}
	want := []string{"root-a", "with_under_score", "_system"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected root folder order: got %v want %v", got, want)
		}
	}
}

func TestRootPagesForDeletionFiltersBySidebarVisibilityFirst(t *testing.T) {
	pages := []api.Page{
		{"identifier": "home", "showInSidebar": true},
		{"identifier": "hidden-home", "showInSidebar": false},
		{"identifier": "_catalog", "showInSidebar": true},
		{"identifier": "details", "parent": "folder-a", "showInSidebar": true},
	}

	roots := rootPagesForDeletion(pages, false)
	if len(roots) != 1 {
		t.Fatalf("expected 1 deletable root page, got %d", len(roots))
	}
	if roots[0]["identifier"] != "home" {
		t.Fatalf("unexpected root page selection: %v", roots)
	}
}

func TestRootPagesForDeletionProtectedAppendedLastWhenEnabled(t *testing.T) {
	pages := []api.Page{
		{"identifier": "home", "showInSidebar": true},
		{"identifier": "with_under_score", "showInSidebar": true},
		{"identifier": "_catalog", "showInSidebar": true},
		{"identifier": "hidden-home", "showInSidebar": false},
	}

	roots := rootPagesForDeletion(pages, true)
	got := []string{
		roots[0]["identifier"].(string),
		roots[1]["identifier"].(string),
		roots[2]["identifier"].(string),
	}
	want := []string{"home", "with_under_score", "_catalog"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected root page order: got %v want %v", got, want)
		}
	}
}

func TestIsDeletablePage(t *testing.T) {
	if !isDeletablePage(api.Page{"showInSidebar": true}) {
		t.Fatal("expected showInSidebar=true page to be deletable")
	}
	if isDeletablePage(api.Page{"showInSidebar": false}) {
		t.Fatal("expected showInSidebar=false page to be skipped")
	}
	if isDeletablePage(api.Page{}) {
		t.Fatal("expected page without showInSidebar to be skipped")
	}
}

func TestIsProtectedSidebarItemIdentifier(t *testing.T) {
	if isProtectedSidebarItemIdentifier("plain") {
		t.Fatal("expected plain identifier to be non-protected")
	}
	if isProtectedSidebarItemIdentifier("with_under_score") {
		t.Fatal("expected identifier with underscore (but not leading) to be non-protected")
	}
	if !isProtectedSidebarItemIdentifier("_system") {
		t.Fatal("expected underscore-prefixed identifier to be protected")
	}
}

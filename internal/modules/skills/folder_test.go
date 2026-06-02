package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackSkillFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: my-skill
description: Demo skill
---
# Instructions
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}

	pack, err := PackSkillFolder(dir, PackSkillFolderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Identifier != "my-skill" {
		t.Fatalf("identifier = %q", pack.Identifier)
	}
	if pack.Description != "Demo skill" {
		t.Fatalf("description = %q", pack.Description)
	}
	if len(pack.Files) < 2 {
		t.Fatalf("files = %+v", pack.Files)
	}
	if pack.Location != "global" {
		t.Fatalf("location = %q, want global", pack.Location)
	}
}

func TestPackSkillFolder_locationFromFlag(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, "# Skill\n")

	pack, err := PackSkillFolder(dir, PackSkillFolderOptions{Location: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Location != "project" {
		t.Fatalf("location = %q", pack.Location)
	}
}

func TestPackSkillFolder_locationFromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, `---
location: project
---
# Skill
`)

	pack, err := PackSkillFolder(dir, PackSkillFolderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Location != "project" {
		t.Fatalf("location = %q", pack.Location)
	}
}

func TestPackSkillFolder_flagOverridesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, `---
location: project
---
# Skill
`)

	pack, err := PackSkillFolder(dir, PackSkillFolderOptions{Location: "global"})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Location != "global" {
		t.Fatalf("location = %q", pack.Location)
	}
}

func TestNormalizeSkillLocation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "global", false},
		{"global", "global", false},
		{"PROJECT", "project", false},
		{"invalid", "", true},
	} {
		got, err := NormalizeSkillLocation(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("NormalizeSkillLocation(%q) expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeSkillLocation(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("got %q want %q", got, tt.want)
		}
	}
}

func writeSkillMD(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackSkillFolder_requiresSkillMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := PackSkillFolder(dir, PackSkillFolderOptions{})
	if err == nil {
		t.Fatal("expected error without SKILL.md")
	}
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackSkillFolder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestPackSkillFolder_symlinkedSkillDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(`---
name: find-skills
description: Via symlink
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "find-skills")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	pack, err := PackSkillFolder(link, PackSkillFolderOptions{})
	if err != nil {
		t.Fatalf("PackSkillFolder symlink: %v", err)
	}
	if pack.Identifier != "find-skills" {
		t.Fatalf("identifier = %q", pack.Identifier)
	}
	if len(pack.Files) != 1 || pack.Files[0].Path != "SKILL.md" {
		t.Fatalf("files = %+v", pack.Files)
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

func TestPackSkillFolder_rejectsNameFolderMismatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my-folder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkillMD(t, dir, `---
name: other-name
description: Demo
---
# Skill
`)
	_, err := PackSkillFolder(dir, PackSkillFolderOptions{})
	if err == nil {
		t.Fatal("expected error when folder name and SKILL.md name differ")
	}
	if !strings.Contains(err.Error(), "does not match SKILL.md name") {
		t.Fatalf("unexpected error: %v", err)
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

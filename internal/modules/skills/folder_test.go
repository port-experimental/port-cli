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

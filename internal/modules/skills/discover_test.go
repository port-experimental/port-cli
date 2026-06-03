package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkillRoots_singleSkill(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	roots, err := DiscoverSkillRoots(dir)
	if err != nil {
		t.Fatalf("DiscoverSkillRoots: %v", err)
	}
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d: %v", len(roots), roots)
	}
	if roots[0] != dir {
		t.Fatalf("expected root %q, got %q", dir, roots[0])
	}
}

func TestDiscoverSkillRoots_bundle(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"skill-a", "skill-b"} {
		child := filepath.Join(parent, name)
		if err := os.Mkdir(child, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(child, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	roots, err := DiscoverSkillRoots(parent)
	if err != nil {
		t.Fatalf("DiscoverSkillRoots: %v", err)
	}
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d: %v", len(roots), roots)
	}
}

func TestDiscoverSkillRoots_none(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverSkillRoots(dir)
	if err == nil {
		t.Fatal("expected error when no SKILL.md found")
	}
}

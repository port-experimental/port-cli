package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckGitCleanForWriteRoots_DirtySkips(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	skillsPort := filepath.Join(dir, ".cursor", "skills", PortSkillsDir)
	if err := os.MkdirAll(skillsPort, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsPort, "dirty.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	roots := []WriteRoot{{AbsPath: dir, SkillsPortRel: ".cursor/skills/" + PortSkillsDir}}
	result, err := CheckGitCleanForWriteRoots(roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CleanRoots) != 0 || len(result.DirtyRoots) != 1 {
		t.Fatalf("expected dirty root, got clean=%d dirty=%d", len(result.CleanRoots), len(result.DirtyRoots))
	}
}

func TestCheckGitCleanForWriteRoots_CleanAllows(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	skillsPort := filepath.Join(dir, ".cursor", "skills", PortSkillsDir)
	if err := os.MkdirAll(skillsPort, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")

	roots := []WriteRoot{{AbsPath: dir, SkillsPortRel: ".cursor/skills/" + PortSkillsDir}}
	result, err := CheckGitCleanForWriteRoots(roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CleanRoots) != 1 || len(result.DirtyRoots) != 0 {
		t.Fatalf("expected clean root, got clean=%d dirty=%d", len(result.CleanRoots), len(result.DirtyRoots))
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

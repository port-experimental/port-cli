package skills

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WriteRoot is a directory tree where skills/port may be written.
type WriteRoot struct {
	AbsPath       string
	SkillsPortRel string
}

// GitWriteGuardResult reports per-root git cleanliness before WriteSkills.
type GitWriteGuardResult struct {
	CleanRoots  []WriteRoot
	DirtyRoots  []WriteRoot
	SkippedWarn []string
}

// CheckGitCleanForWriteRoots returns roots that are safe to write (no uncommitted changes under skills/port).
func CheckGitCleanForWriteRoots(roots []WriteRoot) (GitWriteGuardResult, error) {
	var result GitWriteGuardResult
	for _, root := range roots {
		clean, err := isSkillsPortPathClean(root.AbsPath, root.SkillsPortRel)
		if err != nil {
			return result, err
		}
		if clean {
			result.CleanRoots = append(result.CleanRoots, root)
		} else {
			result.DirtyRoots = append(result.DirtyRoots, root)
			result.SkippedWarn = append(result.SkippedWarn,
				fmt.Sprintf("%s (uncommitted changes under %s)", root.AbsPath, root.SkillsPortRel))
		}
	}
	return result, nil
}

// SkillsWriteRoots derives git-check roots from global targets and project directories.
func SkillsWriteRoots(globalTargets, projectDirs []string) []WriteRoot {
	seen := make(map[string]bool)
	var roots []WriteRoot
	add := func(absTarget string) {
		absTarget = expandHome(absTarget)
		portDir := filepath.Join(absTarget, "skills", PortSkillsDir)
		repoRoot, rel, ok := gitRepoRootAndRelPath(portDir)
		if !ok {
			return
		}
		key := repoRoot + "\x00" + rel
		if seen[key] {
			return
		}
		seen[key] = true
		roots = append(roots, WriteRoot{AbsPath: repoRoot, SkillsPortRel: rel})
	}
	for _, t := range globalTargets {
		add(t)
	}
	for _, d := range projectDirs {
		for _, sub := range projectSkillTargetSubdirs(d) {
			add(sub)
		}
	}
	return roots
}

func projectSkillTargetSubdirs(projectDir string) []string {
	projectDir = expandHome(projectDir)
	subs := []string{
		filepath.Join(projectDir, ".cursor"),
		filepath.Join(projectDir, ".claude"),
		filepath.Join(projectDir, ".gemini"),
		filepath.Join(projectDir, ".codex"),
		filepath.Join(projectDir, ".codeium", "windsurf"),
		filepath.Join(projectDir, ".github"),
	}
	return subs
}

func gitRepoRootAndRelPath(absPortDir string) (repoRoot, rel string, ok bool) {
	absPortDir, err := filepath.Abs(absPortDir)
	if err != nil {
		return "", "", false
	}
	cmd := exec.Command("git", "-C", absPortDir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", "", false
	}
	repoRoot = strings.TrimSpace(string(out))
	rel, err = filepath.Rel(repoRoot, absPortDir)
	if err != nil {
		return "", "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		rel = ""
	}
	return repoRoot, rel, true
}

func isSkillsPortPathClean(repoRoot, relPath string) (bool, error) {
	args := []string{"-C", repoRoot, "status", "--porcelain"}
	if relPath != "" {
		args = append(args, "--", relPath)
	}
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if _, lookErr := exec.LookPath("git"); lookErr != nil {
			return true, nil
		}
		return false, fmt.Errorf("git status failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return len(bytes.TrimSpace(out)) == 0, nil
}

// FilterTargetsByCleanGit returns only targets whose skills/port path is git-clean.
func FilterTargetsByCleanGit(globalTargets, projectDirs []string, ignore bool) (cleanGlobal []string, cleanProjectDirs []string, guard GitWriteGuardResult, err error) {
	if ignore {
		return globalTargets, projectDirs, GitWriteGuardResult{CleanRoots: SkillsWriteRoots(globalTargets, projectDirs)}, nil
	}
	roots := SkillsWriteRoots(globalTargets, projectDirs)
	guard, err = CheckGitCleanForWriteRoots(roots)
	if err != nil {
		return nil, nil, guard, err
	}
	cleanSet := make(map[string]bool)
	for _, r := range guard.CleanRoots {
		if r.SkillsPortRel == "" {
			cleanSet[r.AbsPath] = true
			continue
		}
		full := filepath.Join(r.AbsPath, filepath.FromSlash(r.SkillsPortRel))
		cleanSet[filepath.Clean(full)] = true
	}
	for _, t := range globalTargets {
		portDir := filepath.Clean(filepath.Join(expandHome(t), "skills", PortSkillsDir))
		if _, _, inRepo := gitRepoRootAndRelPath(portDir); !inRepo {
			cleanGlobal = append(cleanGlobal, t)
			continue
		}
		if cleanSet[portDir] {
			cleanGlobal = append(cleanGlobal, t)
		}
	}
	for _, d := range projectDirs {
		keep := false
		for _, sub := range projectSkillTargetSubdirs(d) {
			portDir := filepath.Clean(filepath.Join(expandHome(sub), "skills", PortSkillsDir))
			if _, _, inRepo := gitRepoRootAndRelPath(portDir); !inRepo {
				keep = true
				break
			}
			if cleanSet[portDir] {
				keep = true
				break
			}
		}
		if keep {
			cleanProjectDirs = append(cleanProjectDirs, d)
		}
	}
	return cleanGlobal, cleanProjectDirs, guard, nil
}

// WriteSkillsGitSkipped reports whether any roots were skipped for dirty git.
func WriteSkillsGitSkipped(guard GitWriteGuardResult) bool {
	return len(guard.DirtyRoots) > 0
}

func gitDirtyMessage(guard GitWriteGuardResult) string {
	if len(guard.SkippedWarn) == 0 {
		return ""
	}
	return "skipped writing skills due to uncommitted git changes:\n  - " + strings.Join(guard.SkippedWarn, "\n  - ")
}

func warnGitDirty(osStderr func(string, ...any), guard GitWriteGuardResult) {
	msg := gitDirtyMessage(guard)
	if msg != "" {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", msg)
	}
}

//go:build e2e

package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var frontmatterNameLine = regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`)

func portSkillsRoot(cursorDir string) string {
	return filepath.Join(cursorDir, "skills", "port")
}

func resetPortSkillsDir(cursorDir string) error {
	root := portSkillsRoot(cursorDir)
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return os.MkdirAll(root, 0o755)
}

func findSkillMD(portRoot, skillID string) (string, error) {
	if skillID == "" {
		return "", fmt.Errorf("skill id is required")
	}
	var found string
	err := filepath.WalkDir(portRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := frontmatterNameLine.FindSubmatch(data); len(m) == 2 && string(m[1]) == skillID {
			if found != "" {
				return fmt.Errorf("multiple SKILL.md files for skill %q under %s", skillID, portRoot)
			}
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("SKILL.md for skill %q not found under %s", skillID, portRoot)
	}
	return found, nil
}

func skillPresent(portRoot, skillID string) bool {
	_, err := findSkillMD(portRoot, skillID)
	return err == nil
}

func readSkillMD(portRoot, skillID string) (string, error) {
	path, err := findSkillMD(portRoot, skillID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func listSyncedSkillIDs(portRoot string) ([]string, error) {
	var ids []string
	err := filepath.WalkDir(portRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := frontmatterNameLine.FindSubmatch(data); len(m) == 2 {
			ids = append(ids, string(m[1]))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func parseFrontmatterName(content string) string {
	if m := frontmatterNameLine.FindStringSubmatch(content); len(m) == 2 {
		return m[1]
	}
	return ""
}

// distinctiveSnippet returns a stable substring from skill body text for regression checks.
func distinctiveSnippet(skillMD string) string {
	body := skillMD
	if idx := strings.Index(skillMD, "\n---"); idx >= 0 {
		rest := skillMD[idx+len("\n---"):]
		if j := strings.Index(rest, "\n"); j >= 0 {
			body = strings.TrimSpace(rest[j+1:])
		}
	}
	best := ""
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 24 {
			continue
		}
		if strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		if len(line) > len(best) {
			best = line
		}
	}
	return best
}

func assertOnlySeedCatalogSkills(t testingT, portRoot string, allowed []string) {
	t.Helper()
	allowedSet := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = true
	}
	ids, err := listSyncedSkillIDs(portRoot)
	if err != nil {
		t.Fatalf("list synced skills: %v", err)
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "e2e-") {
			continue
		}
		if !seedCatalogSkillIDs[id] {
			continue
		}
		if !allowedSet[id] {
			t.Fatalf("unexpected seed catalog skill %q on disk (allowed: %v)", id, allowed)
		}
	}
}

type testingT interface {
	Helper()
	Fatalf(string, ...any)
}

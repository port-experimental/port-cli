package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DiscoverSkillRoots returns skill directory paths under path.
// If path contains SKILL.md at its root, returns [path].
// Otherwise scans immediate child directories for SKILL.md (bundle layout).
func DiscoverSkillRoots(path string) ([]string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve skill path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("skill path %q: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skill path %q is not a directory", path)
	}

	if _, err := os.Stat(filepath.Join(abs, "SKILL.md")); err == nil {
		return []string{abs}, nil
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read skill directory %q: %w", path, err)
	}

	var roots []string
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "." || entry.Name() == ".." {
			continue
		}
		child := filepath.Join(abs, entry.Name())
		if _, err := os.Stat(filepath.Join(child, "SKILL.md")); err != nil {
			continue
		}
		roots = append(roots, child)
	}
	sort.Strings(roots)
	if len(roots) == 0 {
		return nil, fmt.Errorf("no skill directories found under %q (expected SKILL.md at root or in immediate subdirectories)", path)
	}
	return roots, nil
}

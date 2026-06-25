package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func filterOrphanSkillFiles(skill Skill, files []SkillFile) []SkillFile {
	filtered := make([]SkillFile, 0, len(files))
	for _, file := range files {
		if isOrphanSkillFile(skill, file.Path) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func isOrphanSkillFile(skill Skill, path string) bool {
	parts, ok := pathPartsAfterSkillsDir(path)
	if !ok {
		return false
	}
	skillDirName, err := skillDirName(skill)
	if err != nil {
		return true
	}
	_, found := trimToSkillDir(parts, skillDirName, skill)
	return !found
}

// FilterSkills returns skills matching the provided selection criteria.
func FilterSkills(fetched *FetchedSkills, selectAll, selectAllGroups, selectAllUngrouped bool, selectedGroups, selectedSkills []string, serverFilteredGroups bool) []Skill {
	if selectAll {
		return append([]Skill(nil), fetched.Skills...)
	}

	selectedGroupSet := toSet(selectedGroups)
	selectedSkillSet := toSet(selectedSkills)

	var result []Skill
	for _, s := range fetched.Skills {
		ungrouped := len(s.GroupIDs) == 0
		if !ungrouped && serverFilteredGroups {
			result = append(result, s)
			continue
		}
		switch {
		case ungrouped && selectAllUngrouped:
			result = append(result, s)
		case ungrouped && selectedSkillSet[s.Identifier]:
			result = append(result, s)
		case !ungrouped && selectAllGroups:
			result = append(result, s)
		case !ungrouped && anyGroupSelected(selectedGroupSet, s.GroupIDs):
			result = append(result, s)
		case selectedSkillSet[s.Identifier]:
			result = append(result, s)
		}
	}
	return result
}

func anyGroupSelected(selectedGroupSet map[string]bool, groupIDs []string) bool {
	for _, gid := range groupIDs {
		if selectedGroupSet[gid] {
			return true
		}
	}
	return false
}

// GroupName resolves the display name for a group, falling back to its identifier.
func GroupName(groups []SkillGroup, groupID string) string {
	for _, g := range groups {
		if g.Identifier == groupID {
			if g.Title != "" {
				return g.Title
			}
			return g.Identifier
		}
	}
	if groupID != "" {
		return groupID
	}
	return NoGroupDir
}

type skillKey struct{ group, skill string }

// WriteSkills writes SKILL.md files (plus references, assets, scripts, and
// additional files) for each skill,
// routing each one based on its Location property:
//   - SkillLocationGlobal  → written into every dir in globalTargets
//   - SkillLocationProject → written into the matching tool sub-directory
//     inside every projectDir (e.g. <projectDir>/.agents/skills/port/…)
func WriteSkills(skills []Skill, groups []SkillGroup, globalTargets []string, projectDirs []string) error {
	globalSkills := make([]Skill, 0, len(skills))
	projectSkills := make([]Skill, 0)
	for _, s := range skills {
		if s.Location == SkillLocationProject {
			projectSkills = append(projectSkills, s)
		} else {
			globalSkills = append(globalSkills, s)
		}
	}

	skillsByPortDir := make(map[string][]Skill)
	addSkillsForTargets := func(targets []string, list []Skill) {
		for _, target := range targets {
			portDir := portSkillsDirForTarget(target)
			skillsByPortDir[portDir] = append(skillsByPortDir[portDir], list...)
		}
	}
	addSkillsForTargets(globalTargets, globalSkills)
	if len(projectDirs) > 0 && len(projectSkills) > 0 {
		addSkillsForTargets(buildProjectTargets(globalTargets, projectDirs), projectSkills)
	}

	for portDir, list := range skillsByPortDir {
		if err := writeSkillsToPortDir(mergeSkillsByIdentifier(list), groups, portDir); err != nil {
			return err
		}
	}
	return nil
}

func portSkillsDirForTarget(target string) string {
	return filepath.Join(expandHome(target), "skills", PortSkillsDir)
}

func mergeSkillsByIdentifier(skills []Skill) []Skill {
	if len(skills) == 0 {
		return nil
	}
	byID := make(map[string]Skill, len(skills))
	order := make([]string, 0, len(skills))
	for _, s := range skills {
		if _, seen := byID[s.Identifier]; !seen {
			order = append(order, s.Identifier)
		}
		byID[s.Identifier] = s
	}
	out := make([]Skill, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

// buildProjectTargets creates project-level target paths by combining each
// project directory with the tool sub-directory derived from the global
// targets. When a tool defines a ProjectDir override, that name is used under
// each project dir instead of Dir (rare; most tools use Dir only).
func buildProjectTargets(globalTargets []string, projectDirs []string) []string {
	toolDirs := extractProjectDirs(globalTargets)
	seen := make(map[string]bool)
	var result []string
	for _, pd := range projectDirs {
		for _, td := range toolDirs {
			p := filepath.Join(pd, td)
			if !seen[p] {
				result = append(result, p)
				seen[p] = true
			}
		}
	}
	return result
}

// extractProjectDirs returns the relative directory names to use for
// project-scoped skills. For each global target it checks known hook targets:
// if the target has a ProjectDir override that directory is used, otherwise
// the target's Dir is used. Unrecognized paths fall back to the base name.
// Legacy GitHub Copilot paths ending in /.copilot map to ".github".
func extractProjectDirs(globalTargets []string) []string {
	knownTargets := DefaultHookTargets()
	seen := make(map[string]bool)
	var dirs []string
	for _, gt := range globalTargets {
		expanded := expandHome(gt)
		matched := false
		for _, kt := range knownTargets {
			if matchesTarget(expanded, kt) {
				d := kt.Dir
				if kt.ProjectDir != "" {
					d = kt.ProjectDir
				}
				if !seen[d] {
					dirs = append(dirs, d)
					seen[d] = true
				}
				matched = true
				break
			}
		}
		if !matched {
			base := filepath.Base(expanded)
			if !seen[base] {
				dirs = append(dirs, base)
				seen[base] = true
			}
		}
	}
	return dirs
}

func writeSkillsToPortDir(skills []Skill, groups []SkillGroup, portDir string) error {
	expected := make(map[skillKey]bool)
	for _, s := range skills {
		skillDirName, err := skillDirName(s)
		if err != nil {
			return err
		}
		groupDirs, err := skillGroupDirs(s, groups)
		if err != nil {
			return err
		}
		for _, groupDir := range groupDirs {
			expected[skillKey{groupDir, skillDirName}] = true
		}
	}

	for _, s := range skills {
		skillDirName, err := skillDirName(s)
		if err != nil {
			return err
		}
		groupDirs, err := skillGroupDirs(s, groups)
		if err != nil {
			return err
		}
		for _, groupDir := range groupDirs {
			skillDir := filepath.Join(portDir, groupDir, skillDirName)
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				return fmt.Errorf("failed to create skill directory %s: %w", skillDir, err)
			}

			if err := writeSkillFiles(skillDir, skillDirName, s); err != nil {
				return err
			}
		}
	}

	if err := reconcileSkills(portDir, expected); err != nil {
		return fmt.Errorf("reconciliation failed for %s: %w", portDir, err)
	}
	return nil
}

// skillGroupDirs returns the list of group directory names for a skill.
// Skills without any group use NoGroupDir; multi-group skills return one entry per group.
func skillGroupDirs(s Skill, groups []SkillGroup) ([]string, error) {
	if len(s.GroupIDs) == 0 {
		return []string{NoGroupDir}, nil
	}
	dirs := make([]string, 0, len(s.GroupIDs))
	for _, gid := range s.GroupIDs {
		dir, err := groupDirName(gid, groups)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

func reconcileSkills(portDir string, expected map[skillKey]bool) error {
	groupEntries, err := os.ReadDir(portDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cleanPortDir := filepath.Clean(portDir) + string(filepath.Separator)

	for _, groupEntry := range groupEntries {
		if !groupEntry.IsDir() {
			continue
		}
		groupName := groupEntry.Name()
		if !isSafeDirName(groupName) {
			continue
		}
		cleanGroupPath := filepath.Clean(filepath.Join(portDir, groupName))
		if !strings.HasPrefix(cleanGroupPath+string(filepath.Separator), cleanPortDir) {
			continue
		}

		skillEntries, err := os.ReadDir(cleanGroupPath)
		if err != nil {
			continue
		}

		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			skillName := skillEntry.Name()
			if !isSafeDirName(skillName) {
				continue
			}
			cleanSkillPath := filepath.Clean(filepath.Join(cleanGroupPath, skillName))
			if !strings.HasPrefix(cleanSkillPath+string(filepath.Separator), cleanGroupPath+string(filepath.Separator)) {
				continue
			}
			key := skillKey{groupName, skillName}
			if !expected[key] {
				if err := os.RemoveAll(cleanSkillPath); err != nil {
					return fmt.Errorf("failed to remove stale skill %s/%s: %w", groupName, skillName, err)
				}
			}
		}

		remaining, _ := os.ReadDir(cleanGroupPath)
		if len(remaining) == 0 {
			_ = os.Remove(cleanGroupPath)
		}
	}
	return nil
}

// isSafeDirName returns true if name is a plain directory basename with no path
// traversal sequences or separators. This prevents path traversal when names
// sourced from os.ReadDir are used in subsequent file operations.
func isSafeDirName(name string) bool {
	return name != "." && name != ".." && !strings.ContainsAny(name, "/\\")
}

func writeSkillFiles(skillDir, skillDirName string, s Skill) error {
	hasSkillMD := false
	for _, f := range filterOrphanSkillFiles(s, s.Files) {
		relPath, err := normalizeSkillFilePath(f.Path, skillDirName, s)
		if err != nil {
			return fmt.Errorf("failed to write file %s for skill %s: %w", f.Path, s.Identifier, err)
		}
		file := f
		if relPath == "SKILL.md" {
			hasSkillMD = true
			file.Content = normalizeSkillMDContent(s, skillDirName, f.Content)
		}
		if err := writeSkillFile(skillDir, SkillFile{Path: relPath, Content: file.Content}); err != nil {
			return fmt.Errorf("failed to write file %s for skill %s: %w", f.Path, s.Identifier, err)
		}
	}
	if !hasSkillMD {
		return fmt.Errorf("skill %s has no SKILL.md in catalog files", s.Identifier)
	}
	return nil
}

func writeSkillFile(skillDir string, f SkillFile) error {
	dest := filepath.Join(skillDir, filepath.FromSlash(f.Path))
	cleanDest := filepath.Clean(dest)
	cleanBase := filepath.Clean(skillDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanDest+string(filepath.Separator), cleanBase) {
		return fmt.Errorf("skill file path %q escapes skill directory", f.Path)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dest, err)
	}
	return os.WriteFile(dest, []byte(f.Content), 0o644)
}

func normalizeSkillFilePath(path, skillDirName string, s Skill) (string, error) {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == "" || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("skill file path %q escapes skill directory", path)
	}

	parts := strings.Split(path, "/")
	if skillsParts, ok := pathPartsAfterSkillsDir(path); ok {
		trimmedParts, found := trimToSkillDir(skillsParts, skillDirName, s)
		if !found {
			return "", fmt.Errorf("skill file path %q is not inside a skill directory", path)
		}
		parts = trimmedParts
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("skill file path %q escapes skill directory", path)
	}
	return strings.Join(parts, "/"), nil
}

func pathPartsAfterSkillsDir(path string) ([]string, bool) {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "skills" {
			return parts[i+1:], true
		}
	}
	return nil, false
}

func trimToSkillDir(parts []string, skillDirName string, s Skill) ([]string, bool) {
	for i := 0; i < len(parts); i++ {
		if isSkillDirPart(parts[i], skillDirName, s) && i+1 < len(parts) {
			return parts[i+1:], true
		}
	}
	return nil, false
}

func isSkillDirPart(part, skillDirName string, s Skill) bool {
	return part == skillDirName || part == s.Title || part == skillIdentifierBase(s.Identifier)
}

func skillIdentifierBase(identifier string) string {
	identifier = strings.Trim(identifier, "/\\")
	if identifier == "" {
		return ""
	}
	return filepath.Base(filepath.ToSlash(identifier))
}

func groupDirName(groupID string, groups []SkillGroup) (string, error) {
	if validatePathComponent(groupID) == nil {
		return groupID, nil
	}
	for _, group := range groups {
		if group.Identifier == groupID && validatePathComponent(group.Title) == nil {
			return group.Title, nil
		}
	}
	return "", fmt.Errorf("invalid group ID %q: %w", groupID, validatePathComponent(groupID))
}

func skillDirName(s Skill) (string, error) {
	name, err := agentSkillNameFromIdentifier(s.Identifier)
	if err != nil {
		return "", fmt.Errorf("invalid skill directory name for %q: %w", s.Identifier, err)
	}
	return name, nil
}

func normalizeSkillMDContent(s Skill, skillName, content string) string {
	description := strings.TrimSpace(s.Description)
	if description == "" {
		description = frontmatterValue(content, "description")
	}
	if description == "" {
		description = fmt.Sprintf("Port skill %s.", skillName)
	}
	return upsertSkillMDFrontmatter(content, skillName, description)
}

func upsertSkillMDFrontmatter(content, skillName, description string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := []string{
		fmt.Sprintf("name: %s", skillName),
		fmt.Sprintf("description: %s", sanitizeFrontmatterScalar(description)),
	}
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[len("---\n"):], "\n---")
		if end >= 0 {
			end += len("---\n")
			block := content[len("---\n"):end]
			body := content[end+len("\n---"):]
			for _, line := range strings.Split(block, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "name:") || strings.HasPrefix(trimmed, "description:") {
					continue
				}
				lines = append(lines, line)
			}
			return "---\n" + strings.Join(lines, "\n") + "\n---" + body
		}
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n\n" + content
}

func frontmatterValue(content, key string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	end := strings.Index(content[len("---\n"):], "\n---")
	if end < 0 {
		return ""
	}
	block := content[len("---\n") : len("---\n")+end]
	for _, line := range strings.Split(block, "\n") {
		foundKey, val, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(foundKey) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return ""
}

func sanitizeFrontmatterScalar(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func validatePathComponent(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("contains invalid path characters")
	}
	return nil
}

func toSet(slice []string) map[string]bool {
	s := make(map[string]bool, len(slice))
	for _, v := range slice {
		s[v] = true
	}
	return s
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home := userHomeDir(); home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func collectPortSkillDirs(globalTargets, projectDirs []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	add := func(targets []string) {
		for _, target := range targets {
			portDir := portSkillsDirForTarget(target)
			if !seen[portDir] {
				seen[portDir] = true
				dirs = append(dirs, portDir)
			}
		}
	}
	add(globalTargets)
	if len(projectDirs) > 0 {
		add(buildProjectTargets(globalTargets, projectDirs))
	}
	return dirs
}

// UnloadSkillFromTargets removes local copies of a skill under skills/port/ for every target.
func UnloadSkillFromTargets(identifier string, globalTargets, projectDirs []string) error {
	for _, portDir := range collectPortSkillDirs(globalTargets, projectDirs) {
		if err := removeSkillFromPortDir(portDir, identifier); err != nil {
			return err
		}
	}
	return nil
}

func removeSkillFromPortDir(portDir, identifier string) error {
	groupEntries, err := os.ReadDir(portDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cleanPortDir := filepath.Clean(portDir) + string(filepath.Separator)
	for _, groupEntry := range groupEntries {
		if !groupEntry.IsDir() || !isSafeDirName(groupEntry.Name()) {
			continue
		}
		groupPath := filepath.Join(portDir, groupEntry.Name())
		if !strings.HasPrefix(filepath.Clean(groupPath)+string(filepath.Separator), cleanPortDir) {
			continue
		}
		skillEntries, err := os.ReadDir(groupPath)
		if err != nil {
			continue
		}
		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() || !isSafeDirName(skillEntry.Name()) {
				continue
			}
			if !matchesSkillDirName(skillEntry.Name(), identifier) {
				continue
			}
			skillPath := filepath.Join(groupPath, skillEntry.Name())
			if err := os.RemoveAll(skillPath); err != nil {
				return fmt.Errorf("failed to remove skill %s: %w", skillPath, err)
			}
		}
	}
	return nil
}

func matchesSkillDirName(dirName, identifier string) bool {
	if dirName == identifier {
		return true
	}
	return dirName == skillIdentifierBase(identifier)
}

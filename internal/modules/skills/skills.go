package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// SkillFile represents a bundled file attached to a skill (reference, asset,
// script, or additional file).
type SkillFile struct {
	Path    string
	Content string
}

// SkillLocation controls where a skill is written on disk.
type SkillLocation string

const (
	SkillLocationGlobal  SkillLocation = "global"
	SkillLocationProject SkillLocation = "project"
)

// Skill holds the data for a single skill entity fetched from Port.
type Skill struct {
	Identifier      string
	Title           string
	Description     string
	Instructions    string
	GroupIDs        []string
	Required        bool
	AutoSync        bool
	Location        SkillLocation
	Versioned       bool
	Files           []SkillFile
	References      []SkillFile
	Assets          []SkillFile
	Scripts         []SkillFile
	AdditionalFiles []SkillFile
}

// SkillGroup holds the data for a single skill_group entity fetched from Port.
type SkillGroup struct {
	Identifier string
	Title      string
	Required   bool
	AutoSync   bool
	SkillIDs   []string
}

// FetchedSkills contains skills split by whether they are required.
type FetchedSkills struct {
	Required []Skill
	Optional []Skill
	Groups   []SkillGroup
}

// FetchSkills retrieves all skill groups and skills from the Port API and
// partitions them into required vs optional.
func FetchSkills(ctx context.Context, client *api.Client) (*FetchedSkills, error) {
	groupEntities, err := client.GetSkillGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill groups: %w", err)
	}

	skillEntities, err := client.GetSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skills: %w", err)
	}

	return ParseFetchedSkills(groupEntities, skillEntities), nil
}

func isMissingSkillBlueprintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Match Port-specific blueprint-not-found error codes and the HTTP 404
	// status. Deliberately avoids broad substrings like "not found" or "does
	// not exist" that could shadow unrelated errors (e.g. bad API URL).
	return strings.Contains(msg, "404") ||
		strings.Contains(msg, "blueprint_not_found") ||
		strings.Contains(msg, "blueprint does not exist")
}

// LoadLatestVersionFiles enriches selected skills with only the latest
// skill_version and its related skill_file entities. Legacy organizations that
// do not have the versioned blueprints keep the original skill JSON content.
func LoadLatestVersionFiles(ctx context.Context, client *api.Client, skills []Skill) ([]Skill, error) {
	skillIDs := skillIdentifiers(skills)
	versions, err := client.GetSkillVersionsForSkills(ctx, skillIDs)
	if err != nil {
		if isMissingSkillBlueprintError(err) {
			return loadLegacySkillContent(ctx, client, skills)
		}
		return nil, fmt.Errorf("failed to fetch skill versions: %w", err)
	}

	latestVersionBySkill := latestVersionsBySkill(versions)
	versionsByID := make(map[string]api.Entity, len(latestVersionBySkill))
	versionIDs := make([]string, 0, len(latestVersionBySkill))
	for _, version := range latestVersionBySkill {
		versionID := stringProp(version, "identifier")
		if versionID == "" {
			continue
		}
		versionsByID[versionID] = version
		versionIDs = append(versionIDs, versionID)
	}

	fileEntities, err := client.GetSkillFilesForVersions(ctx, versionIDs)
	if err != nil {
		if isMissingSkillBlueprintError(err) {
			return loadLegacySkillContent(ctx, client, skills)
		}
		return nil, fmt.Errorf("failed to fetch skill files: %w", err)
	}
	filesByVersion := filesByVersion(fileEntities)

	enriched := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		version := latestVersionBySkill[skill.Identifier]
		if version == nil {
			enriched = append(enriched, skill)
			continue
		}
		versionID := stringProp(version, "identifier")
		versionProps, _ := versionsByID[versionID]["properties"].(map[string]interface{})
		skill.Description = firstNonEmpty(stringFromMap(versionProps, "description"), skill.Description)
		skill.Files = filesByVersion[versionID]
		skill.Versioned = true
		skill.Files = filterOrphanSkillFiles(skill, skill.Files)
		if !hasSyncableContent(skill) {
			continue
		}
		enriched = append(enriched, skill)
	}

	return enriched, nil
}

func loadLegacySkillContent(ctx context.Context, client *api.Client, skills []Skill) ([]Skill, error) {
	// Fetch full skill entities (all properties) without an include filter so
	// that legacy fields like instructions, references, assets, and scripts are
	// present. SearchEntities is used here for pagination; omitting the include
	// key causes Port to return all properties.
	entities, err := client.SearchEntities(ctx, "skill", map[string]interface{}{
		"limit": 1000,
		"query": map[string]interface{}{
			"combinator": "and",
			"rules":      []map[string]interface{}{},
		},
	})
	if err != nil {
		return nil, err
	}
	legacyByID := make(map[string]Skill, len(entities))
	for _, entity := range entities {
		props, _ := entity["properties"].(map[string]interface{})
		id := stringProp(entity, "identifier")
		if id == "" {
			continue
		}
		legacyByID[id] = Skill{
			Description:     stringFromMap(props, "description"),
			Instructions:    stringFromMap(props, "instructions"),
			References:      parseSkillFiles(props, "references"),
			Assets:          parseSkillFiles(props, "assets"),
			Scripts:         parseSkillFiles(props, "scripts"),
			AdditionalFiles: parseSkillFiles(props, "additional_files"),
		}
	}

	enriched := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		if legacy, ok := legacyByID[skill.Identifier]; ok {
			skill.Description = firstNonEmpty(skill.Description, legacy.Description)
			skill.Instructions = firstNonEmpty(skill.Instructions, legacy.Instructions)
			if len(skill.References) == 0 {
				skill.References = legacy.References
			}
			if len(skill.Assets) == 0 {
				skill.Assets = legacy.Assets
			}
			if len(skill.Scripts) == 0 {
				skill.Scripts = legacy.Scripts
			}
			if len(skill.AdditionalFiles) == 0 {
				skill.AdditionalFiles = legacy.AdditionalFiles
			}
		}
		enriched = append(enriched, skill)
	}
	return enriched, nil
}

// LoadSyncableFetchedSkills returns the catalog after applying the same
// versioned-content enrichment used by sync, so prompts and summaries do not
// advertise placeholder skills that will later be dropped.
func LoadSyncableFetchedSkills(ctx context.Context, client *api.Client, fetched *FetchedSkills) (*FetchedSkills, error) {
	if fetched == nil {
		return &FetchedSkills{}, nil
	}
	allSkills := make([]Skill, 0, len(fetched.Required)+len(fetched.Optional))
	allSkills = append(allSkills, fetched.Required...)
	allSkills = append(allSkills, fetched.Optional...)

	syncableSkills, err := LoadLatestVersionFiles(ctx, client, allSkills)
	if err != nil {
		return nil, err
	}

	usedGroupIDs := make(map[string]bool)
	result := &FetchedSkills{}
	for _, skill := range syncableSkills {
		for _, groupID := range skill.GroupIDs {
			usedGroupIDs[groupID] = true
		}
		if skill.Required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}

	for _, group := range fetched.Groups {
		if usedGroupIDs[group.Identifier] {
			result.Groups = append(result.Groups, group)
		}
	}

	return result, nil
}

func skillIdentifiers(skills []Skill) []string {
	ids := make([]string, 0, len(skills))
	for _, skill := range skills {
		if skill.Identifier != "" {
			ids = append(ids, skill.Identifier)
		}
	}
	return ids
}

func hasSyncableContent(skill Skill) bool {
	return skill.Instructions != "" ||
		len(skill.Files) > 0 ||
		len(skill.References) > 0 ||
		len(skill.Assets) > 0 ||
		len(skill.Scripts) > 0 ||
		len(skill.AdditionalFiles) > 0
}

// ParseFetchedSkills builds a FetchedSkills from raw API entities.
// Exported so tests can exercise parsing without hitting the network.
func ParseFetchedSkills(groupEntities, skillEntities []api.Entity) *FetchedSkills {
	return ParseFetchedSkillEntities(groupEntities, skillEntities, nil, nil)
}

// ParseFetchedSkillEntities builds a FetchedSkills value from the versioned Port
// model: skill_file => skill_version => skill => skill_group. The version
// entities are expected to be sorted latest-first by the API call; the first
// version seen for each skill is the one synced to disk.
func ParseFetchedSkillEntities(groupEntities, skillEntities, versionEntities, fileEntities []api.Entity) *FetchedSkills {
	groups := make([]SkillGroup, 0, len(groupEntities))
	requiredSkillIDs := make(map[string]bool)
	autoSyncSkillIDs := make(map[string]bool)
	skillGroupMap := make(map[string][]string)
	groupsByID := make(map[string]SkillGroup)

	for _, e := range groupEntities {
		props, _ := e["properties"].(map[string]interface{})
		relations, _ := e["relations"].(map[string]interface{})

		groupID := stringProp(e, "identifier")
		enforcement := stringFromMap(props, "enforcement")
		isRequired := enforcement == "required"
		autoSync := boolFromMapDefault(props, "auto_sync", false)

		var skillIDs []string
		if rel, ok := relations["skills"]; ok {
			for _, sid := range relationIDs(rel) {
				skillIDs = append(skillIDs, sid)
				skillGroupMap[sid] = appendUniqueString(skillGroupMap[sid], groupID)
				if isRequired {
					requiredSkillIDs[sid] = true
				}
				if autoSync {
					autoSyncSkillIDs[sid] = true
				}
			}
		}

		group := SkillGroup{
			Identifier: groupID,
			Title:      stringProp(e, "title"),
			Required:   isRequired,
			AutoSync:   autoSync,
			SkillIDs:   skillIDs,
		}
		groups = append(groups, group)
		groupsByID[groupID] = group
	}

	for _, e := range skillEntities {
		skillID := stringProp(e, "identifier")
		relations, _ := e["relations"].(map[string]interface{})
		for _, groupID := range relationIDs(relations["skill_to_skill_group"]) {
			skillGroupMap[skillID] = appendUniqueString(skillGroupMap[skillID], groupID)
			if group, ok := groupsByID[groupID]; ok {
				if group.Required {
					requiredSkillIDs[skillID] = true
				}
				if group.AutoSync {
					autoSyncSkillIDs[skillID] = true
				}
				groupsByID[groupID] = groupWithSkill(group, skillID)
			}
		}
	}

	for i, group := range groups {
		if updated, ok := groupsByID[group.Identifier]; ok {
			groups[i] = updated
		}
	}

	latestVersionBySkill := latestVersionsBySkill(versionEntities)
	filesByVersion := filesByVersion(fileEntities)

	result := &FetchedSkills{Groups: groups}
	for _, e := range skillEntities {
		props, _ := e["properties"].(map[string]interface{})
		skillID := stringProp(e, "identifier")
		latestVersion := latestVersionBySkill[skillID]
		versionProps, _ := latestVersion["properties"].(map[string]interface{})
		versionID := stringProp(latestVersion, "identifier")

		skill := Skill{
			Identifier:      skillID,
			Title:           stringProp(e, "title"),
			Description:     firstNonEmpty(stringFromMap(versionProps, "description"), stringFromMap(props, "description")),
			Instructions:    stringFromMap(props, "instructions"),
			GroupIDs:        skillGroupMap[skillID],
			Required:        requiredSkillIDs[skillID],
			AutoSync:        autoSyncSkillIDs[skillID],
			Location:        parseSkillLocation(stringFromMap(props, "location")),
			Versioned:       versionID != "",
			Files:           filesByVersion[versionID],
			References:      parseSkillFiles(props, "references"),
			Assets:          parseSkillFiles(props, "assets"),
			Scripts:         parseSkillFiles(props, "scripts"),
			AdditionalFiles: parseSkillFiles(props, "additional_files"),
		}

		if skill.Required {
			result.Required = append(result.Required, skill)
		} else {
			result.Optional = append(result.Optional, skill)
		}
	}

	return result
}

func parseSkillLocation(raw string) SkillLocation {
	if raw == string(SkillLocationProject) {
		return SkillLocationProject
	}
	return SkillLocationGlobal
}

func parseSkillFiles(props map[string]interface{}, key string) []SkillFile {
	if props == nil {
		return nil
	}
	raw, ok := props[key].([]interface{})
	if !ok {
		return nil
	}
	var files []SkillFile
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		path := stringFromMap(m, "path")
		content := stringFromMap(m, "content")
		if path != "" && content != "" {
			files = append(files, SkillFile{Path: path, Content: content})
		}
	}
	return files
}

func latestVersionsBySkill(versionEntities []api.Entity) map[string]api.Entity {
	versionEntities = sortedVersionsDesc(versionEntities)
	latest := make(map[string]api.Entity)
	for _, e := range versionEntities {
		relations, _ := e["relations"].(map[string]interface{})
		skillIDs := relationIDs(relations["skill_version_to_skill"])
		if len(skillIDs) == 0 {
			continue
		}
		skillID := skillIDs[0]
		if _, exists := latest[skillID]; !exists {
			latest[skillID] = e
		}
	}
	return latest
}

func sortedVersionsDesc(versionEntities []api.Entity) []api.Entity {
	sorted := append([]api.Entity(nil), versionEntities...)
	sort.SliceStable(sorted, func(i, j int) bool {
		iprops, _ := sorted[i]["properties"].(map[string]interface{})
		jprops, _ := sorted[j]["properties"].(map[string]interface{})
		return compareVersionStrings(stringFromMap(iprops, "version"), stringFromMap(jprops, "version")) > 0
	})
	return sorted
}

func compareVersionStrings(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	aParts := versionParts(a)
	bParts := versionParts(b)
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		aExists := i < len(aParts)
		bExists := i < len(bParts)
		aPart, bPart := "0", "0"
		if aExists {
			aPart = aParts[i]
		}
		if bExists {
			bPart = bParts[i]
		}
		aNum, aErr := strconv.Atoi(aPart)
		bNum, bErr := strconv.Atoi(bPart)
		if aErr == nil && bErr == nil {
			if aNum != bNum {
				if aNum > bNum {
					return 1
				}
				return -1
			}
			continue
		}
		// One or both segments are non-numeric (pre-release identifiers such as
		// "alpha", "beta", "rc1"). Per semver, a version that has a pre-release
		// segment at a position where the other version has no segment at all is
		// the lower-precedence version (e.g. 1.2.3-alpha < 1.2.3).
		if aErr != nil && !bExists {
			return -1
		}
		if bErr != nil && !aExists {
			return 1
		}
		if aPart != bPart {
			if aPart > bPart {
				return 1
			}
			return -1
		}
	}
	return 0
}

func versionParts(version string) []string {
	return strings.FieldsFunc(version, func(r rune) bool {
		//nolint:staticcheck // QF1001: De Morgan's law simplification here reduces unicode clarity
		return !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	})
}

func filesByVersion(fileEntities []api.Entity) map[string][]SkillFile {
	files := make(map[string][]SkillFile)
	for _, e := range fileEntities {
		file, ok := skillFileFromEntity(e)
		if !ok {
			continue
		}
		relations, _ := e["relations"].(map[string]interface{})
		versionIDs := relationIDs(relations["skill_file_to_skill_version"])
		for _, versionID := range versionIDs {
			files[versionID] = append(files[versionID], file)
		}
	}
	return files
}

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

func skillFileFromEntity(e api.Entity) (SkillFile, bool) {
	props, _ := e["properties"].(map[string]interface{})
	path := stringFromMap(props, "path")
	content, ok := props["content"].(string)
	if path == "" || !ok {
		return SkillFile{}, false
	}
	return SkillFile{Path: path, Content: content}, true
}

func relationIDs(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case map[string]interface{}:
		if id := stringFromMap(v, "identifier"); id != "" {
			return []string{id}
		}
	case []interface{}:
		var ids []string
		for _, item := range v {
			ids = append(ids, relationIDs(item)...)
		}
		return ids
	}
	return nil
}

func boolFromMapDefault(m map[string]interface{}, key string, defaultValue bool) bool {
	if m == nil {
		return defaultValue
	}
	v, ok := m[key]
	if !ok || v == nil {
		return defaultValue
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultValue
}

func groupWithSkill(group SkillGroup, skillID string) SkillGroup {
	group.SkillIDs = appendUniqueString(group.SkillIDs, skillID)
	return group
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// FilterSkills returns the union of all required skills plus the optional
// skills matching the provided selection criteria.
func FilterSkills(fetched *FetchedSkills, selectAll, selectAllGroups, selectAllUngrouped bool, selectedGroups, selectedSkills []string) []Skill {
	var result []Skill
	result = append(result, fetched.Required...)

	if selectAll {
		result = append(result, fetched.Optional...)
		return result
	}

	selectedGroupSet := toSet(selectedGroups)
	selectedSkillSet := toSet(selectedSkills)

	for _, s := range fetched.Optional {
		ungrouped := len(s.GroupIDs) == 0
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

const (
	NoGroupDir    = "_skills_without_group"
	PortSkillsDir = "port"
)

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

	if err := writeSkillsToTargets(globalSkills, groups, globalTargets); err != nil {
		return err
	}

	if len(projectDirs) > 0 && len(projectSkills) > 0 {
		projectTargets := buildProjectTargets(globalTargets, projectDirs)
		if err := writeSkillsToTargets(projectSkills, groups, projectTargets); err != nil {
			return err
		}
	}

	return nil
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

func writeSkillsToTargets(skills []Skill, groups []SkillGroup, targets []string) error {
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

	for _, target := range targets {
		expanded := expandHome(target)
		portDir := filepath.Join(expanded, "skills", PortSkillsDir)

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
			return fmt.Errorf("reconciliation failed for %s: %w", target, err)
		}
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
	for _, f := range filterOrphanSkillFiles(s, allSkillFiles(s)) {
		relPath, err := normalizeSkillFilePath(f.Path, skillDirName, s)
		if err != nil {
			return fmt.Errorf("failed to write file %s for skill %s: %w", f.Path, s.Identifier, err)
		}
		if relPath == "SKILL.md" {
			hasSkillMD = true
		}
		if err := writeSkillFile(skillDir, SkillFile{Path: relPath, Content: f.Content}); err != nil {
			return fmt.Errorf("failed to write file %s for skill %s: %w", f.Path, s.Identifier, err)
		}
	}

	if !hasSkillMD {
		content := buildSkillMD(s)
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write SKILL.md for %s: %w", s.Identifier, err)
		}
	}

	return nil
}

func allSkillFiles(s Skill) []SkillFile {
	files := make([]SkillFile, 0, len(s.Files)+len(s.References)+len(s.Assets)+len(s.Scripts)+len(s.AdditionalFiles))
	files = append(files, s.Files...)
	files = append(files, s.References...)
	files = append(files, s.Assets...)
	files = append(files, s.Scripts...)
	files = append(files, s.AdditionalFiles...)
	return files
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
	if s.Versioned {
		if validatePathComponent(s.Title) == nil {
			return s.Title, nil
		}
		return "", fmt.Errorf("invalid skill title %q: %w", s.Title, validatePathComponent(s.Title))
	}
	if validatePathComponent(s.Identifier) == nil {
		return s.Identifier, nil
	}
	return "", fmt.Errorf("invalid skill identifier %q: %w", s.Identifier, validatePathComponent(s.Identifier))
}

func validatePathComponent(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("contains invalid path characters")
	}
	return nil
}

func buildSkillMD(s Skill) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %s\n", s.Identifier)
	if s.Description != "" {
		fmt.Fprintf(&sb, "description: %s\n", s.Description)
	}
	sb.WriteString("---\n\n")

	if s.Instructions != "" {
		sb.WriteString(s.Instructions)
		if !strings.HasSuffix(s.Instructions, "\n") {
			sb.WriteString("\n")
		}
	} else {
		fmt.Fprintf(&sb, "# %s\n\n_No instructions provided._\n", s.Title)
	}

	return sb.String()
}

func stringProp(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func stringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

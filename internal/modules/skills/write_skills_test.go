package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSkills_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{Identifier: "my-skill", Title: "My Skill", Description: "does stuff", Instructions: "step 1\nstep 2\n", GroupIDs: []string{"my-group"}},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	content, err := os.ReadFile(skillMDPath(dir, "my-group", "my-skill"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	body := string(content)
	for _, want := range []string{"name: my-skill", "description: does stuff", "step 1"} {
		if !containsStr(body, want) {
			t.Errorf("SKILL.md missing %q", want)
		}
	}
}

func TestWriteSkills_UngroupedUsesNoGroupDir(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSkills([]Skill{{Identifier: "solo-skill", Title: "Solo"}}, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "", "solo-skill"))
}

func TestWriteSkills_WritesReferencesAndAssets(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier:   "skill-files",
			GroupIDs:     []string{"grp"},
			Instructions: "do it",
			References:   []SkillFile{{Path: "references/guide.md", Content: "# Guide"}},
			Assets:       []SkillFile{{Path: "assets/config.yaml", Content: "key: value"}},
		},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files", "references", "guide.md"))
	assertFileExists(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files", "assets", "config.yaml"))
}

func TestWriteSkills_WritesScriptsAndAdditionalFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier:      "skill-more-files",
			GroupIDs:        []string{"grp"},
			Instructions:    "run it",
			Scripts:         []SkillFile{{Path: "scripts/extract.py", Content: "print(1)\n"}},
			AdditionalFiles: []SkillFile{{Path: "NOTICE", Content: "legal"}},
		},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-more-files", "scripts", "extract.py"))
	assertFileExists(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-more-files", "NOTICE"))
}

func TestWriteSkills_MultipleTargets(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	skills := []Skill{{Identifier: "sk", GroupIDs: []string{"g"}, Instructions: "x"}}
	if err := WriteSkills(skills, nil, []string{dir1, dir2}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir1, "g", "sk"))
	assertFileExists(t, skillMDPath(dir2, "g", "sk"))
}

func TestWriteSkills_ReconcileRemovesStaleSkillAndEmptyGroup(t *testing.T) {
	dir := t.TempDir()
	initial := []Skill{
		{Identifier: "keep", GroupIDs: []string{"grp"}, Instructions: "x"},
		{Identifier: "stale", GroupIDs: []string{"grp"}, Instructions: "y"},
		{Identifier: "sk", GroupIDs: []string{"gone-group"}, Instructions: "z"},
	}
	if err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills: %v", err)
	}

	updated := []Skill{{Identifier: "keep", GroupIDs: []string{"grp"}, Instructions: "x"}}
	if err := WriteSkills(updated, nil, []string{dir}, nil); err != nil {
		t.Fatalf("second WriteSkills: %v", err)
	}

	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "stale"))
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "gone-group"))
	assertFileExists(t, skillMDPath(dir, "grp", "keep"))
}

func TestWriteSkills_MultiGroupSkillWrittenToAllGroups(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{Identifier: "shared-skill", GroupIDs: []string{"group-a", "group-b"}, Instructions: "x"},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "group-a", "shared-skill"))
	assertFileExists(t, skillMDPath(dir, "group-b", "shared-skill"))
}

func TestWriteSkills_WritesVersionedFilesUsingSafeNames(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "org/platform/deploy-helper",
			Title:      "deploy-helper",
			GroupIDs:   []string{"org/platform"},
			Files: []SkillFile{
				{Path: ".cursor/skills/port/deploy-helper/SKILL.md", Content: "versioned skill"},
				{Path: ".cursor/skills/port/deploy-helper/references/runbook.md", Content: "# Runbook"},
			},
		},
	}
	groups := []SkillGroup{{Identifier: "org/platform", Title: "platform"}}

	if err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "SKILL.md"), "versioned skill")
	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "references", "runbook.md"), "# Runbook")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", ".cursor"))
}

func TestWriteSkills_StripsFullSlashIdentifierFromVersionedPaths(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "org/platform/deploy-helper",
			Title:      "deploy-helper",
			GroupIDs:   []string{"org/platform"},
			Files: []SkillFile{
				{Path: ".cursor/skills/port/org/platform/deploy-helper/SKILL.md", Content: "full identifier path"},
			},
		},
	}
	groups := []SkillGroup{{Identifier: "org/platform", Title: "platform"}}

	if err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "SKILL.md"), "full identifier path")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "org"))
}

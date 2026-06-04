package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func skillWithMD(id, title, groupID, body string) Skill {
	s := Skill{
		Identifier: id,
		Title:      title,
		GroupIDs:   []string{groupID},
		Files:      []SkillFile{{Path: "SKILL.md", Content: body}},
	}
	if groupID == "" {
		s.GroupIDs = nil
	}
	return s
}

func TestWriteSkills_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		skillWithMD("my-skill", "my-skill", "my-group", "---\nname: my-skill\ndescription: does stuff\n---\n\nstep 1\nstep 2\n"),
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
	if err := WriteSkills([]Skill{skillWithMD("solo-skill", "solo-skill", "", "# Solo")}, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "", "solo-skill"))
}

func TestWriteSkills_WritesBundledFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "skill-files",
			Title:      "skill-files",
			GroupIDs:   []string{"grp"},
			Files: []SkillFile{
				{Path: "SKILL.md", Content: "# Skill"},
				{Path: "references/guide.md", Content: "# Guide"},
				{Path: "assets/config.yaml", Content: "key: value"},
				{Path: "scripts/run.sh", Content: "#!/bin/sh\n"},
				{Path: "NOTICE", Content: "MIT"},
			},
		},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	base := filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files")
	assertFileExists(t, filepath.Join(base, "references", "guide.md"))
	assertFileExists(t, filepath.Join(base, "assets", "config.yaml"))
	assertFileExists(t, filepath.Join(base, "scripts", "run.sh"))
	assertFileExists(t, filepath.Join(base, "NOTICE"))
}

func TestWriteSkills_MultipleTargets(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	skills := []Skill{skillWithMD("sk", "sk", "g", "# x")}
	if err := WriteSkills(skills, nil, []string{dir1, dir2}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir1, "g", "sk"))
	assertFileExists(t, skillMDPath(dir2, "g", "sk"))
}

func TestWriteSkills_ReconcileRemovesStaleSkillAndEmptyGroup(t *testing.T) {
	dir := t.TempDir()
	initial := []Skill{
		skillWithMD("keep", "keep", "grp", "# keep"),
		skillWithMD("stale", "stale", "grp", "# stale"),
		skillWithMD("sk", "sk", "gone-group", "# z"),
	}
	if err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills: %v", err)
	}

	updated := []Skill{skillWithMD("keep", "keep", "grp", "# keep")}
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
		{
			Identifier: "shared-skill",
			Title:      "shared-skill",
			GroupIDs:   []string{"group-a", "group-b"},
			Files:      []SkillFile{{Path: "SKILL.md", Content: "# x"}},
		},
	}
	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "group-a", "shared-skill"))
	assertFileExists(t, skillMDPath(dir, "group-b", "shared-skill"))
}

func TestWriteSkills_WritesFilesUnderSkillTitle(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "org/platform/deploy-helper",
			Title:      "Deploy Helper",
			GroupIDs:   []string{"org/platform"},
			Files: []SkillFile{
				{Path: "SKILL.md", Content: "versioned skill"},
				{Path: "references/runbook.md", Content: "# Runbook"},
			},
		},
	}
	groups := []SkillGroup{{Identifier: "org/platform", Title: "platform"}}

	if err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "Deploy Helper", "SKILL.md"), "versioned skill")
	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "Deploy Helper", "references", "runbook.md"), "# Runbook")
}

func TestWriteSkills_NormalizesSourceStylePathsUsingSkillTitle(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "org/platform/deploy-helper",
			Title:      "deploy-helper",
			GroupIDs:   []string{"org/platform"},
			Files: []SkillFile{
				{Path: ".cursor/skills/engineering/deploy-helper/SKILL.md", Content: "source style path"},
			},
		},
	}
	groups := []SkillGroup{{Identifier: "org/platform", Title: "platform"}}

	if err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "SKILL.md"), "source style path")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "engineering"))
}

func TestWriteSkills_NormalizesSourceStylePathsUsingIdentifierBase(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "org/platform/deploy-helper",
			Title:      "Deploy Helper",
			GroupIDs:   []string{"org/platform"},
			Files: []SkillFile{
				{Path: ".cursor/skills/engineering/deploy-helper/SKILL.md", Content: "source style path"},
			},
		},
	}
	groups := []SkillGroup{{Identifier: "org/platform", Title: "platform"}}

	if err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "Deploy Helper", "SKILL.md"), "source style path")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "Deploy Helper", "engineering"))
}

func TestWriteSkills_IgnoresSourceStyleOrphanFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "deploy-helper",
			Title:      "deploy-helper",
			GroupIDs:   []string{"platform"},
			Files: []SkillFile{
				{Path: ".cursor/skills/engineering/orphan-file", Content: "ignored"},
				{Path: "SKILL.md", Content: "kept"},
			},
		},
	}

	if err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileContent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "SKILL.md"), "kept")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "orphan-file"))
}

func TestWriteSkills_GlobalAndProjectSamePortDirPreservesBoth(t *testing.T) {
	workdir := t.TempDir()
	cursorTarget := filepath.Join(workdir, ".cursor")
	global := skillWithMD("global-skill", "global-skill", "grp-a", "name: global-skill\n---\n# Global")
	global.Location = SkillLocationGlobal
	project := skillWithMD("project-skill", "project-skill", "grp-b", "name: project-skill\n---\n# Project")
	project.Location = SkillLocationProject

	if err := WriteSkills(
		[]Skill{global, project},
		[]SkillGroup{{Identifier: "grp-a"}, {Identifier: "grp-b"}},
		[]string{cursorTarget},
		[]string{workdir},
	); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileExists(t, skillMDPath(cursorTarget, "grp-a", "global-skill"))
	assertFileExists(t, skillMDPath(cursorTarget, "grp-b", "project-skill"))
}

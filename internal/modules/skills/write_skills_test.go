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
	if _, err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	content, err := os.ReadFile(skillMDPath(dir, "my-group", "my-skill", "my-skill"))
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

func TestWriteSkills_NormalizesSkillDirectoryAndFrontmatterNameFromIdentifier(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier:  "deploy_helper",
			Title:       "Deploy Helper",
			Description: "Deploys services safely.",
			GroupIDs:    []string{"platform"},
			Files: []SkillFile{
				{Path: "SKILL.md", Content: "---\ndescription: Deploys services safely.\n---\nRun the deploy steps."},
			},
		},
	}

	if _, err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	// identDir = "deploy-helper" (normalized from "deploy_helper")
	// titleDir = "deploy-helper" (normalized from "Deploy Helper")
	content, err := os.ReadFile(skillMDPath(dir, "platform", "deploy-helper", "deploy-helper"))
	if err != nil {
		t.Fatalf("read normalized SKILL.md: %v", err)
	}
	body := string(content)
	for _, want := range []string{"name: deploy-helper", "description: Deploys services safely.", "Run the deploy steps."} {
		if !containsStr(body, want) {
			t.Errorf("SKILL.md missing %q", want)
		}
	}
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "Deploy Helper"))
}

func TestWriteSkills_UngroupedUsesNoGroupDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteSkills([]Skill{skillWithMD("solo-skill", "solo-skill", "", "# Solo")}, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "", "solo-skill", "solo-skill"))
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
	if _, err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	// identDir = "skill-files", titleDir = "skill-files"
	base := filepath.Join(dir, "skills", PortSkillsDir, "grp", "skill-files", "skill-files")
	assertFileExists(t, filepath.Join(base, "references", "guide.md"))
	assertFileExists(t, filepath.Join(base, "assets", "config.yaml"))
	assertFileExists(t, filepath.Join(base, "scripts", "run.sh"))
	assertFileExists(t, filepath.Join(base, "NOTICE"))
}

func TestWriteSkills_DefaultSyncTargetsWritesAgentsAndClaude(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	targets := TargetPaths(DefaultSyncTargets(), home, repo)
	skills := []Skill{skillWithMD("sk", "sk", "g", "# x")}
	if _, err := WriteSkills(skills, nil, targets, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	for _, target := range targets {
		assertFileExists(t, skillMDPath(target, "g", "sk", "sk"))
	}
}

func TestWriteSkills_MultipleTargets(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	skills := []Skill{skillWithMD("sk", "sk", "g", "# x")}
	if _, err := WriteSkills(skills, nil, []string{dir1, dir2}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir1, "g", "sk", "sk"))
	assertFileExists(t, skillMDPath(dir2, "g", "sk", "sk"))
}

func TestWriteSkills_ReconcileRemovesStaleSkillAndEmptyGroup(t *testing.T) {
	dir := t.TempDir()
	initial := []Skill{
		skillWithMD("keep", "keep", "grp", "# keep"),
		skillWithMD("stale", "stale", "grp", "# stale"),
		skillWithMD("sk", "sk", "gone-group", "# z"),
	}
	if _, err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills: %v", err)
	}

	updated := []Skill{skillWithMD("keep", "keep", "grp", "# keep")}
	if _, err := WriteSkills(updated, nil, []string{dir}, nil); err != nil {
		t.Fatalf("second WriteSkills: %v", err)
	}

	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "stale"))
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "gone-group"))
	assertFileExists(t, skillMDPath(dir, "grp", "keep", "keep"))
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
	if _, err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir, "group-a", "shared-skill", "shared-skill"))
	assertFileExists(t, skillMDPath(dir, "group-b", "shared-skill", "shared-skill"))
}

func TestWriteSkills_WritesFilesUnderSpecSafeSkillName(t *testing.T) {
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

	if _, err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	// identDir = "deploy-helper" (base of "org/platform/deploy-helper")
	// titleDir = "deploy-helper" (normalized from "Deploy Helper")
	base := filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "deploy-helper")
	assertFileContent(t, filepath.Join(base, "SKILL.md"), "---\nname: deploy-helper\ndescription: Port skill deploy-helper.\n---\n\nversioned skill")
	assertFileContent(t, filepath.Join(base, "references", "runbook.md"), "# Runbook")
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

	if _, err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	// identDir = "deploy-helper" (base of "org/platform/deploy-helper"), titleDir = "deploy-helper"
	base := filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "deploy-helper")
	assertFileContent(t, filepath.Join(base, "SKILL.md"), "---\nname: deploy-helper\ndescription: Port skill deploy-helper.\n---\n\nsource style path")
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

	if _, err := WriteSkills(skills, groups, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	// identDir = "deploy-helper" (base of "org/platform/deploy-helper"), titleDir = "deploy-helper"
	base := filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "deploy-helper")
	assertFileContent(t, filepath.Join(base, "SKILL.md"), "---\nname: deploy-helper\ndescription: Port skill deploy-helper.\n---\n\nsource style path")
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "engineering"))
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

	if _, err := WriteSkills(skills, nil, []string{dir}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	// identDir = "deploy-helper", titleDir = "deploy-helper"
	base := filepath.Join(dir, "skills", PortSkillsDir, "platform", "deploy-helper", "deploy-helper")
	assertFileContent(t, filepath.Join(base, "SKILL.md"), "---\nname: deploy-helper\ndescription: Port skill deploy-helper.\n---\n\nkept")
	assertFileAbsent(t, filepath.Join(base, "orphan-file"))
}

func TestWriteSkills_GlobalAndProjectSamePortDirPreservesBoth(t *testing.T) {
	workdir := t.TempDir()
	cursorTarget := filepath.Join(workdir, ".cursor")
	global := skillWithMD("global-skill", "global-skill", "grp-a", "name: global-skill\n---\n# Global")
	global.Location = SkillLocationGlobal
	project := skillWithMD("project-skill", "project-skill", "grp-b", "name: project-skill\n---\n# Project")
	project.Location = SkillLocationProject

	if _, err := WriteSkills(
		[]Skill{global, project},
		[]SkillGroup{{Identifier: "grp-a"}, {Identifier: "grp-b"}},
		[]string{cursorTarget},
		[]string{workdir},
	); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}

	assertFileExists(t, skillMDPath(cursorTarget, "grp-a", "global-skill", "global-skill"))
	assertFileExists(t, skillMDPath(cursorTarget, "grp-b", "project-skill", "project-skill"))
}

func TestWriteSkills_SkipsSkillWithNoSkillMD(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		// valid skill — should be written
		skillWithMD("good-skill", "good-skill", "grp", "# Good"),
		// skill with no SKILL.md — should be skipped with a warning
		{
			Identifier: "no-skill-md",
			Title:      "no-skill-md",
			GroupIDs:   []string{"grp"},
			Files:      []SkillFile{{Path: "scripts/run.sh", Content: "#!/bin/sh\n"}},
		},
	}

	warnings, err := WriteSkills(skills, nil, []string{dir}, nil)
	if err != nil {
		t.Fatalf("WriteSkills returned unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !containsStr(warnings[0], "no-skill-md") {
		t.Errorf("warning should mention skipped skill identifier, got: %s", warnings[0])
	}

	// Good skill written as normal.
	assertFileExists(t, skillMDPath(dir, "grp", "good-skill", "good-skill"))
	// Skipped skill's identifier dir should not exist on disk.
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "no-skill-md"))
}

func TestWriteSkills_SkipDoesNotAbortOtherSkills(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{
			Identifier: "missing-md",
			Title:      "missing-md",
			GroupIDs:   []string{"grp"},
			Files:      []SkillFile{{Path: "references/guide.md", Content: "# Guide"}},
		},
		skillWithMD("after-skip", "after-skip", "grp", "# After"),
	}

	warnings, err := WriteSkills(skills, nil, []string{dir}, nil)
	if err != nil {
		t.Fatalf("WriteSkills should succeed even when a skill has no SKILL.md: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected at least one warning for the skipped skill")
	}
	assertFileExists(t, skillMDPath(dir, "grp", "after-skip", "after-skip"))
}

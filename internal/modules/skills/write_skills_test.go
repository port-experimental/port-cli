package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSkills_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	skills := []Skill{
		{Identifier: "my-skill", Title: "My Skill", Description: "does stuff", Instructions: "step 1\nstep 2\n", GroupID: "my-group"},
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
			GroupID:      "grp",
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

func TestWriteSkills_MultipleTargets(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	skills := []Skill{{Identifier: "sk", GroupID: "g", Instructions: "x"}}
	if err := WriteSkills(skills, nil, []string{dir1, dir2}, nil); err != nil {
		t.Fatalf("WriteSkills: %v", err)
	}
	assertFileExists(t, skillMDPath(dir1, "g", "sk"))
	assertFileExists(t, skillMDPath(dir2, "g", "sk"))
}

func TestWriteSkills_ReconcileRemovesStaleSkillAndEmptyGroup(t *testing.T) {
	dir := t.TempDir()
	initial := []Skill{
		{Identifier: "keep", GroupID: "grp", Instructions: "x"},
		{Identifier: "stale", GroupID: "grp", Instructions: "y"},
		{Identifier: "sk", GroupID: "gone-group", Instructions: "z"},
	}
	if err := WriteSkills(initial, nil, []string{dir}, nil); err != nil {
		t.Fatalf("initial WriteSkills: %v", err)
	}

	updated := []Skill{{Identifier: "keep", GroupID: "grp", Instructions: "x"}}
	if err := WriteSkills(updated, nil, []string{dir}, nil); err != nil {
		t.Fatalf("second WriteSkills: %v", err)
	}

	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "grp", "stale"))
	assertFileAbsent(t, filepath.Join(dir, "skills", PortSkillsDir, "gone-group"))
	assertFileExists(t, skillMDPath(dir, "grp", "keep"))
}

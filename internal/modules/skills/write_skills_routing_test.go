package skills

import (
	"path/filepath"
	"testing"
)

func TestWriteSkills_LocationRouting(t *testing.T) {
	tests := []struct {
		name          string
		location      SkillLocation
		hasProjectDir bool
		wantInGlobal  bool
		wantInProject bool
	}{
		{
			name:          "global skill goes to global only",
			location:      SkillLocationGlobal,
			hasProjectDir: true,
			wantInGlobal:  true,
			wantInProject: false,
		},
		{
			name:          "project skill goes to project only",
			location:      SkillLocationProject,
			hasProjectDir: true,
			wantInGlobal:  false,
			wantInProject: true,
		},
		{
			name:          "default (empty) location is global",
			location:      "",
			hasProjectDir: true,
			wantInGlobal:  true,
			wantInProject: false,
		},
		{
			name:          "project skill skipped when no projectDirs",
			location:      SkillLocationProject,
			hasProjectDir: false,
			wantInGlobal:  false,
			wantInProject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			globalTarget := filepath.Join(homeDir, ".cursor")

			var projectDirs []string
			var projectDir string
			if tt.hasProjectDir {
				projectDir = t.TempDir()
				projectDirs = []string{projectDir}
			}

			skills := []Skill{{Identifier: "skill", GroupID: "grp", Instructions: "x", Location: tt.location}}
			if err := WriteSkills(skills, nil, []string{globalTarget}, projectDirs); err != nil {
				t.Fatalf("WriteSkills: %v", err)
			}

			globalPath := skillMDPath(globalTarget, "grp", "skill")
			if tt.wantInGlobal {
				assertFileExists(t, globalPath)
			} else {
				assertFileAbsent(t, globalPath)
			}

			if projectDir != "" {
				projectPath := skillMDPath(filepath.Join(projectDir, ".cursor"), "grp", "skill")
				if tt.wantInProject {
					assertFileExists(t, projectPath)
				} else {
					assertFileAbsent(t, projectPath)
				}
			}
		})
	}
}

func TestWriteSkills_PathTraversalPrevention(t *testing.T) {
	tests := []struct {
		name        string
		skill       Skill
		wantErrFrag string
	}{
		{
			name:        "traversal in identifier",
			skill:       Skill{Identifier: "../../../etc", GroupID: "grp", Instructions: "x"},
			wantErrFrag: "invalid skill identifier",
		},
		{
			name:        "traversal in group ID",
			skill:       Skill{Identifier: "ok-skill", GroupID: "../../etc", Instructions: "x"},
			wantErrFrag: "invalid group ID",
		},
		{
			name:        "traversal in asset file path",
			skill:       Skill{Identifier: "sk", GroupID: "grp", Instructions: "x", Assets: []SkillFile{{Path: "../../../../tmp/evil", Content: "pwned"}}},
			wantErrFrag: "escapes skill directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := WriteSkills([]Skill{tt.skill}, nil, []string{dir}, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErrFrag) {
				t.Errorf("want error containing %q, got: %v", tt.wantErrFrag, err)
			}
		})
	}
}

func TestGitHubCopilot_SkillRouting(t *testing.T) {
	homeDir := t.TempDir()
	copilotTarget := filepath.Join(homeDir, ".copilot")
	cursorTarget := filepath.Join(homeDir, ".cursor")

	t.Run("global skills go to ~/.copilot", func(t *testing.T) {
		skills := []Skill{{Identifier: "global-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationGlobal}}
		if err := WriteSkills(skills, nil, []string{copilotTarget}, nil); err != nil {
			t.Fatalf("WriteSkills: %v", err)
		}
		assertFileExists(t, skillMDPath(copilotTarget, "grp", "global-skill"))
	})

	t.Run("project skills go to repo/.github", func(t *testing.T) {
		repoDir := t.TempDir()
		skills := []Skill{{Identifier: "proj-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationProject}}
		if err := WriteSkills(skills, nil, []string{copilotTarget}, []string{repoDir}); err != nil {
			t.Fatalf("WriteSkills: %v", err)
		}
		assertFileExists(t, skillMDPath(filepath.Join(repoDir, ".github"), "grp", "proj-skill"))
		assertFileAbsent(t, skillMDPath(copilotTarget, "grp", "proj-skill"))
	})

	t.Run("multiple tools write to correct project dirs", func(t *testing.T) {
		repoDir := t.TempDir()
		skills := []Skill{{Identifier: "multi-skill", GroupID: "grp", Instructions: "x", Location: SkillLocationProject}}
		if err := WriteSkills(skills, nil, []string{cursorTarget, copilotTarget}, []string{repoDir}); err != nil {
			t.Fatalf("WriteSkills: %v", err)
		}
		assertFileExists(t, skillMDPath(filepath.Join(repoDir, ".cursor"), "grp", "multi-skill"))
		assertFileExists(t, skillMDPath(filepath.Join(repoDir, ".github"), "grp", "multi-skill"))
	})
}

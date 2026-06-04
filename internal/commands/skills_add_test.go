package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSkillsAdd_CommandRegistered(t *testing.T) {
	root := &cobra.Command{Use: "port"}
	RegisterSkills(root)

	addCmd, _, err := root.Find([]string{"skills", "add"})
	if err != nil || addCmd == nil {
		t.Fatal("skills add command not found")
	}
}

func TestSkillsAdd_FlagsRegistered(t *testing.T) {
	root := &cobra.Command{Use: "port"}
	RegisterSkills(root)

	addCmd, _, err := root.Find([]string{"skills", "add"})
	if err != nil || addCmd == nil {
		t.Fatal("skills add command not found")
	}

	if err := addCmd.ParseFlags([]string{"--group", "my-group", "--skill", "my-skill", "--tool", "Cursor"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	groups, _ := addCmd.Flags().GetStringArray("group")
	if len(groups) != 1 || groups[0] != "my-group" {
		t.Errorf("group flag: got %v", groups)
	}
	skills, _ := addCmd.Flags().GetStringArray("skill")
	if len(skills) != 1 || skills[0] != "my-skill" {
		t.Errorf("skill flag: got %v", skills)
	}
	tools, _ := addCmd.Flags().GetStringArray("tool")
	if len(tools) != 1 || tools[0] != "Cursor" {
		t.Errorf("tool flag: got %v", tools)
	}
}

func TestResolveTargetsByName(t *testing.T) {
	targets, err := resolveTargetsByName([]string{"Cursor", "Claude Code"})
	if err != nil {
		t.Fatalf("resolveTargetsByName: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
}

func TestResolveTargetsByName_Agents(t *testing.T) {
	targets, err := resolveTargetsByName([]string{"Agents (cross-platform)"})
	if err != nil {
		t.Fatalf("resolveTargetsByName: %v", err)
	}
	if len(targets) != 1 || targets[0].Dir != ".agents" || !targets[0].SkillsOnly {
		t.Fatalf("got %+v", targets)
	}
}

func TestResolveTargetsByName_Unknown(t *testing.T) {
	_, err := resolveTargetsByName([]string{"Not A Tool"})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

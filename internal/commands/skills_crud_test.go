package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestPackSkillLocationFromFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	cmd.Flags().String("location", "global", "")
	_ = cmd.Flags().Set("location", "project")

	got, err := packSkillLocationFromFlag(cmd, "project")
	if err != nil {
		t.Fatal(err)
	}
	if got != "project" {
		t.Fatalf("got %q", got)
	}
}

func TestPackSkillLocationFromFlag_omittedUsesFrontmatter(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	cmd.Flags().String("location", "global", "")

	got, err := packSkillLocationFromFlag(cmd, "global")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected empty when flag omitted, got %q", got)
	}
}

func TestPackSkillLocationFromFlag_invalid(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	cmd.Flags().String("location", "global", "")
	_ = cmd.Flags().Set("location", "invalid")

	_, err := packSkillLocationFromFlag(cmd, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid location")
	}
}

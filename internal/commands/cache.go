package commands

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// RegisterCache registers the cache command group.
func RegisterCache(rootCmd *cobra.Command) {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage locally cached Port data",
		Long:  `Manage data that Port CLI caches locally on your machine.`,
	}

	cacheCmd.AddCommand(registerCacheClear())

	rootCmd.AddCommand(cacheCmd)
}

func registerCacheClear() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete all locally synced Port skills from AI tool directories",
		Long: `Delete all Port skills that were synced by 'port skills sync'.

This removes the skills/port/ directory from every configured AI tool target
(e.g. ~/.cursor/skills/port/, ~/.claude/skills/port/, ~/.gemini/skills/port/)
and from any registered project directories.

Hooks are NOT removed — run 'port skills init' again or edit the hook files
manually if you want to stop auto-syncing. Skills will be re-synced automatically
the next time you start a new AI session.

Use --force to skip the confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newSkillsModule(flags)
			if err != nil {
				return err
			}

			if !force {
				ok, err := confirmPrompt(
					"Delete all locally cached Port skills?",
					"This will remove skills/port/ from all configured AI tool directories.\nHooks will remain in place — skills will be re-synced on the next session start.",
				)
				if err != nil {
					return err
				}
				if !ok {
					lipgloss.Printf("%s Cancelled — no skills were deleted.\n", styles.ExclamationMark)
					return nil
				}
			}

			result, err := mod.ClearSkills()
			if err != nil {
				return fmt.Errorf("failed to clear skills: %w", err)
			}

			for _, t := range result.DeletedTargets {
				lipgloss.Printf("%s Deleted skills/port/ from %s\n", styles.CheckMark, styles.Bold.Render(t))
			}
			for _, t := range result.SkippedTargets {
				lipgloss.Printf("%s Skipped %s (no skills directory found)\n", styles.QuestionMark, t)
			}
			if len(result.DeletedTargets) == 0 {
				lipgloss.Printf("%s No Port skills found locally — nothing to delete.\n", styles.QuestionMark)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt")
	return cmd
}

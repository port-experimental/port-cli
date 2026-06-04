package commands

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

func skillsSelectionNonInteractive(cmd *cobra.Command, selectAllGroups, selectAllUngrouped bool) bool {
	return cmd.Flags().Changed("group") || cmd.Flags().Changed("skill") ||
		cmd.Flags().Changed("select-all-groups") || cmd.Flags().Changed("select-all-ungrouped") ||
		selectAllGroups || selectAllUngrouped
}

func loadSkillsOptsFromSelectionFlags(
	groups, skillsIDs []string,
	selectAllGroups, selectAllUngrouped bool,
	replaceSelection bool,
) (skills.LoadSkillsOptions, error) {
	opts := skills.LoadSkillsOptions{
		SelectAllGroups:    selectAllGroups,
		SelectAllUngrouped: selectAllUngrouped,
		SelectedGroups:     groups,
		SelectedSkills:     skillsIDs,
		ReplaceSelection:   replaceSelection,
	}
	if selectAllGroups && selectAllUngrouped {
		opts.SelectAll = true
	}
	if len(opts.SelectedGroups) == 0 && len(opts.SelectedSkills) == 0 &&
		!opts.SelectAllGroups && !opts.SelectAllUngrouped && !opts.SelectAll {
		return opts, fmt.Errorf("non-interactive selection requires --group, --skill, --select-all-groups, and/or --select-all-ungrouped")
	}
	return opts, nil
}

func runSkillsSelect(cmd *cobra.Command, mod *skills.Module, configManager *config.ConfigManager, interactive bool, loadOpts skills.LoadSkillsOptions) error {
	ctx := cmd.Context()

	if interactive {
		var err error
		loadOpts, _, err = buildLoadSkillsOpts(ctx, mod, configManager, true)
		if err != nil {
			return err
		}
		loadOpts.ReplaceSelection = true
	} else {
		fetched, err := mod.FetchSkills(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch skills from Port: %w", err)
		}
		loadOpts.Fetched = fetched
	}

	if clearResult, err := mod.ClearSkills(); err != nil {
		return fmt.Errorf("failed to clear existing skills: %w", err)
	} else {
		for _, t := range clearResult.DeletedTargets {
			lipgloss.Printf("%s Cleared existing skills from %s\n", styles.CheckMark, styles.Bold.Render(t))
		}
	}

	result, err := mod.LoadSkills(ctx, loadOpts)
	if err != nil {
		return fmt.Errorf("failed to sync skills: %w", err)
	}
	printLoadResult(result)
	if result.GitDirtySkipped {
		return fmt.Errorf("sync skipped for one or more directories due to uncommitted git changes (use --ignore-git-dirty to override)")
	}
	return nil
}

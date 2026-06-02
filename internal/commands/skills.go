package commands

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// RegisterSkills registers the skills command group.
func RegisterSkills(rootCmd *cobra.Command) {
	var skillsOrg string

	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Port AI skills: hooks and local skill sync",
		Long: `Manage Port AI skills: hooks and local skill sync.

Use 'port skills init' to install session-start hooks into your AI tools
(Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, GitHub Copilot).
Once installed, every new AI session will automatically sync your selected skills
from Port.`,
	}
	skillsCmd.PersistentFlags().StringVar(&skillsOrg, "org", "", "Organization name (uses default from config if not specified)")

	skillsCmd.AddCommand(registerSkillsInit())
	skillsCmd.AddCommand(registerSkillsSelect())
	skillsCmd.AddCommand(registerSkillsCreate())
	skillsCmd.AddCommand(registerSkillsEdit())
	skillsCmd.AddCommand(registerSkillsArchive())
	skillsCmd.AddCommand(registerSkillsAdd())
	skillsCmd.AddCommand(registerSkillsRemove())
	skillsCmd.AddCommand(registerSkillsSync())
	skillsCmd.AddCommand(registerSkillsList())
	skillsCmd.AddCommand(registerSkillsSearch())
	skillsCmd.AddCommand(registerSkillsClear())
	skillsCmd.AddCommand(registerSkillsStatus())

	rootCmd.AddCommand(skillsCmd)
}

func registerSkillsInit() *cobra.Command {
	var (
		tools              []string
		installHooks       bool
		groups             []string
		skillsIDs          []string
		selectAllGroups    bool
		selectAllUngrouped bool
		ignoreGitDirty     bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install AI session-start hooks and sync skills from Port",
		Long: `Install AI session-start hooks for Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, and GitHub Copilot.

On every new AI session the hook will run 'port skills sync',
keeping your local skills in sync with the Port registry. Hooks are installed
globally in your home directory for most tools. GitHub Copilot is different:
hooks and synced skills are installed only under <repo>/.github (run init from
the repository root).
Skills are written to the correct location based on each skill's 'location'
property in Port ("global" → AI tool directories, "project" → tool directory
inside each registered project directory). For Copilot, both global and
project skills from Port are written under <repo>/.github/skills/port/.

Non-interactive use: pass --tool and selection flags; add --install-hooks to write hooks.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)
			org := skillsOrgName(cmd)

			nonInteractive := cmd.Flags().Changed("tool") || cmd.Flags().Changed("group") ||
				cmd.Flags().Changed("skill") || selectAllGroups || selectAllUngrouped

			var targets []skills.HookTarget
			if nonInteractive {
				if len(tools) == 0 {
					return fmt.Errorf("non-interactive init requires at least one --tool")
				}
				resolved, err := resolveTargetsByName(tools)
				if err != nil {
					return err
				}
				targets = resolved
			} else {
				if err := RequireInteractive(); err != nil {
					return err
				}
				var err error
				targets, err = promptTargetSelection(configManager)
				if err != nil {
					return err
				}
				installHooks = true
			}

			mod, configManager, err := newSkillsModuleWithFlags(ctx, flags, org)
			if err != nil {
				return err
			}

			if installHooks {
				initResult, err := mod.Init(ctx, skills.InitOptions{Targets: targets})
				if err != nil {
					return fmt.Errorf("failed to install hooks: %w", err)
				}
				for _, t := range initResult.InstalledTargets {
					lipgloss.Printf("%s Hook installed in %s\n", styles.CheckMark, styles.Bold.Render(t))
				}
			} else if nonInteractive {
				if err := mod.RegisterTargets(ctx, targets); err != nil {
					return fmt.Errorf("failed to save tool targets: %w", err)
				}
			}

			var rawFetched *skills.FetchedSkills
			loadOpts, rawFetched, err := buildLoadSkillsOpts(ctx, mod, !nonInteractive)
			if err != nil {
				return err
			}
			if nonInteractive {
				loadOpts, err = loadSkillsOptsFromSelectionFlags(groups, skillsIDs, selectAllGroups, selectAllUngrouped, false)
				if err != nil {
					return err
				}
			}
			loadOpts.Fetched = rawFetched
			loadOpts.IgnoreGitDirty = ignoreGitDirty

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
		},
	}

	cmd.Flags().StringArrayVar(&tools, "tool", nil, "AI tool name to configure (repeatable, e.g. \"Cursor\")")
	cmd.Flags().BoolVar(&installHooks, "install-hooks", false, "Install or update hooks.json / settings.json for --tool targets (non-interactive)")
	cmd.Flags().StringArrayVar(&groups, "group", nil, "Skill group identifier to sync (repeatable)")
	cmd.Flags().StringArrayVar(&skillsIDs, "skill", nil, "Skill identifier to sync (repeatable)")
	cmd.Flags().BoolVar(&selectAllGroups, "select-all-groups", false, "Sync all skill groups")
	cmd.Flags().BoolVar(&selectAllUngrouped, "select-all-ungrouped", false, "Sync all ungrouped skills")
	cmd.Flags().BoolVar(&ignoreGitDirty, "ignore-git-dirty", false, "Write skills even when skills/port has uncommitted git changes")
	return cmd
}

func registerSkillsSelect() *cobra.Command {
	var (
		groups             []string
		skillsIDs          []string
		selectAllGroups    bool
		selectAllUngrouped bool
		ignoreGitDirty     bool
	)

	cmd := &cobra.Command{
		Use:   "select",
		Short: "Change which optional skills and groups are synced",
		Long: `Re-run the skill selection flow from 'port skills init' without reinstalling hooks.

Updates your saved selection in ~/.port/config.yaml, clears previously synced
optional skills from disk, and syncs the new selection. Required skills are always
included and cannot be deselected.

Interactive: run in a terminal to pick groups and ungrouped skills.

Non-interactive: pass --group, --skill, --select-all-groups, and/or
--select-all-ungrouped (same flags as 'port skills init').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, configManager, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			skillsCfg, err := configManager.LoadSkillsConfig()
			if err != nil || len(skillsCfg.Targets) == 0 {
				return fmt.Errorf("no skills configuration found — run 'port skills init' first")
			}

			nonInteractive := skillsSelectionNonInteractive(cmd, selectAllGroups, selectAllUngrouped)
			if !nonInteractive {
				if err := RequireInteractive(); err != nil {
					return err
				}
			}

			loadOpts := skills.LoadSkillsOptions{IgnoreGitDirty: ignoreGitDirty}
			if nonInteractive {
				loadOpts, err = loadSkillsOptsFromSelectionFlags(groups, skillsIDs, selectAllGroups, selectAllUngrouped, true)
				if err != nil {
					return err
				}
			}

			return runSkillsSelect(cmd, mod, !nonInteractive, loadOpts)
		},
	}

	cmd.Flags().StringArrayVar(&groups, "group", nil, "Skill group identifier to sync (repeatable)")
	cmd.Flags().StringArrayVar(&skillsIDs, "skill", nil, "Ungrouped skill identifier to sync (repeatable)")
	cmd.Flags().BoolVar(&selectAllGroups, "select-all-groups", false, "Sync all optional skill groups")
	cmd.Flags().BoolVar(&selectAllUngrouped, "select-all-ungrouped", false, "Sync all ungrouped skills")
	cmd.Flags().BoolVar(&ignoreGitDirty, "ignore-git-dirty", false, "Write skills even when skills/port has uncommitted git changes")
	return cmd
}

func registerSkillsAdd() *cobra.Command {
	var (
		groups    []string
		skillsIDs []string
		tools     []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add skills or AI tools to your existing selection",
		Long: `Add skill groups, individual skills, or AI tool targets to your saved
selection without re-selecting everything configured during 'port skills init'.

When run without flags, an interactive prompt lists only groups, ungrouped
skills, and AI tools that are not already part of your configuration.

After updating the selection, skills are synced to disk (same as 'port skills sync').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, configManager, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			skillsCfg, err := configManager.LoadSkillsConfig()
			if err != nil {
				skillsCfg = &config.SkillsConfig{}
			}

			addOpts := skills.AddSkillsOptions{
				Groups: groups,
				Skills: skillsIDs,
			}

			nonInteractive := cmd.Flags().Changed("group") || cmd.Flags().Changed("skill") || cmd.Flags().Changed("tool")
			if !nonInteractive {
				fetched, err := mod.FetchSkills(ctx)
				if err != nil {
					return fmt.Errorf("failed to fetch skills from Port: %w", err)
				}

				availableGroups := skills.AvailableGroupsToAdd(skillsCfg, fetched)
				if len(availableGroups) > 0 {
					selected, err := promptAddGroupSelection(availableGroups)
					if err != nil {
						return err
					}
					addOpts.Groups = append(addOpts.Groups, selected...)
				}

				availableSkills := skills.AvailableSkillsToAdd(skillsCfg, fetched)
				if len(availableSkills) > 0 {
					selected, err := promptAddSkillSelection(availableSkills)
					if err != nil {
						return err
					}
					addOpts.Skills = append(addOpts.Skills, selected...)
				}

				unconfigured, err := unconfiguredHookTargets(configManager)
				if err != nil {
					return err
				}
				if len(unconfigured) > 0 {
					configuredTools, err := configuredHookTargetNames(configManager)
					if err != nil {
						return err
					}
					targets, err := promptAddTargetSelection(unconfigured, configuredTools)
					if err != nil {
						return err
					}
					addOpts.Targets = targets
				}

				if len(addOpts.Groups) == 0 && len(addOpts.Skills) == 0 && len(addOpts.Targets) == 0 {
					lipgloss.Printf("%s Nothing new to add — your current selection already includes all optional skills and configured tools.\n", styles.QuestionMark)
					return nil
				}
			} else if len(tools) > 0 {
				resolved, err := resolveTargetsByName(tools)
				if err != nil {
					return err
				}
				addOpts.Targets = resolved
			}

			if !skillsCfg.HasSelection() && len(addOpts.Targets) == 0 &&
				len(addOpts.Groups) == 0 && len(addOpts.Skills) == 0 {
				return fmt.Errorf("no skill selection configured — run 'port skills init' first")
			}
			if nonInteractive && len(addOpts.Groups) == 0 && len(addOpts.Skills) == 0 && len(addOpts.Targets) == 0 {
				return fmt.Errorf("specify at least one of --group, --skill, or --tool")
			}

			result, err := mod.AddSkills(ctx, addOpts)
			if err != nil {
				return err
			}

			for _, t := range result.NewTargets {
				lipgloss.Printf("%s Hook installed for %s\n", styles.CheckMark, styles.Bold.Render(t))
			}
			for _, g := range result.Merge.AddedGroups {
				lipgloss.Printf("%s Added group %s\n", styles.CheckMark, styles.Bold.Render(g))
			}
			for _, s := range result.Merge.AddedSkills {
				lipgloss.Printf("%s Added skill %s\n", styles.CheckMark, styles.Bold.Render(s))
			}
			for _, g := range result.Merge.SkippedGroups {
				lipgloss.Printf("%s Group %s already in your selection\n", styles.QuestionMark, g)
			}
			for _, s := range result.Merge.SkippedSkills {
				lipgloss.Printf("%s Skill %s already in your selection\n", styles.QuestionMark, s)
			}

			if result.Sync != nil {
				printLoadResult(result.Sync)
			} else if !result.Merge.HasChanges() && len(result.NewTargets) == 0 {
				lipgloss.Printf("%s No changes were made.\n", styles.QuestionMark)
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&groups, "group", nil, "Skill group identifier to add (repeatable)")
	cmd.Flags().StringArrayVar(&skillsIDs, "skill", nil, "Ungrouped or individual skill identifier to add (repeatable)")
	cmd.Flags().StringArrayVar(&tools, "tool", nil, "AI tool name to install hooks for (repeatable, e.g. \"Cursor\")")
	return cmd
}

func registerSkillsRemove() *cobra.Command {
	var (
		groups    []string
		skillsIDs []string
		tools     []string
	)

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove skills, groups, or AI tools from your selection",
		Long: `Remove skill groups, individual skills, or AI tool targets from your saved
selection.

When run without flags, an interactive prompt lists only items currently in
your configuration. Removed targets have their hooks uninstalled and their
synced skills/port/ directory deleted. Required skills cannot be removed.

If your selection currently uses "all groups" or "all ungrouped skills",
removing a single item first materializes the selection into explicit lists.
Future items added in Port will no longer auto-sync — run 'port skills add'
to include them.

After updating the selection, remaining skills are re-synced to disk.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, configManager, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			skillsCfg, err := configManager.LoadSkillsConfig()
			if err != nil || (!skillsCfg.HasSelection() && len(skillsCfg.Targets) == 0) {
				return fmt.Errorf("no skills configuration found — run 'port skills init' first")
			}

			removeOpts := skills.RemoveSkillsOptions{
				Groups: groups,
				Skills: skillsIDs,
			}

			nonInteractive := cmd.Flags().Changed("group") || cmd.Flags().Changed("skill") || cmd.Flags().Changed("tool")
			if nonInteractive {
				if len(tools) > 0 {
					resolved, err := resolveTargetsByName(tools)
					if err != nil {
						return err
					}
					removeOpts.Targets = resolved
				}
				if len(removeOpts.Groups) == 0 && len(removeOpts.Skills) == 0 && len(removeOpts.Targets) == 0 {
					return fmt.Errorf("specify at least one of --group, --skill, or --tool")
				}
			} else {
				fetched, err := mod.FetchSkills(ctx)
				if err != nil {
					return fmt.Errorf("failed to fetch skills from Port: %w", err)
				}

				removableGroups := skills.RemovableGroups(skillsCfg, fetched)
				if len(removableGroups) > 0 {
					selected, err := promptRemoveGroupSelection(removableGroups)
					if err != nil {
						return err
					}
					removeOpts.Groups = append(removeOpts.Groups, selected...)
				}

				removableSkills := skills.RemovableSkills(skillsCfg, fetched)
				if len(removableSkills) > 0 {
					selected, err := promptRemoveSkillSelection(removableSkills)
					if err != nil {
						return err
					}
					removeOpts.Skills = append(removeOpts.Skills, selected...)
				}

				configuredTargets, err := configuredHookTargets(configManager)
				if err != nil {
					return err
				}
				if len(configuredTargets) > 0 {
					selected, err := promptRemoveTargetSelection(configuredTargets)
					if err != nil {
						return err
					}
					removeOpts.Targets = selected
				}

				if len(removeOpts.Groups) == 0 && len(removeOpts.Skills) == 0 && len(removeOpts.Targets) == 0 {
					lipgloss.Printf("%s Nothing selected — no changes made.\n", styles.QuestionMark)
					return nil
				}

				if !ShouldSkipConfirm(cmd, false) {
					ok, err := confirmPrompt(
						"Apply these removals?",
						"Hooks for selected tools will be uninstalled and their synced skills deleted. Removed groups/skills will be pruned from local AI tool directories.",
					)
					if err != nil {
						return err
					}
					if !ok {
						lipgloss.Printf("%s Cancelled — no changes made.\n", styles.ExclamationMark)
						return nil
					}
				}
			}

			result, err := mod.RemoveSkills(ctx, removeOpts)
			if err != nil {
				return err
			}

			if result.Remove.Materialized {
				lipgloss.Printf(
					"%s Selection switched from \"all\" to specific items. Future groups or skills added in Port will not auto-sync — run 'port skills add' to include them.\n",
					styles.ExclamationMark,
				)
			}
			for _, t := range result.RemovedTargets {
				lipgloss.Printf("%s Hook removed from %s\n", styles.CheckMark, styles.Bold.Render(t))
			}
			for _, g := range result.Remove.RemovedGroups {
				lipgloss.Printf("%s Removed group %s\n", styles.CheckMark, styles.Bold.Render(g))
			}
			for _, s := range result.Remove.RemovedSkills {
				lipgloss.Printf("%s Removed skill %s\n", styles.CheckMark, styles.Bold.Render(s))
			}
			for _, g := range result.Remove.SkippedGroups {
				lipgloss.Printf("%s Skipped group %s (required or not in selection)\n", styles.QuestionMark, g)
			}
			for _, s := range result.Remove.SkippedSkills {
				lipgloss.Printf("%s Skipped skill %s (required or not in selection)\n", styles.QuestionMark, s)
			}

			if result.Sync != nil {
				printLoadResult(result.Sync)
			} else if !result.Remove.HasChanges() && len(result.RemovedTargets) == 0 {
				lipgloss.Printf("%s No changes were made.\n", styles.QuestionMark)
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&groups, "group", nil, "Skill group identifier to remove (repeatable)")
	cmd.Flags().StringArrayVar(&skillsIDs, "skill", nil, "Skill identifier to remove (repeatable)")
	cmd.Flags().StringArrayVar(&tools, "tool", nil, "AI tool name to remove hooks for (repeatable, e.g. \"Cursor\")")
	return cmd
}

func registerSkillsSync() *cobra.Command {
	var ignoreGitDirty bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch skills from Port and sync them to local AI tool directories",
		Long: `Fetch skills from Port and sync them to the appropriate directories.

Uses the selection configured during 'port skills init'. Skills with
location="global" are written to your configured AI tool directories; skills with
location="project" are written under each registered project directory (per tool).
GitHub Copilot uses only <repo>/.github/skills/port/ for synced skills when Copilot
is enabled — there is no global ~/.copilot path.
Required skills are always included. Skills removed from Port are deleted
locally. Run 'port skills select' or 'port skills init' to change your selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)

			mod, configManager, err := newSkillsModuleWithFlags(ctx, flags, skillsOrgName(cmd))
			if err != nil {
				return err
			}

			skillsCfg, err := configManager.LoadSkillsConfig()
			if err != nil || !skillsCfg.HasSelection() {
				return fmt.Errorf("no skill selection configured — run 'port skills init' first")
			}

			result, err := mod.LoadSkills(ctx, skills.LoadSkillsOptions{
				IgnoreGitDirty: ignoreGitDirty,
			})
			if err != nil {
				return fmt.Errorf("failed to sync skills: %w", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				printLoadResult(result)
			}
			if result.GitDirtySkipped {
				return fmt.Errorf("sync skipped for one or more directories due to uncommitted git changes (use --ignore-git-dirty to override)")
			}
			return nil
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "Suppress output (used automatically by AI tool hooks)")
	cmd.Flags().BoolVar(&ignoreGitDirty, "ignore-git-dirty", false, "Write skills even when skills/port has uncommitted git changes")
	return cmd
}

func registerSkillsClear() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete all locally synced Port skills from AI tool directories",
		Long: `Delete all Port skills that were synced by 'port skills sync'.

This removes the skills/port/ directory from every configured AI tool target
(e.g. ~/.cursor/skills/port/, ~/.claude/skills/port/, ~/.gemini/skills/port/)
and from any registered project directories.

Hooks are NOT removed — run 'port skills init' again to reinstall, or run
'port cache clear' to fully remove everything Port CLI installed.

Use --force to skip the confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newSkillsModule(flags)
			if err != nil {
				return err
			}

			if !ShouldSkipConfirm(cmd, force) {
				ok, err := confirmPrompt(
					"Delete all locally synced Port skills?",
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

func registerSkillsStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current skills configuration and last sync time",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newSkillsModule(flags)
			if err != nil {
				return err
			}

			status, err := mod.Status()
			if err != nil {
				return fmt.Errorf("failed to get skills status: %w", err)
			}

			printSkillsStatus(status)
			return nil
		},
	}
}

// --- shared helpers ---

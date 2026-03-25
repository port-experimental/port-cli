package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/plugin"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// RegisterPlugin registers the plugin command group.
func RegisterPlugin(rootCmd *cobra.Command) {
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage Port AI skill hooks and local skill sync",
		Long: `Manage Port AI skill hooks and local skill sync.

Use 'port plugin init' to install session-start hooks into your AI tools
(Cursor, Claude Code, Agents). Once installed, every new AI session will
automatically sync your selected skills from Port.`,
	}

	pluginCmd.AddCommand(registerPluginInit())
	pluginCmd.AddCommand(registerPluginLoadSkills())
	pluginCmd.AddCommand(registerPluginClearSkills())
	pluginCmd.AddCommand(registerPluginStatus())
	pluginCmd.AddCommand(registerPluginRemove())

	rootCmd.AddCommand(pluginCmd)
}

// --- init ---

func registerPluginInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install AI session-start hooks and sync skills from Port",
		Long: `Install AI session-start hooks for Cursor, Claude Code, and Agents.

On every new AI session the hook will run 'port plugin reconcile',
keeping your local skills in sync with the Port registry.

You will be asked whether to install the hooks globally (in your home
directory, affecting all projects) or locally (in the current directory,
affecting only this project).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			// --- step 1: scope ---
			scope := "global"
			scopeForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Where should the hooks be installed?").
						Description("Global installs apply to all projects; local installs apply only to this directory.").
						Options(
							huh.NewOption("Global (home directory)", "global"),
							huh.NewOption("Local (current directory only)", "local"),
						).
						Value(&scope),
				),
			).WithTheme(&themeBase{})
			if err := scopeForm.Run(); err != nil {
				return fmt.Errorf("prompt error: %w", err)
			}

			scopeRoot, err := resolveScopeRoot(scope)
			if err != nil {
				return err
			}

			// --- step 2: which AI tools ---
			allTargets := plugin.DefaultHookTargets()
			targetOptions := make([]huh.Option[string], 0, len(allTargets))
			for _, t := range allTargets {
				targetOptions = append(targetOptions, huh.NewOption(t.Name, t.Name))
			}

			var selectedTargetNames []string
			targetForm := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which AI tools should have hooks installed?").
						Description("Use space to select/deselect, enter to confirm.").
						Options(targetOptions...).
						Height(len(targetOptions) + 4).
						Value(&selectedTargetNames),
				),
			).WithHeight(0).WithTheme(&themeBase{})
			if err := targetForm.Run(); err != nil {
				return fmt.Errorf("prompt error: %w", err)
			}

			if len(selectedTargetNames) == 0 {
				return fmt.Errorf("no AI tools selected — nothing to install")
			}

			selectedNameSet := make(map[string]bool, len(selectedTargetNames))
			for _, n := range selectedTargetNames {
				selectedNameSet[n] = true
			}
			var targets []plugin.HookTarget
			for _, t := range allTargets {
				if selectedNameSet[t.Name] {
					targets = append(targets, t)
				}
			}

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, "")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			orgConfig, err := cfg.GetOrgConfig("")
			if err != nil {
				return fmt.Errorf("failed to get org config: %w", err)
			}

			mod := plugin.NewModule(orgConfig, configManager)

			initResult, err := mod.Init(ctx, plugin.InitOptions{
				Scope:     scope,
				ScopeRoot: scopeRoot,
				Targets:   targets,
			})
			if err != nil {
				return fmt.Errorf("failed to install hooks: %w", err)
			}

			for _, t := range initResult.InstalledTargets {
				lipgloss.Printf("%s Hook installed in %s\n", styles.CheckMark, styles.Bold.Render(t))
			}

			// Always prompt for skill selection on init so the user can
			// review / change their selection every time they re-run init.
			loadOpts, err := buildLoadSkillsOpts(ctx, mod, true)
			if err != nil {
				return err
			}

			// Clear existing skills before writing the new selection so stale
			// skills from a previous run are not left behind.
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
			return nil
		},
	}
}

// --- reconcile ---

func registerPluginLoadSkills() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Fetch skills from Port and sync them to local AI tool directories",
		Long: `Fetch skills from Port and sync them to all configured AI tool directories.

Uses the selection configured during 'port plugin init'. Required skills are
always included. Skills removed from Port are deleted locally. Run
'port plugin init' to change your selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			pluginCfg, err := configManager.LoadPluginConfig()
			if err != nil || (len(pluginCfg.Targets) == 0 && !pluginCfg.SelectAll && !pluginCfg.SelectAllGroups && !pluginCfg.SelectAllUngrouped && len(pluginCfg.SelectedGroups) == 0 && len(pluginCfg.SelectedSkills) == 0) {
				return fmt.Errorf("no skill selection configured — run 'port plugin init' first")
			}

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, "")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			orgConfig, err := cfg.GetOrgConfig("")
			if err != nil {
				return fmt.Errorf("failed to get org config: %w", err)
			}

			mod := plugin.NewModule(orgConfig, configManager)

			result, err := mod.LoadSkills(ctx, plugin.LoadSkillsOptions{})
			if err != nil {
				return fmt.Errorf("failed to sync skills: %w", err)
			}
			printLoadResult(result)
			return nil
		},
	}

	return cmd
}

// --- clear ---

func registerPluginClearSkills() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete all locally synced Port skills from AI tool directories",
		Long: `Delete all Port skills that were synced by 'port plugin reconcile'.

This removes the skills/port/ directory from every configured AI tool target
(~/.cursor/skills/port/, ~/.claude/skills/port/, ~/.agents/skills/port/).

Hooks are NOT removed — run 'port plugin init' again or edit the hook files
manually if you want to stop auto-syncing.

Use --force to skip the confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newPluginModule(flags)
			if err != nil {
				return err
			}

			if !force {
				confirmed := false
				confirmForm := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Delete all locally synced Port skills?").
							Description("This will remove skills/port/ from all configured AI tool directories.\nHooks will remain in place — skills will be re-synced on the next session start.").
							Value(&confirmed),
					),
				).WithTheme(&themeBase{})
				if err := confirmForm.Run(); err != nil {
					return fmt.Errorf("prompt error: %w", err)
				}
				if !confirmed {
					lipgloss.Printf("%s Cancelled — no skills were deleted.\n", styles.QuestionMark)
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
				lipgloss.Printf("  Skipped %s (no skills directory found)\n", t)
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

// --- remove ---

func registerPluginRemove() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Fully uninstall the Port plugin (hooks, skills, and config)",
		Long: `Remove everything installed by 'port plugin init':

  • Port hook entries from hooks.json / settings.json (other hooks are preserved)
  • Generated hook script (hooks/port-reconcile.sh or .cmd)
  • Locally synced skills directories (skills/port/)
  • The plugin section from ~/.port/config.yaml

Other entries already in your hooks files are left untouched.
Use --force to skip the confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newPluginModule(flags)
			if err != nil {
				return err
			}

			if !force {
				confirmed := false
				confirmForm := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Remove the Port plugin?").
							Description("This will remove all Port hooks, skill files, and plugin config.\nOther hooks in your AI tool configs will be left untouched.").
							Value(&confirmed),
					),
				).WithTheme(&themeBase{})
				if err := confirmForm.Run(); err != nil {
					return fmt.Errorf("prompt error: %w", err)
				}
				if !confirmed {
					lipgloss.Printf("%s Cancelled — nothing was removed.\n", styles.QuestionMark)
					return nil
				}
			}

			result, err := mod.Remove()
			if err != nil {
				return fmt.Errorf("failed to remove plugin: %w", err)
			}

			for _, t := range result.HooksResult.RemovedFrom {
				lipgloss.Printf("%s Removed Port hook from %s\n", styles.CheckMark, styles.Bold.Render(t))
			}
			for _, t := range result.HooksResult.Skipped {
				lipgloss.Printf("  Skipped %s (no hook file found)\n", t)
			}
			for _, t := range result.SkillsResult.DeletedTargets {
				lipgloss.Printf("%s Deleted skills/port/ from %s\n", styles.CheckMark, styles.Bold.Render(t))
			}
			lipgloss.Printf("%s Plugin config cleared.\n", styles.CheckMark)
			lipgloss.Printf("\n%s Port plugin fully removed.\n", styles.CheckMark)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt")
	return cmd
}

// --- status ---

func registerPluginStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current plugin configuration and last sync time",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			mod, _, err := newPluginModule(flags)
			if err != nil {
				return err
			}

			status, err := mod.Status()
			if err != nil {
				return fmt.Errorf("failed to get plugin status: %w", err)
			}

			fmt.Println("\nPort Plugin Status")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("Scope:           %s\n", valueOrNone(status.Scope))
			fmt.Printf("Last synced:     %s\n", valueOrNone(status.LastSyncedAt))
			fmt.Printf("\nTargets (%d):\n", len(status.Targets))
			for _, t := range status.Targets {
				fmt.Printf("  - %s/skills/port/\n", t)
			}
			fmt.Printf("\nSkill selection:\n")
			if status.SelectAll {
				fmt.Println("  Groups:           all")
				fmt.Println("  Ungrouped skills: all")
			} else {
				if status.SelectAllGroups {
					fmt.Println("  Groups:           all")
				} else {
					fmt.Printf("  Groups (%d):\n", len(status.SelectedGroups))
					if len(status.SelectedGroups) == 0 {
						fmt.Println("    (none)")
					}
					for _, g := range status.SelectedGroups {
						fmt.Printf("    - %s\n", g)
					}
				}
				if status.SelectAllUngrouped {
					fmt.Println("  Ungrouped skills: all")
				} else {
					fmt.Printf("  Ungrouped skills (%d):\n", len(status.SelectedSkills))
					if len(status.SelectedSkills) == 0 {
						fmt.Println("    (none)")
					}
					for _, s := range status.SelectedSkills {
						fmt.Printf("    - %s\n", s)
					}
				}
			}
			return nil
		},
	}
}

// --- shared helpers ---

// newPluginModule creates a Module that does not need live API credentials.
// Used by commands (clear, remove, status) that only operate on local state.
func newPluginModule(flags GlobalFlags) (*plugin.Module, *config.ConfigManager, error) {
	configManager := config.NewConfigManager(flags.ConfigFile)
	cfg, err := configManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	orgCfg := &config.OrganizationConfig{APIURL: "https://api.getport.io/v1"}
	if cfg.DefaultOrg != "" {
		if oc, ocErr := cfg.GetOrgConfig(cfg.DefaultOrg); ocErr == nil {
			orgCfg = oc
		}
	}
	return plugin.NewModule(orgCfg, configManager), configManager, nil
}

// buildLoadSkillsOpts either returns an empty options struct (use saved config)
// or walks the user through an interactive skill selection flow.
//
// Flow:
//  1. Ask "Sync all groups?" → yes → confirm list → done (SelectAll=true)
//  2. No → multi-select individual groups → print confirmed selection
//  3. Ask "Sync all remaining skills?" → yes → confirm list → done (SelectAll=true)
//  4. No → multi-select individual skills → print confirmed selection
func buildLoadSkillsOpts(ctx context.Context, mod *plugin.Module, promptSelection bool) (plugin.LoadSkillsOptions, error) {
	if !promptSelection {
		return plugin.LoadSkillsOptions{}, nil
	}

	fetched, err := mod.FetchSkills(ctx)
	if err != nil {
		return plugin.LoadSkillsOptions{}, fmt.Errorf("failed to fetch skills from Port: %w", err)
	}

	// Print required skills notice.
	if len(fetched.Required) > 0 {
		requiredNames := make([]string, 0, len(fetched.Required))
		for _, s := range fetched.Required {
			name := s.Title
			if name == "" {
				name = s.Identifier
			}
			requiredNames = append(requiredNames, name)
		}
		lipgloss.Printf(
			"\n%s Required skills (always synced regardless of selection):\n  %s\n\n",
			styles.CheckMark,
			strings.Join(requiredNames, ", "),
		)
	}

	if len(fetched.Optional) == 0 && len(fetched.Groups) == 0 {
		lipgloss.Printf("%s No optional skills found — only required skills will be synced.\n", styles.QuestionMark)
		return plugin.LoadSkillsOptions{}, nil
	}

	// ── step 1: grouped skills ───────────────────────────────────────────────
	var selectedGroups []string
	selectAllGroups := false

	if len(fetched.Groups) > 0 {
		syncAllGroups := false
		allGroupsForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Sync all skill groups?").
					Description(fmt.Sprintf("%d group(s) available. Yes = sync all groups, No = pick specific groups.", len(fetched.Groups))).
					Value(&syncAllGroups),
			),
		).WithTheme(&themeBase{})
		if err := allGroupsForm.Run(); err != nil {
			return plugin.LoadSkillsOptions{}, fmt.Errorf("prompt error: %w", err)
		}

		if syncAllGroups {
			selectAllGroups = true
			lipgloss.Printf("\n%s All groups selected:\n", styles.CheckMark)
			for _, g := range fetched.Groups {
				label := g.Title
				if label == "" {
					label = g.Identifier
				}
				lipgloss.Printf("  %s %s\n", styles.CheckMark, label)
			}
			fmt.Println()
		} else {
			groupOptions := make([]huh.Option[string], 0, len(fetched.Groups))
			for _, g := range fetched.Groups {
				label := g.Title
				if label == "" {
					label = g.Identifier
				}
				groupOptions = append(groupOptions, huh.NewOption(label, g.Identifier))
			}
			groupPickForm := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which skill groups would you like to sync?").
						Description("Use space to select/deselect, enter to confirm.").
						Options(groupOptions...).
						Height(len(groupOptions) + 4).
						Value(&selectedGroups),
				),
			).WithHeight(0).WithTheme(&themeBase{})
			if err := groupPickForm.Run(); err != nil {
				return plugin.LoadSkillsOptions{}, fmt.Errorf("prompt error: %w", err)
			}

			selectedGroupSet := toStringSet(selectedGroups)
			lipgloss.Printf("\n%s Groups:\n", styles.CheckMark)
			for _, g := range fetched.Groups {
				label := g.Title
				if label == "" {
					label = g.Identifier
				}
				if selectedGroupSet[g.Identifier] {
					lipgloss.Printf("  %s %s\n", styles.CheckMark, label)
				} else {
					fmt.Printf("  ○ %s\n", label)
				}
			}
			fmt.Println()
		}
	}

	// ── step 2: ungrouped skills ─────────────────────────────────────────────
	var ungroupedSkills []plugin.Skill
	for _, s := range fetched.Optional {
		if s.GroupID == "" {
			ungroupedSkills = append(ungroupedSkills, s)
		}
	}

	var selectedSkills []string
	selectAllUngrouped := false

	if len(ungroupedSkills) > 0 {
		syncAllUngrouped := false
		allUngroupedForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Sync all skills without a group?").
					Description(fmt.Sprintf("%d skill(s) are not part of any group. Yes = sync all, No = pick specific ones.", len(ungroupedSkills))).
					Value(&syncAllUngrouped),
			),
		).WithTheme(&themeBase{})
		if err := allUngroupedForm.Run(); err != nil {
			return plugin.LoadSkillsOptions{}, fmt.Errorf("prompt error: %w", err)
		}

		if syncAllUngrouped {
			selectAllUngrouped = true
			lipgloss.Printf("\n%s All ungrouped skills selected:\n", styles.CheckMark)
			for _, s := range ungroupedSkills {
				label := s.Title
				if label == "" {
					label = s.Identifier
				}
				lipgloss.Printf("  %s %s\n", styles.CheckMark, label)
			}
			fmt.Println()
		} else {
			skillOptions := make([]huh.Option[string], 0, len(ungroupedSkills))
			for _, s := range ungroupedSkills {
				label := s.Title
				if label == "" {
					label = s.Identifier
				}
				skillOptions = append(skillOptions, huh.NewOption(label, s.Identifier))
			}
			skillPickForm := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which ungrouped skills would you like to sync?").
						Description("These skills have no group. Use space to select/deselect, enter to confirm.").
						Options(skillOptions...).
						Height(len(skillOptions) + 4).
						Value(&selectedSkills),
				),
			).WithHeight(0).WithTheme(&themeBase{})
			if err := skillPickForm.Run(); err != nil {
				return plugin.LoadSkillsOptions{}, fmt.Errorf("prompt error: %w", err)
			}

			selectedSkillSet := toStringSet(selectedSkills)
			lipgloss.Printf("\n%s Ungrouped skills:\n", styles.CheckMark)
			for _, s := range ungroupedSkills {
				label := s.Title
				if label == "" {
					label = s.Identifier
				}
				if selectedSkillSet[s.Identifier] {
					lipgloss.Printf("  %s %s\n", styles.CheckMark, label)
				} else {
					fmt.Printf("  ○ %s\n", label)
				}
			}
			fmt.Println()
		}
	}

	return plugin.LoadSkillsOptions{
		SelectAllGroups:    selectAllGroups,
		SelectAllUngrouped: selectAllUngrouped,
		SelectedGroups:     selectedGroups,
		SelectedSkills:     selectedSkills,
	}, nil
}

func toStringSet(slice []string) map[string]bool {
	s := make(map[string]bool, len(slice))
	for _, v := range slice {
		s[v] = true
	}
	return s
}

func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func resolveScopeRoot(scope string) (string, error) {
	if scope == "local" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		return cwd, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return home, nil
}

func printLoadResult(result *plugin.LoadSkillsResult) {
	total := result.RequiredCount + result.SelectedCount
	lipgloss.Printf(
		"%s %d skill(s) synced across %d target(s) (%d required, %d selected)\n",
		styles.CheckMark,
		total,
		result.TargetCount,
		result.RequiredCount,
		result.SelectedCount,
	)
}

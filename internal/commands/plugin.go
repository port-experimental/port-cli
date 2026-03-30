package commands

import (
	"context"
	"fmt"
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
(Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, GitHub Copilot).
Once installed, every new AI session will automatically sync your selected skills
from Port.`,
	}

	pluginCmd.AddCommand(registerPluginInit())
	pluginCmd.AddCommand(registerPluginLoadSkills())
	pluginCmd.AddCommand(registerPluginList())
	pluginCmd.AddCommand(registerPluginClearSkills())
	pluginCmd.AddCommand(registerPluginStatus())
	pluginCmd.AddCommand(registerPluginRemove())

	rootCmd.AddCommand(pluginCmd)
}

func registerPluginInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install AI session-start hooks and sync skills from Port",
		Long: `Install AI session-start hooks for Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, and GitHub Copilot.

On every new AI session the hook will run 'port plugin sync',
keeping your local skills in sync with the Port registry. Hooks are installed
globally in your home directory. GitHub Copilot uses the ~/.agents directory,
following the open agent skills standard.
Skills are written to the correct location based on each skill's 'location'
property in Port ("global" → AI tool directories, "project" → tool directory
inside the current repository).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			targets, err := promptTargetSelection(configManager)
			if err != nil {
				return err
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
				Targets: targets,
			})
			if err != nil {
				return fmt.Errorf("failed to install hooks: %w", err)
			}

			for _, t := range initResult.InstalledTargets {
				lipgloss.Printf("%s Hook installed in %s\n", styles.CheckMark, styles.Bold.Render(t))
			}

			loadOpts, err := buildLoadSkillsOpts(ctx, mod, true)
			if err != nil {
				return err
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
			return nil
		},
	}
}

func registerPluginLoadSkills() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Fetch skills from Port and sync them to local AI tool directories",
		Long: `Fetch skills from Port and sync them to the appropriate directories.

Uses the selection configured during 'port plugin init'. Skills with
location="global" are written to your AI tool directories; skills with
location="project" are written to the current working directory.
Required skills are always included. Skills removed from Port are deleted
locally. Run 'port plugin init' to change your selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			pluginCfg, err := configManager.LoadPluginConfig()
			if err != nil || !pluginCfg.HasSelection() {
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
}

func registerPluginList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available skills from Port",
		Long: `Fetch and display all skills available in your Port organization.

Shows skills grouped by their skill group, with required skills marked.
This is a read-only command — it does not sync or modify any local files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, "")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			orgConfig, err := cfg.GetOrgConfig("")
			if err != nil {
				return fmt.Errorf("failed to get org config: %w", err)
			}

			mod := plugin.NewModule(orgConfig, configManager)
			fetched, err := mod.FetchSkills(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch skills: %w", err)
			}

			total := len(fetched.Required) + len(fetched.Optional)
			fmt.Printf("\nFound %d skill(s) in %d group(s)\n", total, len(fetched.Groups))
			fmt.Println(strings.Repeat("─", 40))

			if len(fetched.Required) > 0 {
				fmt.Printf("\n%s Required (always synced):\n", styles.CheckMark)
				for _, s := range fetched.Required {
					printSkillLine(s, fetched.Groups)
				}
			}

			// Build map of grouped optional skills by group ID.
			groupedSkills := make(map[string][]plugin.Skill)
			var ungrouped []plugin.Skill
			for _, s := range fetched.Optional {
				if s.GroupID == "" {
					ungrouped = append(ungrouped, s)
				} else {
					groupedSkills[s.GroupID] = append(groupedSkills[s.GroupID], s)
				}
			}

			for _, g := range fetched.Groups {
				skills := groupedSkills[g.Identifier]
				if len(skills) == 0 {
					continue
				}
				label := g.Title
				if label == "" {
					label = g.Identifier
				}
				fmt.Printf("\n%s (%d):\n", styles.Bold.Render(label), len(skills))
				for _, s := range skills {
					printSkillLine(s, fetched.Groups)
				}
			}

			if len(ungrouped) > 0 {
				fmt.Printf("\n%s (%d):\n", styles.Bold.Render("Ungrouped"), len(ungrouped))
				for _, s := range ungrouped {
					printSkillLine(s, fetched.Groups)
				}
			}

			fmt.Println()
			return nil
		},
	}
}

func printSkillLine(s plugin.Skill, groups []plugin.SkillGroup) {
	name := s.Title
	if name == "" {
		name = s.Identifier
	}
	loc := "global"
	if s.Location == plugin.SkillLocationProject {
		loc = "project"
	}
	fmt.Printf("  %-40s [%s]\n", name, loc)
}

func registerPluginClearSkills() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete all locally synced Port skills from AI tool directories",
		Long: `Delete all Port skills that were synced by 'port plugin sync'.

This removes the skills/port/ directory from every configured AI tool target
(e.g. ~/.cursor/skills/port/, ~/.claude/skills/port/, ~/.gemini/skills/port/).

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

func registerPluginRemove() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Fully uninstall the Port plugin (hooks, skills, and config)",
		Long: `Remove everything installed by 'port plugin init':

  • Port hook entries from hooks.json / settings.json (other hooks are preserved)
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
				ok, err := confirmPrompt(
					"Remove the Port plugin?",
					"This will remove all Port hooks, skill files, and plugin config.\nOther hooks in your AI tool configs will be left untouched.",
				)
				if err != nil {
					return err
				}
				if !ok {
					lipgloss.Printf("%s Cancelled — nothing was removed.\n", styles.ExclamationMark)
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
				lipgloss.Printf("%s Skipped %s (no hook file found)\n", styles.QuestionMark, t)
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

			printPluginStatus(status)
			return nil
		},
	}
}

// --- shared helpers ---

// newPluginModule creates a Module for commands that only operate on local state.
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

// confirmPrompt shows a yes/no confirmation and returns whether the user accepted.
func confirmPrompt(title, description string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&confirmed),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("prompt error: %w", err)
	}
	return confirmed, nil
}

// promptTargetSelection shows an interactive multi-select of AI tools and
// returns the selected HookTargets. Previously saved targets are pre-selected.
func promptTargetSelection(configManager *config.ConfigManager) ([]plugin.HookTarget, error) {
	allTargets := plugin.DefaultHookTargets()

	var preSelected []string
	if configManager != nil {
		if pluginCfg, err := configManager.LoadPluginConfig(); err == nil {
			preSelected = plugin.ResolveTargetNames(pluginCfg.Targets, allTargets)
		}
	}

	targetOptions := make([]huh.Option[string], 0, len(allTargets))
	for _, t := range allTargets {
		label := t.Name
		if t.Note != "" {
			label = fmt.Sprintf("%s (%s)", t.Name, t.Note)
		}
		opt := huh.NewOption(label, t.Name)
		for _, ps := range preSelected {
			if ps == t.Name {
				opt = opt.Selected(true)
				break
			}
		}
		targetOptions = append(targetOptions, opt)
	}

	selectedNames := make([]string, len(preSelected))
	copy(selectedNames, preSelected)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which AI tools should have hooks installed?").
				Description("Use space to select/deselect, enter to confirm.").
				Options(targetOptions...).
				Height(len(targetOptions) + 4).
				Value(&selectedNames),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt error: %w", err)
	}

	if len(selectedNames) == 0 {
		return nil, fmt.Errorf("no AI tools selected — nothing to install")
	}

	nameSet := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		nameSet[n] = true
	}
	var targets []plugin.HookTarget
	for _, t := range allTargets {
		if nameSet[t.Name] {
			targets = append(targets, t)
		}
	}
	return targets, nil
}

func buildLoadSkillsOpts(ctx context.Context, mod *plugin.Module, promptSelection bool) (plugin.LoadSkillsOptions, error) {
	if !promptSelection {
		return plugin.LoadSkillsOptions{}, nil
	}

	fetched, err := mod.FetchSkills(ctx)
	if err != nil {
		return plugin.LoadSkillsOptions{}, fmt.Errorf("failed to fetch skills from Port: %w", err)
	}

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

	selectAllGroups, selectedGroups, err := promptGroupSelection(fetched.Groups)
	if err != nil {
		return plugin.LoadSkillsOptions{}, err
	}

	var ungroupedSkills []plugin.Skill
	for _, s := range fetched.Optional {
		if s.GroupID == "" {
			ungroupedSkills = append(ungroupedSkills, s)
		}
	}

	selectAllUngrouped, selectedSkills, err := promptUngroupedSelection(ungroupedSkills)
	if err != nil {
		return plugin.LoadSkillsOptions{}, err
	}

	return plugin.LoadSkillsOptions{
		SelectAllGroups:    selectAllGroups,
		SelectAllUngrouped: selectAllUngrouped,
		SelectedGroups:     selectedGroups,
		SelectedSkills:     selectedSkills,
	}, nil
}

func promptGroupSelection(groups []plugin.SkillGroup) (selectAll bool, selected []string, err error) {
	if len(groups) == 0 {
		return false, nil, nil
	}

	syncAll := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync all skill groups?").
				Description(fmt.Sprintf("%d group(s) available. Yes = sync all groups, No = pick specific groups.", len(groups))).
				Value(&syncAll),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	if syncAll {
		lipgloss.Printf("\n%s All groups selected:\n", styles.CheckMark)
		for _, g := range groups {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, groupLabel(g))
		}
		fmt.Println()
		return true, nil, nil
	}

	groupOptions := make([]huh.Option[string], 0, len(groups))
	for _, g := range groups {
		groupOptions = append(groupOptions, huh.NewOption(groupLabel(g), g.Identifier))
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which skill groups would you like to sync?").
				Description("Use space to select/deselect, enter to confirm.").
				Options(groupOptions...).
				Height(len(groupOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := pickForm.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	selectedSet := toStringSet(selected)
	lipgloss.Printf("\n%s Groups:\n", styles.CheckMark)
	for _, g := range groups {
		if selectedSet[g.Identifier] {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, groupLabel(g))
		} else {
			fmt.Printf("  ○ %s\n", groupLabel(g))
		}
	}
	fmt.Println()

	return false, selected, nil
}

func promptUngroupedSelection(ungroupedSkills []plugin.Skill) (selectAll bool, selected []string, err error) {
	if len(ungroupedSkills) == 0 {
		return false, nil, nil
	}

	syncAll := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sync all skills without a group?").
				Description(fmt.Sprintf("%d skill(s) are not part of any group. Yes = sync all, No = pick specific ones.", len(ungroupedSkills))).
				Value(&syncAll),
		),
	).WithTheme(&styles.FormTheme{})
	if err := form.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	if syncAll {
		lipgloss.Printf("\n%s All ungrouped skills selected:\n", styles.CheckMark)
		for _, s := range ungroupedSkills {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, skillLabel(s))
		}
		fmt.Println()
		return true, nil, nil
	}

	skillOptions := make([]huh.Option[string], 0, len(ungroupedSkills))
	for _, s := range ungroupedSkills {
		skillOptions = append(skillOptions, huh.NewOption(skillLabel(s), s.Identifier))
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which ungrouped skills would you like to sync?").
				Description("These skills have no group. Use space to select/deselect, enter to confirm.").
				Options(skillOptions...).
				Height(len(skillOptions) + 4).
				Value(&selected),
		),
	).WithHeight(0).WithTheme(&styles.FormTheme{})
	if err := pickForm.Run(); err != nil {
		return false, nil, fmt.Errorf("prompt error: %w", err)
	}

	selectedSet := toStringSet(selected)
	lipgloss.Printf("\n%s Ungrouped skills:\n", styles.CheckMark)
	for _, s := range ungroupedSkills {
		if selectedSet[s.Identifier] {
			lipgloss.Printf("  %s %s\n", styles.CheckMark, skillLabel(s))
		} else {
			fmt.Printf("  ○ %s\n", skillLabel(s))
		}
	}
	fmt.Println()

	return false, selected, nil
}

func groupLabel(g plugin.SkillGroup) string {
	if g.Title != "" {
		return g.Title
	}
	return g.Identifier
}

func skillLabel(s plugin.Skill) string {
	if s.Title != "" {
		return s.Title
	}
	return s.Identifier
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

func printPluginStatus(status *plugin.StatusResult) {
	fmt.Println("\nPort Plugin Status")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Last synced:     %s\n", valueOrNone(status.LastSyncedAt))
	fmt.Printf("\nHook targets (%d):\n", len(status.Targets))
	for _, t := range status.Targets {
		fmt.Printf("  - %s/skills/port/\n", t)
	}
	fmt.Printf("\nProject directories (%d):\n", len(status.ProjectDirs))
	if len(status.ProjectDirs) == 0 {
		fmt.Println("  (none)")
	}
	for _, d := range status.ProjectDirs {
		fmt.Printf("  - %s\n", d)
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
}

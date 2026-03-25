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
	pluginCmd.AddCommand(registerPluginStatus())

	rootCmd.AddCommand(pluginCmd)
}

// --- init ---

func registerPluginInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install AI session-start hooks and sync skills from Port",
		Long: `Install AI session-start hooks for Cursor, Claude Code, and Agents.

On every new AI session the hook will run 'port plugin load-skills',
keeping your local skills in sync with the Port registry.

You will be asked whether to install the hooks globally (in your home
directory, affecting all projects) or locally (in the current directory,
affecting only this project).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			flags := GetGlobalFlags(ctx)
			configManager := config.NewConfigManager(flags.ConfigFile)

			// --- scope prompt ---
			scope := "global"
			scopeForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Where should the hooks be installed?").
						Description("Global installs apply to all projects; local installs apply only to this directory.").
						Options(
							huh.NewOption("Global (~/.cursor, ~/.claude, ~/.agents)", "global"),
							huh.NewOption("Local (.cursor, .claude, .agents in current directory)", "local"),
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

			targets := plugin.DefaultHookTargets()

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

			// Run skill selection if nothing is saved yet.
			pluginCfg, _ := configManager.LoadPluginConfig()
			needsSelection := pluginCfg == nil ||
				(len(pluginCfg.SelectedGroups) == 0 && len(pluginCfg.SelectedSkills) == 0)

			if needsSelection {
				lipgloss.Printf("\n%s No skill selection found. Let's pick which skills to sync.\n", styles.QuestionMark)
			}

			loadOpts, err := buildLoadSkillsOpts(ctx, mod, needsSelection)
			if err != nil {
				return err
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

// --- load-skills ---

func registerPluginLoadSkills() *cobra.Command {
	var forceSelect bool

	cmd := &cobra.Command{
		Use:   "load-skills",
		Short: "Fetch skills from Port and write them to local AI tool directories",
		Long: `Fetch skills from Port and write them to all configured AI tool directories.

Required skills (marked required=true in Port) are always synced regardless
of your selection. On the first run, or when --select is passed, you will be
prompted to choose which skill groups and individual skills to sync.`,
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

			pluginCfg, _ := configManager.LoadPluginConfig()
			needsSelection := forceSelect ||
				pluginCfg == nil ||
				(len(pluginCfg.SelectedGroups) == 0 && len(pluginCfg.SelectedSkills) == 0)

			loadOpts, err := buildLoadSkillsOpts(ctx, mod, needsSelection)
			if err != nil {
				return err
			}

			result, err := mod.LoadSkills(ctx, loadOpts)
			if err != nil {
				return fmt.Errorf("failed to sync skills: %w", err)
			}
			printLoadResult(result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&forceSelect, "select", false, "Re-select which skill groups and skills to sync")
	return cmd
}

// --- status ---

func registerPluginStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current plugin configuration and last sync time",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.Load()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Status makes no API calls; use a minimal org config as a placeholder.
			orgCfg := &config.OrganizationConfig{APIURL: "https://api.getport.io/v1"}
			if cfg.DefaultOrg != "" {
				if oc, ocErr := cfg.GetOrgConfig(cfg.DefaultOrg); ocErr == nil {
					orgCfg = oc
				}
			}

			mod := plugin.NewModule(orgCfg, configManager)
			status, err := mod.Status()
			if err != nil {
				return fmt.Errorf("failed to get plugin status: %w", err)
			}

			fmt.Println("\nPort Plugin Status")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("Scope:           %s\n", valueOrNone(status.Scope))
			fmt.Printf("Last synced:     %s\n", valueOrNone(status.LastSyncedAt))
			fmt.Printf("Targets (%d):\n", len(status.Targets))
			for _, t := range status.Targets {
				fmt.Printf("  - %s\n", t)
			}
			fmt.Printf("Selected groups (%d):\n", len(status.SelectedGroups))
			for _, g := range status.SelectedGroups {
				fmt.Printf("  - %s\n", g)
			}
			fmt.Printf("Selected skills (%d):\n", len(status.SelectedSkills))
			for _, s := range status.SelectedSkills {
				fmt.Printf("  - %s\n", s)
			}
			return nil
		},
	}
}

// --- shared helpers ---

// buildLoadSkillsOpts either returns an empty options struct (use saved config)
// or fetches skills from Port and presents an interactive multi-select prompt.
func buildLoadSkillsOpts(ctx context.Context, mod *plugin.Module, promptSelection bool) (plugin.LoadSkillsOptions, error) {
	if !promptSelection {
		return plugin.LoadSkillsOptions{}, nil
	}

	// Fetch available skills so we can show them in the prompt.
	fetched, err := mod.FetchSkills(ctx)
	if err != nil {
		return plugin.LoadSkillsOptions{}, fmt.Errorf("failed to fetch skills from Port: %w", err)
	}

	// Build group options. Required skills are listed separately as a note.
	var groupOptions []huh.Option[string]
	for _, g := range fetched.Groups {
		label := g.Title
		if label == "" {
			label = g.Identifier
		}
		groupOptions = append(groupOptions, huh.NewOption(label, g.Identifier))
	}

	// Build individual skill options (optional only; required ones are auto-included).
	var skillOptions []huh.Option[string]
	requiredNames := make([]string, 0)
	for _, s := range fetched.Required {
		name := s.Title
		if name == "" {
			name = s.Identifier
		}
		requiredNames = append(requiredNames, name)
	}
	for _, s := range fetched.Optional {
		label := s.Title
		if label == "" {
			label = s.Identifier
		}
		if s.GroupID != "" {
			label = fmt.Sprintf("%s (%s)", label, s.GroupID)
		}
		skillOptions = append(skillOptions, huh.NewOption(label, s.Identifier))
	}

	var selectedGroups []string
	var selectedSkills []string

	formGroups := []*huh.Group{}

	if len(requiredNames) > 0 {
		note := fmt.Sprintf("The following skills are required and will always be synced:\n  %s",
			strings.Join(requiredNames, ", "))
		lipgloss.Printf("\n%s %s\n", styles.QuestionMark, note)
	}

	if len(groupOptions) > 0 {
		formGroups = append(formGroups,
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select skill groups to sync").
					Description("All skills in a selected group will be synced.").
					Options(groupOptions...).
					Value(&selectedGroups),
			),
		)
	}

	if len(skillOptions) > 0 {
		formGroups = append(formGroups,
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select individual skills to sync").
					Description("These are in addition to the groups selected above.").
					Options(skillOptions...).
					Value(&selectedSkills),
			),
		)
	}

	if len(formGroups) == 0 {
		lipgloss.Printf("%s No optional skills found — only required skills will be synced.\n", styles.QuestionMark)
		return plugin.LoadSkillsOptions{}, nil
	}

	form := huh.NewForm(formGroups...).WithTheme(&themeBase{})
	if err := form.Run(); err != nil {
		return plugin.LoadSkillsOptions{}, fmt.Errorf("selection prompt error: %w", err)
	}

	return plugin.LoadSkillsOptions{
		SelectedGroups: selectedGroups,
		SelectedSkills: selectedSkills,
	}, nil
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

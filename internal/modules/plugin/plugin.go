package plugin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
)

// Module orchestrates the plugin feature: hook installation and skill syncing.
type Module struct {
	client        *api.Client
	configManager *config.ConfigManager
}

// NewModule creates a new plugin module.
func NewModule(orgConfig *config.OrganizationConfig, configManager *config.ConfigManager) *Module {
	client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
	return &Module{
		client:        client,
		configManager: configManager,
	}
}

// FetchSkills returns the full set of skill groups and skills from Port without
// writing anything to disk. Used by the command layer for interactive prompts.
func (m *Module) FetchSkills(ctx context.Context) (*FetchedSkills, error) {
	return FetchSkills(ctx, m.client)
}

// InitOptions holds options for the init operation.
type InitOptions struct {
	// Scope is "global" or "local".
	Scope string
	// ScopeRoot is the resolved root directory (home dir or cwd).
	ScopeRoot string
	// Targets is the list of AI tool hook targets to install.
	Targets []HookTarget
}

// InitResult holds the result of an init operation.
type InitResult struct {
	InstalledTargets []string
}

// Init installs hooks into all configured target directories and persists
// the scope + target paths into the plugin config.
func (m *Module) Init(ctx context.Context, opts InitOptions) (*InitResult, error) {
	if err := InstallHooks(opts.Targets, opts.ScopeRoot); err != nil {
		return nil, fmt.Errorf("failed to install hooks: %w", err)
	}

	targetPaths := TargetPaths(opts.Targets, opts.ScopeRoot)

	pluginCfg, err := m.configManager.LoadPluginConfig()
	if err != nil {
		pluginCfg = &config.PluginConfig{}
	}

	pluginCfg.Scope = opts.Scope
	pluginCfg.Targets = targetPaths

	if err := m.configManager.SavePluginConfig(pluginCfg); err != nil {
		return nil, fmt.Errorf("failed to save plugin config: %w", err)
	}

	return &InitResult{InstalledTargets: targetPaths}, nil
}

// LoadSkillsOptions holds options for the load-skills operation.
type LoadSkillsOptions struct {
	// ForceSelect prompts skill selection even if a selection is already saved.
	ForceSelect bool
	// SelectedGroups overrides the saved group selection (used when the caller
	// has already collected the selection interactively).
	SelectedGroups []string
	// SelectedSkills overrides the saved individual skill selection.
	SelectedSkills []string
}

// LoadSkillsResult summarises what was written.
type LoadSkillsResult struct {
	RequiredCount int
	SelectedCount int
	TargetCount   int
}

// LoadSkills fetches skills from Port and writes them to all configured targets.
// It also persists the skill selection and updates LastSyncedAt.
func (m *Module) LoadSkills(ctx context.Context, opts LoadSkillsOptions) (*LoadSkillsResult, error) {
	pluginCfg, err := m.configManager.LoadPluginConfig()
	if err != nil {
		pluginCfg = &config.PluginConfig{}
	}

	if len(pluginCfg.Targets) == 0 {
		home, _ := os.UserHomeDir()
		defaultTargets := DefaultHookTargets()
		pluginCfg.Targets = TargetPaths(defaultTargets, home)
	}

	fetched, err := FetchSkills(ctx, m.client)
	if err != nil {
		return nil, err
	}

	// Persist the selection if provided.
	if len(opts.SelectedGroups) > 0 || len(opts.SelectedSkills) > 0 {
		pluginCfg.SelectedGroups = opts.SelectedGroups
		pluginCfg.SelectedSkills = opts.SelectedSkills
	}

	skills := FilterSkills(fetched, pluginCfg.SelectedGroups, pluginCfg.SelectedSkills)

	if err := WriteSkills(skills, fetched.Groups, pluginCfg.Targets); err != nil {
		return nil, fmt.Errorf("failed to write skills: %w", err)
	}

	pluginCfg.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.configManager.SavePluginConfig(pluginCfg); err != nil {
		return nil, fmt.Errorf("failed to save plugin config: %w", err)
	}

	requiredCount := 0
	for _, s := range skills {
		if s.Required {
			requiredCount++
		}
	}

	return &LoadSkillsResult{
		RequiredCount: requiredCount,
		SelectedCount: len(skills) - requiredCount,
		TargetCount:   len(pluginCfg.Targets),
	}, nil
}

// StatusResult contains the data surfaced by `port plugin status`.
type StatusResult struct {
	Scope          string
	Targets        []string
	SelectedGroups []string
	SelectedSkills []string
	LastSyncedAt   string
}

// Status returns the current plugin configuration state.
func (m *Module) Status() (*StatusResult, error) {
	pluginCfg, err := m.configManager.LoadPluginConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin config: %w", err)
	}

	return &StatusResult{
		Scope:          pluginCfg.Scope,
		Targets:        pluginCfg.Targets,
		SelectedGroups: pluginCfg.SelectedGroups,
		SelectedSkills: pluginCfg.SelectedSkills,
		LastSyncedAt:   pluginCfg.LastSyncedAt,
	}, nil
}

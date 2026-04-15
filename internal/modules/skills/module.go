package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
)

// Module orchestrates hook installation and skill syncing for Port AI skills.
type Module struct {
	client        *api.Client
	configManager *config.ConfigManager
}

func NewModule(token *auth.Token, orgConfig *config.OrganizationConfig, configManager *config.ConfigManager) *Module {
	client := api.NewClient(api.ClientOpts{
		ClientID:     orgConfig.ClientID,
		ClientSecret: orgConfig.ClientSecret,
		APIURL:       orgConfig.APIURL,
		Token:        token,
	})
	return &Module{
		client:        client,
		configManager: configManager,
	}
}

func (m *Module) FetchSkills(ctx context.Context) (*FetchedSkills, error) {
	return FetchSkills(ctx, m.client)
}

// InitOptions holds options for the init operation.
type InitOptions struct {
	Targets []HookTarget
}

// InitResult holds the result of an init operation.
type InitResult struct {
	InstalledTargets []string
}

// Init installs hooks into the user's home directory for all selected targets,
// registers the current working directory as a project dir for project-scoped
// skills, and persists the configuration.
func (m *Module) Init(ctx context.Context, opts InitOptions) (*InitResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := InstallHooks(opts.Targets, home, cwd); err != nil {
		return nil, fmt.Errorf("failed to install hooks: %w", err)
	}

	targetPaths := TargetPaths(opts.Targets, home, cwd)

	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}

	skillsCfg.Targets = mergeUnique(skillsCfg.Targets, targetPaths)
	skillsCfg.ProjectDirs = appendUnique(skillsCfg.ProjectDirs, cwd)

	if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
		return nil, fmt.Errorf("failed to save skills config: %w", err)
	}

	return &InitResult{InstalledTargets: targetPaths}, nil
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func mergeUnique(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, v := range existing {
		seen[v] = true
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, v := range additions {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// LoadSkillsOptions holds options for the load-skills operation.
type LoadSkillsOptions struct {
	SelectAll          bool
	SelectAllGroups    bool
	SelectAllUngrouped bool
	SelectedGroups     []string
	SelectedSkills     []string
}

// TargetResult holds the sync result for a single AI tool directory.
type TargetResult struct {
	Path       string
	SkillCount int
	IsProject  bool
	// GitHubCopilotRepo is true for a unified row under <repo>/.github/skills/port:
	// Port catalog "global" and "project" skills are both written there only, not
	// to a separate home-directory global path — avoid labeling as plain "global".
	GitHubCopilotRepo bool
}

// LoadSkillsResult summarises what was written.
type LoadSkillsResult struct {
	RequiredCount int
	SelectedCount int
	TargetCount   int
	TargetResults []TargetResult
}

// LoadSkills fetches skills from Port and writes them to the appropriate targets.
// Skills with location="project" are written to the current working directory;
// all other skills are written to the configured global AI tool directories.
func (m *Module) LoadSkills(ctx context.Context, opts LoadSkillsOptions) (*LoadSkillsResult, error) {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}

	if len(skillsCfg.Targets) == 0 {
		home, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()
		skillsCfg.Targets = TargetPaths(DefaultHookTargets(), home, cwd)
	}

	fetched, err := FetchSkills(ctx, m.client)
	if err != nil {
		return nil, err
	}

	if opts.SelectAll || opts.SelectAllGroups || opts.SelectAllUngrouped ||
		len(opts.SelectedGroups) > 0 || len(opts.SelectedSkills) > 0 {
		skillsCfg.SelectAll = opts.SelectAll
		skillsCfg.SelectAllGroups = opts.SelectAllGroups
		skillsCfg.SelectAllUngrouped = opts.SelectAllUngrouped
		skillsCfg.SelectedGroups = opts.SelectedGroups
		skillsCfg.SelectedSkills = opts.SelectedSkills
	}

	skills := FilterSkills(fetched, skillsCfg.SelectAll, skillsCfg.SelectAllGroups, skillsCfg.SelectAllUngrouped, skillsCfg.SelectedGroups, skillsCfg.SelectedSkills)

	if err := WriteSkills(skills, fetched.Groups, skillsCfg.Targets, skillsCfg.ProjectDirs); err != nil {
		return nil, fmt.Errorf("failed to write skills: %w", err)
	}

	skillsCfg.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
		return nil, fmt.Errorf("failed to save skills config: %w", err)
	}

	requiredCount := 0
	globalSkillCount := 0
	projectSkillCount := 0
	for _, s := range skills {
		if s.Required {
			requiredCount++
		}
		if s.Location == SkillLocationProject {
			projectSkillCount++
		} else {
			globalSkillCount++
		}
	}

	projectTargets := buildProjectTargets(skillsCfg.Targets, skillsCfg.ProjectDirs)

	targetResults := make([]TargetResult, 0, len(skillsCfg.Targets)+len(projectTargets))
	for _, t := range skillsCfg.Targets {
		if isGitHubCopilotSkillRoot(t) {
			continue
		}
		targetResults = append(targetResults, TargetResult{
			Path:       t,
			SkillCount: globalSkillCount,
			IsProject:  false,
		})
	}
	for _, t := range projectTargets {
		if isGitHubCopilotSkillRoot(t) {
			continue
		}
		targetResults = append(targetResults, TargetResult{
			Path:       t,
			SkillCount: projectSkillCount,
			IsProject:  true,
		})
	}
	copilotRoots := uniqCopilotSkillRoots(append(append([]string{}, skillsCfg.Targets...), projectTargets...))
	for _, root := range copilotRoots {
		targetResults = append(targetResults, TargetResult{
			Path:                root,
			SkillCount:          globalSkillCount + projectSkillCount,
			IsProject:           false,
			GitHubCopilotRepo:   true,
		})
	}

	return &LoadSkillsResult{
		RequiredCount: requiredCount,
		SelectedCount: len(skills) - requiredCount,
		TargetCount:   len(skillsCfg.Targets),
		TargetResults: targetResults,
	}, nil
}

// StatusResult contains the data surfaced by `port skills status`.
type StatusResult struct {
	Targets            []string
	ProjectDirs        []string
	SelectAll          bool
	SelectAllGroups    bool
	SelectAllUngrouped bool
	SelectedGroups     []string
	SelectedSkills     []string
	LastSyncedAt       string
}

// ClearSkillsResult summarises what was deleted.
type ClearSkillsResult struct {
	DeletedTargets []string
	SkippedTargets []string
}

// ClearSkills removes the Port skills directory ({target}/skills/port/) from
// every configured AI tool target and project directory. Targets where the
// directory does not exist are silently skipped.
func (m *Module) ClearSkills() (*ClearSkillsResult, error) {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}

	targets := skillsCfg.Targets
	if len(targets) == 0 {
		home, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()
		targets = TargetPaths(DefaultHookTargets(), home, cwd)
	}

	projectTargets := buildProjectTargets(targets, skillsCfg.ProjectDirs)

	allDirs := make([]string, 0, len(targets)+len(projectTargets))
	allDirs = append(allDirs, targets...)
	allDirs = append(allDirs, projectTargets...)

	result := &ClearSkillsResult{}
	for _, target := range allDirs {
		dir := filepath.Join(expandHome(target), "skills", PortSkillsDir)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			result.SkippedTargets = append(result.SkippedTargets, target)
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("failed to remove skills from %s: %w", target, err)
		}
		result.DeletedTargets = append(result.DeletedTargets, target)
	}

	return result, nil
}

// RemoveResult summarises what was removed by a full skills/hooks uninstall.
type RemoveResult struct {
	HooksResult  *RemoveHooksResult
	SkillsResult *ClearSkillsResult
}

// Remove uninstalls hooks, local synced skills, and clears skills config:
//   - Port hook entries from hooks.json / settings.json (other hooks preserved)
//   - Local skills directories (skills/port/)
//   - The skills section from ~/.port/config.yaml
func (m *Module) Remove() (*RemoveResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}

	hooksResult, err := RemoveHooks(DefaultHookTargets(), home, cwd, skillsCfg.Targets)
	if err != nil {
		return nil, fmt.Errorf("failed to remove hooks: %w", err)
	}

	skillsResult, err := m.ClearSkills()
	if err != nil {
		return nil, fmt.Errorf("failed to clear skills: %w", err)
	}

	if err := m.configManager.SaveSkillsConfig(&config.SkillsConfig{}); err != nil {
		return nil, fmt.Errorf("failed to clear skills config: %w", err)
	}

	return &RemoveResult{
		HooksResult:  hooksResult,
		SkillsResult: skillsResult,
	}, nil
}

// Status returns the current skills configuration state.
func (m *Module) Status() (*StatusResult, error) {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load skills config: %w", err)
	}

	return &StatusResult{
		Targets:            skillsCfg.Targets,
		ProjectDirs:        skillsCfg.ProjectDirs,
		SelectAll:          skillsCfg.SelectAll,
		SelectAllGroups:    skillsCfg.SelectAllGroups,
		SelectAllUngrouped: skillsCfg.SelectAllUngrouped,
		SelectedGroups:     skillsCfg.SelectedGroups,
		SelectedSkills:     skillsCfg.SelectedSkills,
		LastSyncedAt:       skillsCfg.LastSyncedAt,
	}, nil
}

// isGitHubCopilotSkillRoot reports whether absPath is the GitHub Copilot
// repository skill root (…/.github), i.e. where Port writes Copilot skills.
func isGitHubCopilotSkillRoot(absPath string) bool {
	exp := filepath.Clean(expandHome(absPath))
	for _, t := range DefaultHookTargets() {
		if t.Name != "GitHub Copilot" {
			continue
		}
		if matchesTarget(exp, t) {
			return true
		}
	}
	return false
}

// uniqCopilotSkillRoots returns deduplicated paths from candidates that are
// GitHub Copilot skill roots, sorted for stable output.
func uniqCopilotSkillRoots(candidates []string) []string {
	byCanon := make(map[string]string)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		exp := filepath.Clean(expandHome(p))
		if !isGitHubCopilotSkillRoot(exp) {
			continue
		}
		can := filepath.Clean(exp)
		if _, ok := byCanon[can]; !ok {
			byCanon[can] = p
		}
	}
	out := make([]string, 0, len(byCanon))
	for _, orig := range byCanon {
		out = append(out, orig)
	}
	sort.Strings(out)
	return out
}

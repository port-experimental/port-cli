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
	return m.fetchSkills(ctx, nil, nil)
}

// fetchSkills loads the sync catalog using saved config and optional per-call overrides.
func (m *Module) fetchSkills(ctx context.Context, cfg *config.SkillsConfig, opts *LoadSkillsOptions) (*FetchedSkills, error) {
	skillsCfg := cfg
	if skillsCfg == nil {
		var err error
		skillsCfg, err = m.configManager.LoadSkillsConfig()
		if err != nil {
			skillsCfg = &config.SkillsConfig{}
		}
	}
	return FetchSkillsFromAPI(ctx, m.client, buildFetchSkillsQuery(skillsCfg, opts))
}

// buildFetchSkillsQuery maps skills config and load options to GET /skills query params.
func buildFetchSkillsQuery(cfg *config.SkillsConfig, opts *LoadSkillsOptions) FetchSkillsQuery {
	query := FetchSkillsQuery{}
	if cfg == nil {
		cfg = &config.SkillsConfig{}
	}
	if cfg.UsesTeamGroupDefaults() {
		query.IncludeGroups = append([]string(nil), cfg.IncludeGroups...)
		query.ExcludeGroups = append([]string(nil), cfg.ExcludeGroups...)
		query.TeamsDefault = BoolPtr(true)
	} else if cfg.SelectAllGroups {
		// Include every group in the response so skills keep group folder layout on disk.
		query.TeamsDefault = BoolPtr(false)
	}
	if !cfg.SelectAll && !cfg.SelectAllUngrouped && len(cfg.SelectedSkills) > 0 {
		query.SkillIdentifiers = append([]string(nil), cfg.SelectedSkills...)
	}
	if cfg.SelectAll || cfg.SelectAllUngrouped {
		query.IncludeUngrouped = true
	}
	includeInternal := opts != nil && opts.IncludeInternalSkills
	if !includeInternal {
		query.Exclude = append(query.Exclude, "internal")
	}
	if opts != nil && opts.ExcludeLegacySkills {
		query.Exclude = append(query.Exclude, "legacy")
	}
	return query
}

// BoolPtr returns a bool pointer for optional skills API query flags.
func BoolPtr(v bool) *bool {
	return &v
}

// FetchSkillGroups loads skill group metadata for init selection.
func (m *Module) FetchSkillGroups(ctx context.Context) ([]api.SkillGroupCatalogEntry, error) {
	return FetchSkillGroupsFromAPI(ctx, m.client)
}

// FetchSkillsWithQuery loads the sync catalog using explicit skills API query parameters.
func (m *Module) FetchSkillsWithQuery(ctx context.Context, query FetchSkillsQuery) (*FetchedSkills, error) {
	return FetchSkillsFromAPI(ctx, m.client, query)
}

// InitOptions holds options for the init operation.
type InitOptions struct {
	Targets []HookTarget
}

// InitResult holds the result of an init operation.
type InitResult struct {
	InstalledTargets []string
}

// RegisterTargets saves hook target paths without installing hooks.
func (m *Module) RegisterTargets(ctx context.Context, targets []HookTarget) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}
	skillsCfg.Targets = replaceManagedTargets(skillsCfg.Targets, TargetPaths(targets, home, cwd), home, cwd)
	skillsCfg.ProjectDirs = appendUnique(skillsCfg.ProjectDirs, cwd)
	return m.configManager.SaveSkillsConfig(skillsCfg)
}

// ConfigureSelection persists the selected skill groups and ungrouped skills
// without downloading or writing skill files.
func (m *Module) ConfigureSelection(opts LoadSkillsOptions) error {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}
	applySelectionToConfig(skillsCfg, opts)
	if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
		return fmt.Errorf("failed to save skills config: %w", err)
	}
	return nil
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

	skillsCfg.Targets = replaceManagedTargets(skillsCfg.Targets, targetPaths, home, cwd)
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

// replaceManagedTargets reconciles saved target paths with a fresh selection.
// 'init' re-runs the full tool selection, so every target this CLI can resolve for the
// current scope (home dir + cwd) is replaced by selectedPaths — deselected tools are
// dropped. Saved paths that don't resolve to a known tool here, e.g. another
// repository's repo-scoped hook dir, are preserved so re-running init in one repo does
// not silently drop another repo's targets.
func replaceManagedTargets(saved, selectedPaths []string, home, cwd string) []string {
	managed := make(map[string]bool)
	for _, p := range TargetPaths(DefaultHookTargets(), home, cwd) {
		managed[p] = true
	}
	preserved := make([]string, 0, len(saved))
	for _, p := range saved {
		if !managed[p] {
			preserved = append(preserved, p)
		}
	}
	return mergeUnique(preserved, selectedPaths)
}

// AddSkillsOptions holds options for incrementally extending the saved selection.
type AddSkillsOptions struct {
	Groups  []string
	Skills  []string
	Targets []HookTarget
}

// AddSkillsResult summarises an add operation.
type AddSkillsResult struct {
	Merge       MergeSelectionResult
	Sync        *LoadSkillsResult
	NewTargets  []string
	InstalledOK bool
}

// AddSkills merges new groups/skills (and optionally new hook targets) into the
// saved configuration and syncs skills to disk.
func (m *Module) AddSkills(ctx context.Context, opts AddSkillsOptions) (*AddSkillsResult, error) {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		skillsCfg = &config.SkillsConfig{}
	}

	// 'add' is incremental and requires a prior 'init'. Check before mutating
	// state so a fresh-system invocation like `port skills add --tool Cursor`
	// errors out cleanly instead of installing hooks and then no-op-syncing.
	if !skillsCfg.HasSelection() && len(skillsCfg.Targets) == 0 {
		return nil, fmt.Errorf("no skills configuration found — run 'port skills init' first")
	}

	result := &AddSkillsResult{}

	if len(opts.Targets) > 0 {
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
		newPaths := TargetPaths(opts.Targets, home, cwd)
		skillsCfg.Targets = mergeUnique(skillsCfg.Targets, newPaths)
		skillsCfg.ProjectDirs = appendUnique(skillsCfg.ProjectDirs, cwd)
		result.NewTargets = newPaths
		result.InstalledOK = true
	}

	fetched, err := m.FetchSkills(ctx)
	if err != nil {
		return nil, err
	}

	mergeResult, err := MergeSelection(skillsCfg, fetched, opts.Groups, opts.Skills)
	if err != nil {
		return nil, err
	}
	result.Merge = mergeResult

	if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
		return nil, fmt.Errorf("failed to save skills config: %w", err)
	}

	if !mergeResult.HasChanges() && len(result.NewTargets) == 0 {
		return result, nil
	}

	syncResult, err := m.LoadSkills(ctx, LoadSkillsOptions{})
	if err != nil {
		return nil, err
	}
	result.Sync = syncResult
	return result, nil
}

// RemoveSkillsOptions holds options for removing items from the saved selection.
type RemoveSkillsOptions struct {
	Groups  []string
	Skills  []string
	Targets []HookTarget
}

// RemoveSkillsResult summarises a remove operation.
type RemoveSkillsResult struct {
	Remove         RemoveSelectionResult
	Sync           *LoadSkillsResult
	RemovedTargets []string
}

// RemoveSkills drops groups/skills and/or hook targets from the saved
// configuration. Targets have their hooks uninstalled and their synced
// skills/port/ directories deleted; remaining skills are re-synced so any
// pruned items are removed from disk on the remaining targets.
func (m *Module) RemoveSkills(ctx context.Context, opts RemoveSkillsOptions) (*RemoveSkillsResult, error) {
	skillsCfg, err := m.configManager.LoadSkillsConfig()
	if err != nil {
		return nil, fmt.Errorf("no skills configuration found — run 'port skills init' first")
	}
	if !skillsCfg.HasSelection() && len(skillsCfg.Targets) == 0 {
		return nil, fmt.Errorf("no skills configuration found — run 'port skills init' first")
	}

	result := &RemoveSkillsResult{}

	if len(opts.Targets) > 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}

		if _, err := RemoveHooks(opts.Targets, home, cwd, skillsCfg.Targets); err != nil {
			return nil, fmt.Errorf("failed to remove hooks: %w", err)
		}

		var pathsToRemove []string
		for _, savedPath := range skillsCfg.Targets {
			expanded := expandHome(savedPath)
			for _, t := range opts.Targets {
				if matchesTarget(expanded, t) {
					pathsToRemove = append(pathsToRemove, savedPath)
					break
				}
			}
		}
		for _, target := range pathsToRemove {
			skillDir := filepath.Join(expandHome(target), "skills", PortSkillsDir)
			if _, err := os.Stat(skillDir); err == nil {
				if err := os.RemoveAll(skillDir); err != nil {
					return nil, fmt.Errorf("failed to remove synced skills from %s: %w", target, err)
				}
			}
		}
		skillsCfg.Targets = subtractStrings(skillsCfg.Targets, pathsToRemove)
		result.RemovedTargets = pathsToRemove
	}

	fetched, err := m.FetchSkills(ctx)
	if err != nil {
		return nil, err
	}

	removeResult, err := RemoveSelection(skillsCfg, fetched, opts.Groups, opts.Skills)
	if err != nil {
		return nil, err
	}
	result.Remove = removeResult

	if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
		return nil, fmt.Errorf("failed to save skills config: %w", err)
	}

	if !removeResult.HasChanges() && len(result.RemovedTargets) == 0 {
		return result, nil
	}

	if len(skillsCfg.Targets) > 0 && skillsCfg.HasSelection() {
		syncResult, err := m.LoadSkills(ctx, LoadSkillsOptions{})
		if err != nil {
			return nil, err
		}
		result.Sync = syncResult
	}

	return result, nil
}

func subtractStrings(existing, remove []string) []string {
	rmSet := make(map[string]bool, len(remove))
	for _, p := range remove {
		rmSet[p] = true
	}
	out := make([]string, 0, len(existing))
	for _, p := range existing {
		if !rmSet[p] {
			out = append(out, p)
		}
	}
	return out
}

// LoadSkillsOptions holds options for the load-skills operation.
type LoadSkillsOptions struct {
	SelectAll          bool
	SelectAllGroups    bool
	SelectAllUngrouped bool
	SelectedGroups     []string
	SelectedSkills     []string
	IncludeGroups      []string
	ExcludeGroups      []string
	TeamGroupDefaults  bool
	// Fetched is an optional pre-fetched catalog. When set, LoadSkills skips the
	// FetchSkills API call and uses this data directly, avoiding duplicate
	// network requests when the caller already has the catalog in hand (e.g.,
	// the init command fetches once for prompts and reuses the same data for sync).
	Fetched *FetchedSkills
	// ReplaceSelection overwrites saved group/skill selection from opts instead of
	// only updating when opts carry selection fields (used by port skills select).
	ReplaceSelection bool
	// ExcludeLegacySkills omits legacy blueprint `skill` entities from the catalog fetch.
	ExcludeLegacySkills bool
	// IncludeInternalSkills includes Port built-in registry skills (excluded by default).
	IncludeInternalSkills bool
	// TargetOverrides writes to these target directories for this sync only.
	TargetOverrides []string
	// ProjectDirOverrides writes project-scoped skills under these project dirs
	// for this sync only.
	ProjectDirOverrides []string
	// NoSave prevents sync-only options from being written to config.yaml.
	NoSave bool
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
	SkillCount    int
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

	ApplySyncDefaults(skillsCfg)
	if opts.TargetOverrides != nil {
		skillsCfg.Targets = append([]string(nil), opts.TargetOverrides...)
	}
	if opts.ProjectDirOverrides != nil {
		skillsCfg.ProjectDirs = append([]string(nil), opts.ProjectDirOverrides...)
	}
	if len(skillsCfg.Targets) == 0 {
		return nil, fmt.Errorf("no skill targets configured; pass --tool or run 'port skills init' first")
	}
	applySelectionToConfig(skillsCfg, opts)

	fetched := opts.Fetched
	if fetched == nil {
		fetched, err = m.fetchSkills(ctx, skillsCfg, &opts)
		if err != nil {
			return nil, err
		}
	}

	skills := FilterSkills(
		fetched,
		skillsCfg.SelectAll,
		skillsCfg.SelectAllGroups,
		skillsCfg.SelectAllUngrouped,
		skillsCfg.SelectedGroups,
		skillsCfg.SelectedSkills,
		skillsCfg.UsesTeamGroupDefaults(),
	)

	globalTargets := skillsCfg.Targets
	projectDirs := skillsCfg.ProjectDirs

	if len(globalTargets) > 0 || len(projectDirs) > 0 {
		if err := WriteSkills(skills, fetched.Groups, globalTargets, projectDirs); err != nil {
			return nil, fmt.Errorf("failed to write skills: %w", err)
		}
	}

	if !opts.NoSave {
		skillsCfg.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
		if err := m.configManager.SaveSkillsConfig(skillsCfg); err != nil {
			return nil, fmt.Errorf("failed to save skills config: %w", err)
		}
	}

	globalSkillCount := 0
	projectSkillCount := 0
	for _, s := range skills {
		if s.Location == SkillLocationProject {
			projectSkillCount++
		} else {
			globalSkillCount++
		}
	}

	projectTargets := buildProjectTargets(globalTargets, projectDirs)

	targetResults := make([]TargetResult, 0, len(globalTargets)+len(projectTargets))
	for _, t := range globalTargets {
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
	copilotRoots := uniqCopilotSkillRoots(append(append([]string{}, globalTargets...), projectTargets...))
	for _, root := range copilotRoots {
		targetResults = append(targetResults, TargetResult{
			Path:              root,
			SkillCount:        globalSkillCount + projectSkillCount,
			IsProject:         false,
			GitHubCopilotRepo: true,
		})
	}

	return &LoadSkillsResult{
		SkillCount:    len(skills),
		TargetCount:   len(globalTargets),
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

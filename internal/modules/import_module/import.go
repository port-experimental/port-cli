package import_module

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
	systemblueprints "github.com/port-experimental/port-cli/internal/modules/system_blueprints"
	"github.com/port-experimental/port-cli/internal/plan"
)

// Module handles importing data to Port.
type Module struct {
	client *api.Client
}

// NewModule creates a new import module.
func NewModule(token *auth.Token, orgConfig *config.OrganizationConfig) *Module {
	client := api.NewClient(api.ClientOpts{
		Token:        token,
		ClientID:     orgConfig.ClientID,
		ClientSecret: orgConfig.ClientSecret,
		APIURL:       orgConfig.APIURL,
		Timeout:      0,
	})
	return &Module{
		client: client,
	}
}

// Options represents import options.
// ProgressCallback is called to report import progress.
// phase is the current phase name, current is the number of items processed, total is the total count.
type ProgressCallback func(phase string, current, total int)

// Options represents import options.
type Options struct {
	InputPath                     string
	DryRun                        bool
	SkipEntities                  bool
	SkipSystemBlueprints          bool // skip _* blueprint schemas and their entities
	SkipSystemBlueprintProperties bool
	IncludeRuleResults            bool // include _rule_result system blueprint entities (included by default)
	IncludeResources              []string
	ExcludeBlueprints             []string        // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema        []string        // shallow: exclude only the blueprint schema, keep resources
	UsersAsDisabled               bool            // import non-admin users as DISABLED after staging
	UserUpdateEmails              map[string]bool // emails to upsert directly (from diff); skips create-first
	Verbose                       bool
	ShowPagesPipeline             bool
	ProgressCallback              ProgressCallback
	LogCallback                   func(string)
	ErrorHandling                 ErrorHandlingOptions
}

// ValidationWarning represents a pre-import validation warning.
type ValidationWarning struct {
	Type    string // "cycle", "missing_dependency", "protected_resource", "orphaned_permission_field"
	Message string
	Details []string
}

// Result represents the result of an import operation.
type Result struct {
	Success                     bool
	Message                     string
	BlueprintsCreated           int
	BlueprintsUpdated           int
	EntitiesCreated             int
	EntitiesUpdated             int
	ScorecardsCreated           int
	ScorecardsUpdated           int
	ActionsCreated              int
	ActionsUpdated              int
	TeamsCreated                int
	TeamsUpdated                int
	UsersCreated                int
	UsersUpdated                int
	PagesCreated                int
	PagesUpdated                int
	IntegrationsUpdated         int
	BlueprintPermissionsUpdated int
	ActionPermissionsUpdated    int
	PagePermissionsUpdated      int
	Errors                      []string
	ErrorsByCategory            map[string][]string // Categorized errors for verbose output
	Warnings                    []ValidationWarning // Pre-import validation warnings
	DiffResult                  *DiffResult
	SidebarPipeline             []string
	// IgnoredRuleResultTargetRelationCount is how many _rule_result relations with type rule_result_target were omitted from API payloads.
	IgnoredRuleResultTargetRelationCount int
	// IgnoredRuleResultTargetRelationKeys lists relation identifiers omitted (sorted, unique).
	IgnoredRuleResultTargetRelationKeys []string
}

// Execute performs the import operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Load data
	loader := NewLoader()
	streamEntities := !opts.SkipEntities && shouldImport("entities", opts.IncludeResources)
	var data *export.Data
	var err error
	if streamEntities {
		data, err = NewStreamLoader().LoadDataWithoutEntities(opts.InputPath)
	} else {
		data, err = loader.LoadData(opts.InputPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	// Apply blueprint exclusions before diffing/importing
	applyDataExclusion(data, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema, opts.SkipSystemBlueprints, opts.SkipSystemBlueprintProperties)

	// Validate data
	if err := loader.ValidateData(data, opts.IncludeResources); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Diff validation (always enabled)
	comparer := NewDiffComparer(m.client)
	compareOpts := opts
	if streamEntities {
		compareOpts.SkipEntities = true
	}
	diffResult, err := comparer.Compare(ctx, data, compareOpts)
	if err != nil {
		return nil, fmt.Errorf("diff comparison failed: %w", err)
	}

	// Use diff result to filter data
	data = diffResult.FilterData(data)

	sidebarPipeline := PlanSidebarPipeline(data.Folders, data.Pages)

	// Dry run - show what would happen
	if opts.DryRun {
		result := m.generateDryRunResult(data, diffResult, opts)
		if streamEntities {
			importer := NewImporter(m.client)
			if opts.ProgressCallback != nil {
				importer.SetProgressCallback(opts.ProgressCallback)
			}
			if err := importer.ImportEntitiesFromStream(ctx, opts.InputPath, opts, result, true); err != nil {
				return nil, fmt.Errorf("streaming entity dry run failed: %w", err)
			}
		}
		result.SidebarPipeline = DescribeSidebarPipeline(sidebarPipeline)
		return result, nil
	}

	// Import data using new reliable importer
	importer := NewImporter(m.client)
	if len(sidebarPipeline) > 0 && opts.LogCallback != nil && opts.ShowPagesPipeline {
		opts.LogCallback("Proposed sidebar pipeline:")
		for _, line := range DescribeSidebarPipeline(sidebarPipeline) {
			opts.LogCallback(fmt.Sprintf("  %s", line))
		}
	}
	importOpts := opts
	if streamEntities {
		importOpts.SkipEntities = true
	}
	result, err := importer.Import(ctx, data, importOpts)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}
	if streamEntities {
		if err := importer.ImportEntitiesFromStream(ctx, opts.InputPath, opts, result, false); err != nil {
			return nil, fmt.Errorf("streaming entity import failed: %w", err)
		}
	}

	// Import permissions (blueprint and action permissions depend on resources existing)
	bpUpdated, actionUpdated, pageUpdated, permWarnings := importer.importPermissions(ctx, diffResult)

	// Surface permission sanitization warnings as validation warnings
	for _, w := range permWarnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:    "orphaned_permission_field",
			Message: w,
		})
	}

	// Merge any permission errors into result
	result.Errors = importer.errors.ToStringSlice()
	result.BlueprintPermissionsUpdated = bpUpdated
	result.ActionPermissionsUpdated = actionUpdated
	result.PagePermissionsUpdated = pageUpdated

	if len(result.Errors) > 0 {
		result.Success = false
		result.Message = fmt.Sprintf("Import completed with %d error(s)", len(result.Errors))
	} else {
		result.Success = true
		result.Message = "Successfully imported data"
	}
	result.DiffResult = diffResult
	result.SidebarPipeline = DescribeSidebarPipeline(sidebarPipeline)
	return result, nil
}

// generateDryRunResult generates a dry run result with accurate predictions.
func (m *Module) generateDryRunResult(data *export.Data, diffResult *DiffResult, _ Options) *Result {
	if diffResult != nil {
		counters := plan.ApplyCountersFromSummary(plan.Summarize(BuildFromDiffResult(diffResult)))
		result := &Result{
			Success:    true,
			Message:    "Validation passed (dry run - no changes applied)",
			DiffResult: diffResult,
		}
		populateImportResultCounters(result, counters)
		return result
	}

	return &Result{
		Success:           true,
		Message:           "Validation passed (dry run - no changes applied)",
		BlueprintsCreated: len(data.Blueprints),
		EntitiesCreated:   len(data.Entities),
	}
}

// Close closes the API client.
func (m *Module) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// shouldImport checks if a resource type should be imported.
func shouldImport(resourceType string, includeResources []string) bool {
	if len(includeResources) == 0 {
		return true
	}
	for _, r := range includeResources {
		if r == resourceType {
			return true
		}
	}
	return false
}

// cleanSystemFields removes system fields that shouldn't be sent to API.
func cleanSystemFields(resource map[string]interface{}, fieldsToRemove []string) map[string]interface{} {
	cleaned := make(map[string]interface{})
	removeSet := make(map[string]bool)
	for _, f := range fieldsToRemove {
		removeSet[f] = true
	}
	for k, v := range resource {
		if !removeSet[k] {
			cleaned[k] = v
		}
	}
	return cleaned
}

// isConflictError checks if an error is a conflict (409) error.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "409") || strings.Contains(errStr, "Conflict")
}

// IsConflictError checks if an error indicates that the resource already exists.
func IsConflictError(err error) bool {
	return isConflictError(err)
}

func (i *Importer) recordRuleResultIgnoredRelations(ignored []string, result *Result) {
	if len(ignored) == 0 || result == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.ruleResultIgnoreDedupe == nil {
		i.ruleResultIgnoreDedupe = make(map[string]struct{})
	}
	var logKeys []string
	for _, k := range ignored {
		if _, dup := i.ruleResultIgnoreDedupe[k]; dup {
			continue
		}
		i.ruleResultIgnoreDedupe[k] = struct{}{}
		result.IgnoredRuleResultTargetRelationCount++
		result.IgnoredRuleResultTargetRelationKeys = append(result.IgnoredRuleResultTargetRelationKeys, k)
		logKeys = append(logKeys, k)
	}
	if len(logKeys) > 0 && i.log != nil {
		i.log(fmt.Sprintf("Ignored %d _rule_result relation(s) (not sent to API): %s", len(logKeys), strings.Join(logKeys, ", ")))
	}
}

// Importer handles importing data to Port with proper dependency ordering.
type Importer struct {
	client                 *api.Client
	errors                 *ErrorCollector
	mu                     sync.Mutex
	log                    func(string)
	verbose                bool
	progress               ProgressCallback
	ruleResultIgnoreDedupe map[string]struct{}
}

// NewImporter creates a new importer.
func NewImporter(client *api.Client) *Importer {
	return &Importer{
		client: client,
		errors: NewErrorCollector(),
	}
}

// SetProgressCallback sets the progress callback for the importer.
func (i *Importer) SetProgressCallback(cb ProgressCallback) {
	i.progress = cb
}

// CollectedErrors returns all errors accumulated during the last operation.
func (i *Importer) CollectedErrors() []string {
	return i.errors.ToStringSlice()
}

func (i *Importer) SetLogCallback(cb func(string)) {
	i.log = cb
}

// reportProgress reports progress if a callback is set.
func (i *Importer) reportProgress(phase string, current, total int) {
	if i.progress != nil {
		i.progress(phase, current, total)
	}
}

// Import imports data to Port with proper dependency ordering.
func (i *Importer) Import(ctx context.Context, data *export.Data, opts Options) (*Result, error) {
	// Set progress callback if provided
	if opts.ProgressCallback != nil {
		i.progress = opts.ProgressCallback
	}
	i.verbose = opts.Verbose
	if opts.LogCallback != nil {
		i.log = opts.LogCallback
	}

	result := &Result{
		Errors:           []string{},
		ErrorsByCategory: make(map[string][]string),
		Warnings:         []ValidationWarning{},
	}
	i.ruleResultIgnoreDedupe = make(map[string]struct{})

	// Import blueprints with three-phase approach
	if shouldImport("blueprints", opts.IncludeResources) {
		if err := i.importBlueprints(ctx, data.Blueprints, result, opts.ErrorHandling); err != nil {
			return nil, err
		}
	}

	// Import other resources concurrently (but with bounded concurrency)
	if err := i.importOtherResources(ctx, data, opts, result); err != nil {
		return nil, err
	}

	// Convert collected errors to string slice for backward compatibility
	result.Errors = i.errors.ToStringSlice()

	// Populate errors by category for verbose output
	for _, category := range []ErrorCategory{
		ErrDependency, ErrAuth, ErrBlueprintConfig, ErrValidation,
		ErrSchemaMismatch, ErrRateLimit, ErrNetwork, ErrConflict,
		ErrNotFound, ErrUnknown,
	} {
		categoryErrors := i.errors.GetByCategory(category)
		if len(categoryErrors) > 0 {
			categoryStrings := make([]string, len(categoryErrors))
			for j, e := range categoryErrors {
				categoryStrings[j] = e.Error()
			}
			result.ErrorsByCategory[string(category)] = categoryStrings
		}
	}

	return result, nil
}

// ApplyFiltered imports pre-filtered export data (create/update items only) and
// applies permission updates from the diff. This is the shared apply path used by
// migrate after diffing and filtering; it mirrors Module.Execute's apply phase
// without loading or comparing data.
func (i *Importer) ApplyFiltered(ctx context.Context, data *export.Data, diff *DiffResult, opts Options) (*Result, error) {
	if opts.ProgressCallback != nil {
		i.progress = opts.ProgressCallback
	}
	i.verbose = opts.Verbose
	if opts.LogCallback != nil {
		i.log = opts.LogCallback
	}

	if diff != nil && opts.UserUpdateEmails == nil {
		opts.UserUpdateEmails = userUpdateEmailsFromDiff(diff)
	}

	result, err := i.Import(ctx, data, opts)
	if err != nil {
		return nil, err
	}

	bpUpdated, actionUpdated, pageUpdated, permWarnings := i.importPermissions(ctx, diff)
	for _, w := range permWarnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:    "orphaned_permission_field",
			Message: w,
		})
	}

	result.Errors = i.errors.ToStringSlice()
	result.BlueprintPermissionsUpdated = bpUpdated
	result.ActionPermissionsUpdated = actionUpdated
	result.PagePermissionsUpdated = pageUpdated

	return result, nil
}

// importOtherResources imports non-blueprint resources with bounded concurrency.
func (i *Importer) importOtherResources(ctx context.Context, data *export.Data, opts Options, result *Result) error {
	// Import entities
	if !opts.SkipEntities && shouldImport("entities", opts.IncludeResources) {
		if err := i.ImportEntities(ctx, data.Entities, opts.IncludeRuleResults, result); err != nil {
			return err
		}
	}

	// Import other resources concurrently with bounded concurrency
	pool := NewWorkerPool(DefaultConcurrency)

	// Import scorecards
	if shouldImport("scorecards", opts.IncludeResources) {
		i.importScorecards(ctx, data.Scorecards, result, pool)
	}

	// Import actions
	if shouldImport("actions", opts.IncludeResources) || shouldImport("automations", opts.IncludeResources) {
		i.importActions(ctx, data.Actions, result, pool)
	}

	// Import teams
	if !opts.SkipEntities && shouldImport("teams", opts.IncludeResources) {
		i.importTeams(ctx, data.Teams, result, pool)
	}

	// Import users
	if !opts.SkipEntities && shouldImport("users", opts.IncludeResources) {
		i.importUsers(ctx, data.Users, result, opts.UsersAsDisabled, opts.UserUpdateEmails)
	}

	// Import integrations
	if shouldImport("integrations", opts.IncludeResources) {
		i.importIntegrations(ctx, data.Integrations, result, pool)
	}

	pool.Wait()

	// Import pages level-by-level in topological `after` order.
	// Sidebar resources are executed through a shared pipeline so folders and pages
	// can depend on each other via `parent` and `after`.
	if shouldImport("pages", opts.IncludeResources) {
		i.importSidebarPipeline(ctx, PlanSidebarPipeline(data.Folders, data.Pages), result)
	}

	return nil
}

// Returns the counts of successfully updated permissions and any sanitization warnings.

// applyDataExclusion filters data in-place before diffing/importing.
// excludeDeep removes the blueprint schema AND all its entities/scorecards/actions.
// excludeSchema removes only the blueprint schema; resources for that blueprint are kept.
func applyDataExclusion(data *export.Data, excludeDeep, excludeSchema []string, skipSystemBlueprints bool, skipSystemBlueprintProperties bool) {
	// Pre-pass: remove system blueprint schemas and their entities (shallow skip).
	// Scorecards, actions, and permissions are kept.
	if skipSystemBlueprints {
		filteredBPs := data.Blueprints[:0:0]
		for _, bp := range data.Blueprints {
			id, _ := bp["identifier"].(string)
			if strings.HasPrefix(id, "_") {
				if !skipSystemBlueprintProperties {
					if patch := systemblueprints.CustomPatch(bp); patch != nil {
						filteredBPs = append(filteredBPs, patch)
					}
				}
				continue
			}
			filteredBPs = append(filteredBPs, bp)
		}
		data.Blueprints = filteredBPs

		filteredEnts := data.Entities[:0:0]
		for _, e := range data.Entities {
			bpID, _ := e["blueprint"].(string)
			if strings.HasPrefix(bpID, "_") {
				continue
			}
			filteredEnts = append(filteredEnts, e)
		}
		data.Entities = filteredEnts
	}

	if len(excludeDeep) == 0 && len(excludeSchema) == 0 {
		return
	}
	deepSet := make(map[string]bool, len(excludeDeep))
	for _, id := range excludeDeep {
		deepSet[id] = true
	}
	schemaSet := make(map[string]bool, len(excludeSchema))
	for _, id := range excludeSchema {
		schemaSet[id] = true
	}

	// Filter blueprints (both deep and schema-only remove the blueprint record)
	filtered := data.Blueprints[:0:0]
	for _, bp := range data.Blueprints {
		id, _ := bp["identifier"].(string)
		if deepSet[id] || schemaSet[id] {
			continue
		}
		filtered = append(filtered, bp)
	}
	data.Blueprints = filtered

	// Filter entities — only deep exclusion removes them
	filteredEntities := data.Entities[:0:0]
	for _, e := range data.Entities {
		bpID, _ := e["blueprint"].(string)
		if deepSet[bpID] {
			continue
		}
		filteredEntities = append(filteredEntities, e)
	}
	data.Entities = filteredEntities

	// Filter scorecards — only deep exclusion removes them
	filteredScorecards := data.Scorecards[:0:0]
	for _, sc := range data.Scorecards {
		bpID, _ := sc["blueprintIdentifier"].(string)
		if deepSet[bpID] {
			continue
		}
		filteredScorecards = append(filteredScorecards, sc)
	}
	data.Scorecards = filteredScorecards

	// Track action IDs for deep-excluded blueprints so we can clean up ActionPermissions
	excludedActionIDs := make(map[string]bool)
	for _, a := range data.Actions {
		bpID, _ := a["blueprint"].(string)
		if deepSet[bpID] {
			if actionID, _ := a["identifier"].(string); actionID != "" {
				excludedActionIDs[actionID] = true
			}
		}
	}

	// Filter actions — only deep exclusion removes them
	filteredActions := data.Actions[:0:0]
	for _, a := range data.Actions {
		bpID, _ := a["blueprint"].(string)
		if deepSet[bpID] {
			continue
		}
		filteredActions = append(filteredActions, a)
	}
	data.Actions = filteredActions

	// Clean up blueprint permissions for deep exclusions
	for id := range deepSet {
		delete(data.BlueprintPermissions, id)
	}

	// Clean up action permissions for deep exclusions
	if data.ActionPermissions != nil {
		for id := range excludedActionIDs {
			delete(data.ActionPermissions, id)
		}
	}
}

package import_module

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/entities"
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

// isProtectedBlueprint checks if a blueprint is protected (entities can't be created).
func isProtectedBlueprint(blueprintID string, includeRuleResults bool) bool {
	if strings.HasPrefix(blueprintID, "_rule") {
		return !includeRuleResults
	}
	return false
}

// blueprintRelatesToInheritedOwnership checks if a blueprint has ANY relation to a blueprint with inherited ownership.
// This is used to skip all entities from such blueprints, since Port will reject them.
func blueprintRelatesToInheritedOwnership(blueprintID string, inheritedOwnershipBPs map[string]bool, relationTargets map[string]map[string]string) bool {
	// Get the relation targets for this blueprint
	bpRelations, ok := relationTargets[blueprintID]
	if !ok {
		return false
	}

	// Check if any relation targets an inherited ownership blueprint
	for _, targetBP := range bpRelations {
		if inheritedOwnershipBPs[targetBP] {
			return true
		}
	}

	return false
}

// detectInheritedOwnershipBlueprints fetches blueprints and returns:
// 1. A set of blueprint IDs that have inherited ownership enabled
// 2. A map of blueprintID -> relationName -> targetBlueprintID for all blueprints
func (i *Importer) detectInheritedOwnershipBlueprints(ctx context.Context) (map[string]bool, map[string]map[string]string) {
	inheritedOwnership := make(map[string]bool)
	relationTargets := make(map[string]map[string]string)

	blueprints, err := i.client.GetBlueprints(ctx)
	if err != nil {
		// If we can't fetch blueprints, return empty maps and let errors occur naturally
		return inheritedOwnership, relationTargets
	}

	for _, bp := range blueprints {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
			continue
		}

		// Check for teamInheritance field with inheritOwnership property
		if teamInheritance, ok := bp["teamInheritance"].(map[string]interface{}); ok {
			if inheritOwnership, ok := teamInheritance["inheritOwnership"].(bool); ok && inheritOwnership {
				inheritedOwnership[id] = true
			}
		}

		// Also check the older/alternative field name
		if inheritOwnershipVal, ok := bp["inheritedOwnership"].(bool); ok && inheritOwnershipVal {
			inheritedOwnership[id] = true
		}

		// Extract relation targets for this blueprint
		if relations, ok := bp["relations"].(map[string]interface{}); ok {
			relationTargets[id] = make(map[string]string)
			for relName, relDef := range relations {
				if relMap, ok := relDef.(map[string]interface{}); ok {
					if target, ok := relMap["target"].(string); ok {
						relationTargets[id][relName] = target
					}
				}
			}
		}
	}

	return inheritedOwnership, relationTargets
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

// ImportEntities imports entities with two-phase bulk approach.
// Phase 1: bulk upsert all entities with relations stripped.
// Phase 2: bulk upsert entities that have relations (upsert=true, entities exist from Phase 1).
func (i *Importer) ImportEntities(ctx context.Context, entities []api.Entity, includeRuleResults bool, result *Result) error {
	if len(entities) == 0 {
		return nil
	}

	// Fetch blueprints to detect those with inherited ownership and build relation target map
	inheritedOwnershipBPs, relationTargets := i.detectInheritedOwnershipBlueprints(ctx)

	// Build set of blueprints that relate to inherited ownership blueprints
	blueprintsToSkip := make(map[string]bool)
	for bpID := range relationTargets {
		if blueprintRelatesToInheritedOwnership(bpID, inheritedOwnershipBPs, relationTargets) {
			blueprintsToSkip[bpID] = true
		}
	}

	// Filter out entities that:
	// 1. Belong to protected system blueprints
	// 2. Belong to blueprints with inherited ownership
	// 3. Belong to blueprints that have relations to inherited ownership blueprints
	filteredEntities := make([]api.Entity, 0, len(entities))
	protectedSkipped := 0
	inheritedOwnershipSkipped := 0
	for _, entity := range entities {
		blueprintID, _ := entity["blueprint"].(string)
		if isProtectedBlueprint(blueprintID, includeRuleResults) {
			protectedSkipped++
			continue
		}
		if inheritedOwnershipBPs[blueprintID] {
			inheritedOwnershipSkipped++
			continue
		}
		// Check if blueprint has relations to inherited ownership blueprints
		if blueprintsToSkip[blueprintID] {
			inheritedOwnershipSkipped++
			continue
		}
		filteredEntities = append(filteredEntities, entity)
	}

	skippedMsg := ""
	if protectedSkipped > 0 || inheritedOwnershipSkipped > 0 {
		parts := []string{}
		if protectedSkipped > 0 {
			parts = append(parts, fmt.Sprintf("%d protected", protectedSkipped))
		}
		if inheritedOwnershipSkipped > 0 {
			parts = append(parts, fmt.Sprintf("%d inherited-ownership", inheritedOwnershipSkipped))
		}
		skippedMsg = fmt.Sprintf(" (skipped %s)", strings.Join(parts, ", "))
	}

	total := len(filteredEntities)

	// Separate entities with and without relations
	entitiesWithRelations := make([]api.Entity, 0)
	for _, entity := range filteredEntities {
		if HasEntityRelations(entity) {
			entitiesWithRelations = append(entitiesWithRelations, entity)
		}
	}

	// Phase 1: Bulk upsert all entities with relations stripped
	i.reportProgress(fmt.Sprintf("Entities Phase 1%s", skippedMsg), 0, total)
	processedCount := 0
	var progressMu sync.Mutex
	successfulEntities := make(map[string]bool)
	var successMu sync.Mutex

	strippedEntities := make([]api.Entity, 0, len(filteredEntities))
	for _, entity := range filteredEntities {
		bp, ok1 := entity["blueprint"].(string)
		id, ok2 := entity["identifier"].(string)
		if !ok1 || !ok2 || bp == "" || id == "" {
			continue
		}
		strippedEntities = append(strippedEntities, StripEntityRelations(entity))
	}

	i.bulkUpsertEntities(ctx, strippedEntities, false, result, successfulEntities, &successMu, "Entities Phase 1", total, &processedCount, &progressMu)

	// Phase 2: Bulk update entities with relations (upsert=true — known to exist from Phase 1)
	if len(entitiesWithRelations) > 0 {
		phase2Total := len(entitiesWithRelations)
		i.reportProgress("Entities Phase 2 (relations)", 0, phase2Total)
		phase2Count := 0
		var phase2ProgressMu sync.Mutex

		// Filter to entities that succeeded in Phase 1
		successfulWithRelations := make([]api.Entity, 0, len(entitiesWithRelations))
		for _, entity := range entitiesWithRelations {
			blueprintID, _ := entity["blueprint"].(string)
			entityID, _ := entity["identifier"].(string)
			key := fmt.Sprintf("%s:%s", blueprintID, entityID)
			successMu.Lock()
			ok := successfulEntities[key]
			successMu.Unlock()
			if ok {
				successfulWithRelations = append(successfulWithRelations, entity)
			}
		}

		// Pass result=nil to skip double-counting (entities were counted in Phase 1)
		i.bulkUpsertEntities(ctx, successfulWithRelations, true, nil, nil, &successMu, "Entities Phase 2 (relations)", phase2Total, &phase2Count, &phase2ProgressMu)
	}

	return nil
}

// createOrUpdateEntity creates or updates a single entity.
func (i *Importer) createOrUpdateEntity(ctx context.Context, blueprintID, entityID string, entity api.Entity) (bool, bool, error) {
	_, err := i.client.CreateEntity(ctx, blueprintID, entity)
	if err == nil {
		return true, false, nil
	}

	if isConflictError(err) {
		_, updateErr := i.client.UpdateEntity(ctx, blueprintID, entityID, entity)
		if updateErr != nil {
			return false, false, updateErr
		}
		return false, true, nil
	}

	return false, false, err
}

// processBulkChunk sends one batch of entities for a single blueprint to the bulk endpoint.
// When upsert=false, 409 conflicts are collected and retried with upsert=true.
// result may be nil to skip created/updated counting (used in Phase 2 to avoid double-counting).
func (i *Importer) processBulkChunk(
	ctx context.Context,
	blueprintID string,
	chunk []api.Entity,
	upsert bool,
	result *Result,
	successfulEntities map[string]bool,
	successMu *sync.Mutex,
	phaseName string,
	total int,
	processedCount *int,
	progressMu *sync.Mutex,
) {
	chunkResult := entities.ProcessChunk(ctx, i.client, blueprintID, chunk, upsert)

	for _, chunkErr := range chunkResult.Errors {
		i.mu.Lock()
		i.errors.Add(fmt.Errorf("%s", chunkErr.Message), "entity", chunkErr.EntityID)
		i.mu.Unlock()
	}

	if result != nil {
		i.mu.Lock()
		result.EntitiesCreated += chunkResult.Created
		result.EntitiesUpdated += chunkResult.Updated
		i.mu.Unlock()
	}

	if successfulEntities != nil {
		successMu.Lock()
		for _, key := range chunkResult.SuccessfulKeys {
			successfulEntities[key] = true
		}
		successMu.Unlock()
	}

	progressMu.Lock()
	*processedCount += chunkResult.Processed
	cur := *processedCount
	progressMu.Unlock()

	if cur%100 == 0 || cur >= total {
		i.reportProgress(phaseName, cur, total)
	}
}

// bulkUpsertEntities sends entities to the bulk endpoint in batches, grouped by blueprint.
// result may be nil to skip counting (used in Phase 2).
func (i *Importer) bulkUpsertEntities(
	ctx context.Context,
	ents []api.Entity,
	upsert bool,
	result *Result,
	successfulEntities map[string]bool,
	successMu *sync.Mutex,
	phaseName string,
	total int,
	processedCount *int,
	progressMu *sync.Mutex,
) {
	byBlueprint := entities.GroupByBlueprint(ents)

	pool := NewWorkerPool(EntityConcurrency)

	for blueprint, bpEnts := range byBlueprint {
		bp := blueprint
		for _, chunk := range entities.ChunkSlice(bpEnts, entities.BatchSize) {
			chunk := chunk
			pool.Go(func() {
				i.processBulkChunk(ctx, bp, chunk, upsert, result, successfulEntities, successMu, phaseName, total, processedCount, progressMu)
			})
		}
	}

	pool.Wait()
}

// importScorecards imports scorecards grouped by blueprint.
func (i *Importer) importScorecards(ctx context.Context, scorecards []api.Scorecard, result *Result, pool *WorkerPool) {
	byBlueprint := make(map[string][]api.Scorecard)
	for _, sc := range scorecards {
		bpID, ok1 := sc["blueprintIdentifier"].(string)
		scID, ok2 := sc["identifier"].(string)
		if !ok1 || !ok2 || bpID == "" || scID == "" {
			i.errors.Add(fmt.Errorf("scorecard is missing identifier or blueprintIdentifier field, skipping"), "scorecard", "<unknown>")
			continue
		}
		cleaned := cleanSystemFields(sc, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "blueprint", "blueprintIdentifier"})
		byBlueprint[bpID] = append(byBlueprint[bpID], api.Scorecard(cleaned))
	}

	for bpID, scs := range byBlueprint {
		bpID := bpID
		scs := scs
		pool.Go(func() {
			var toMerge []api.Scorecard
			for _, sc := range scs {
				scID := sc["identifier"].(string)
				_, err := i.client.CreateScorecard(ctx, bpID, sc)

				i.mu.Lock()
				if err == nil {
					result.ScorecardsCreated++
				} else if isConflictError(err) {
					toMerge = append(toMerge, sc)
				} else {
					i.errors.Add(err, "scorecard", scID)
				}
				i.mu.Unlock()
			}

			// Port has no PATCH endpoint for individual scorecards, so we
			// fetch the full set, merge in our updates, and bulk PUT.
			if len(toMerge) > 0 {
				existing, fetchErr := i.client.GetScorecards(ctx, bpID)
				if fetchErr != nil {
					i.mu.Lock()
					i.errors.Add(fetchErr, "scorecard", fmt.Sprintf("fetch:%s", bpID))
					i.mu.Unlock()
					return
				}

				mergeSet := make(map[string]api.Scorecard, len(toMerge))
				for _, sc := range toMerge {
					mergeSet[sc["identifier"].(string)] = sc
				}

				merged := make([]api.Scorecard, 0, len(existing))
				for _, ex := range existing {
					exID, _ := ex["identifier"].(string)
					cleaned := cleanSystemFields(ex, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "blueprint", "blueprintIdentifier"})
					if replacement, ok := mergeSet[exID]; ok {
						merged = append(merged, replacement)
						delete(mergeSet, exID)
					} else {
						merged = append(merged, api.Scorecard(cleaned))
					}
				}
				for _, sc := range mergeSet {
					merged = append(merged, sc)
				}

				_, putErr := i.client.UpdateScorecards(ctx, bpID, merged)
				i.mu.Lock()
				if putErr != nil {
					i.errors.Add(putErr, "scorecard", fmt.Sprintf("bulk-put:%s", bpID))
				} else {
					result.ScorecardsUpdated += len(toMerge)
				}
				i.mu.Unlock()
			}
		})
	}
}

// importActions imports actions/automations.
func (i *Importer) importActions(ctx context.Context, actions []api.Action, result *Result, pool *WorkerPool) {
	for _, action := range actions {
		action := action
		pool.Go(func() {
			actionID, ok := action["identifier"].(string)
			if !ok || actionID == "" {
				return
			}

			cleaned := cleanSystemFields(action, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
			apiAction := api.Automation(cleaned)

			_, err := i.client.CreateAutomation(ctx, apiAction)

			i.mu.Lock()
			if err == nil {
				result.ActionsCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateAutomation(ctx, actionID, apiAction)
				if updateErr != nil {
					i.errors.Add(updateErr, "action", actionID)
				} else {
					result.ActionsUpdated++
				}
			} else {
				i.errors.Add(err, "action", actionID)
			}
			i.mu.Unlock()
		})
	}
}

// sanitizeTeamFields removes nil-valued fields from a team map before sending
// to the API. Some fields (e.g. description) exported as null from the source
// org cause invalid_request errors on upsert even though the API stores null
// internally. Omitting the field avoids the validation error.
func sanitizeTeamFields(team api.Team) api.Team {
	result := make(api.Team, len(team))
	for k, v := range team {
		if v != nil {
			result[k] = v
		}
	}
	return result
}

// importTeams imports teams.
func (i *Importer) importTeams(ctx context.Context, teams []api.Team, result *Result, pool *WorkerPool) {
	for _, team := range teams {
		team := team
		pool.Go(func() {
			teamName, ok := team["name"].(string)
			if !ok || teamName == "" {
				return
			}

			sanitized := sanitizeTeamFields(team)
			_, err := i.client.CreateTeam(ctx, sanitized)

			i.mu.Lock()
			if err == nil {
				result.TeamsCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateTeam(ctx, teamName, sanitized)
				if updateErr != nil {
					i.errors.Add(updateErr, "team", teamName)
				} else {
					result.TeamsUpdated++
				}
			} else {
				i.errors.Add(err, "team", teamName)
			}
			i.mu.Unlock()
		})
	}
}

// UserBatchSize is the maximum number of _user entities per bulk API call.
const UserBatchSize = 20

// UserStatusForCreate returns the status to set when creating a new user entity.
func UserStatusForCreate(user api.User, usersAsDisabled bool) string {
	if usersAsDisabled {
		userType, _ := user["type"].(string)
		if userType != "ADMIN" {
			return "DISABLED"
		}
	}
	return "STAGED"
}

// UserToEntity converts a User API response to a _user blueprint entity payload.
// Pass statusOverride="" to keep the source status (used for updates).
func UserToEntity(user api.User, statusOverride string) api.Entity {
	email, _ := user["email"].(string)
	firstName, _ := user["firstName"].(string)
	lastName, _ := user["lastName"].(string)

	systemFields := map[string]bool{
		"id": true, "createdAt": true, "updatedAt": true,
		"createdBy": true, "updatedBy": true,
	}
	props := make(map[string]interface{})
	for k, v := range user {
		if !systemFields[k] {
			props[k] = v
		}
	}
	if statusOverride != "" {
		props["status"] = statusOverride
	}

	title := strings.TrimSpace(firstName + " " + lastName)
	if title == "" {
		title = email
	}
	return api.Entity{
		"identifier": email,
		"title":      title,
		"properties": props,
	}
}

// importUsers imports users as _user blueprint entities.
// New users are created with STAGED status (or DISABLED for non-admins when usersAsDisabled is true).
// Existing users are updated with source data as-is.
func userUpdateEmailsFromDiff(diff *DiffResult) map[string]bool {
	if diff == nil || len(diff.UsersToUpdate) == 0 {
		return nil
	}
	emails := make(map[string]bool, len(diff.UsersToUpdate))
	for _, u := range diff.UsersToUpdate {
		if email, ok := u["email"].(string); ok && email != "" {
			emails[email] = true
		}
	}
	return emails
}

func (i *Importer) importUsers(ctx context.Context, users []api.User, result *Result, usersAsDisabled bool, userUpdateEmails map[string]bool) {
	var toUpdate []api.User
	if len(userUpdateEmails) > 0 {
		var toCreate []api.User
		for _, u := range users {
			email, ok := u["email"].(string)
			if !ok || email == "" {
				continue
			}
			if userUpdateEmails[email] {
				toUpdate = append(toUpdate, u)
			} else {
				toCreate = append(toCreate, u)
			}
		}
		users = toCreate
	}

	// Index by email for conflict resolution
	byEmail := make(map[string]api.User, len(users))
	for _, u := range users {
		if email, ok := u["email"].(string); ok && email != "" {
			byEmail[email] = u
		}
	}

	for start := 0; start < len(users); start += UserBatchSize {
		end := start + UserBatchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[start:end]

		entities := make([]api.Entity, 0, len(batch))
		for _, u := range batch {
			if email, ok := u["email"].(string); !ok || email == "" {
				continue
			}
			status := UserStatusForCreate(u, usersAsDisabled)
			entities = append(entities, UserToEntity(u, status))
		}
		if len(entities) == 0 {
			continue
		}

		errs, err := i.client.CreateUserEntitiesBulk(ctx, entities, false)
		if err != nil {
			i.mu.Lock()
			for _, e := range entities {
				if email, ok := e["identifier"].(string); ok {
					i.errors.Add(err, "user", email)
				}
			}
			i.mu.Unlock()
			continue
		}

		i.mu.Lock()
		result.UsersCreated += len(entities) - len(errs)
		i.mu.Unlock()

		// Collect conflicting users and re-POST with upsert=true, source data as-is
		var conflictEntities []api.Entity
		var nonConflictErrs []api.BulkEntityError
		for _, be := range errs {
			if int(be.StatusCode) == 409 {
				if orig, ok := byEmail[be.Identifier]; ok {
					conflictEntities = append(conflictEntities, UserToEntity(orig, ""))
				}
			} else {
				nonConflictErrs = append(nonConflictErrs, be)
			}
		}

		for _, be := range nonConflictErrs {
			i.mu.Lock()
			i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
			i.mu.Unlock()
		}

		if len(conflictEntities) > 0 {
			updateErrs, updateErr := i.client.CreateUserEntitiesBulk(ctx, conflictEntities, true)
			if updateErr != nil {
				i.mu.Lock()
				for _, e := range conflictEntities {
					if email, ok := e["identifier"].(string); ok {
						i.errors.Add(updateErr, "user", email)
					}
				}
				i.mu.Unlock()
			} else {
				i.mu.Lock()
				result.UsersUpdated += len(conflictEntities) - len(updateErrs)
				i.mu.Unlock()
				for _, be := range updateErrs {
					i.mu.Lock()
					i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
					i.mu.Unlock()
				}
			}
		}
	}
	i.importUserUpdates(ctx, toUpdate, result)
}

func (i *Importer) importUserUpdates(ctx context.Context, users []api.User, result *Result) {
	for start := 0; start < len(users); start += UserBatchSize {
		end := start + UserBatchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[start:end]

		entities := make([]api.Entity, 0, len(batch))
		for _, u := range batch {
			if email, ok := u["email"].(string); !ok || email == "" {
				continue
			}
			entities = append(entities, UserToEntity(u, ""))
		}
		if len(entities) == 0 {
			continue
		}

		updateErrs, err := i.client.CreateUserEntitiesBulk(ctx, entities, true)
		i.mu.Lock()
		if err != nil {
			for _, e := range entities {
				if email, ok := e["identifier"].(string); ok {
					i.errors.Add(err, "user", email)
				}
			}
		} else {
			result.UsersUpdated += len(entities) - len(updateErrs)
			for _, be := range updateErrs {
				i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
			}
		}
		i.mu.Unlock()
	}
}

// isInvalidPermissionsError returns true when the Port API rejects a permissions
// PATCH because the payload references relations or properties that don't exist
// on the target blueprint (e.g., orphaned scorecard relations on _rule_result).
func isInvalidPermissionsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "invalid_permissions")
}

// IsInvalidPermissionsError is the exported form for use by the migrate package.
func IsInvalidPermissionsError(err error) bool {
	return isInvalidPermissionsError(err)
}

// invalidPermBodyPattern extracts the JSON body from the API error string.
// The error format is: "API request to ... failed: 422 ... Body: {JSON}"
var invalidPermBodyPattern = regexp.MustCompile(`(?s)Body: (\{.*\})`)

// ParseInvalidPermissionFields extracts the invalidRelations and
// invalidProperties arrays from an invalid_permissions API error.
// Returns nil slices when the error is not parseable or not an
// invalid_permissions error.
func ParseInvalidPermissionFields(err error) (relations, properties []string) {
	if err == nil {
		return nil, nil
	}
	matches := invalidPermBodyPattern.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return nil, nil
	}
	var body struct {
		Error   string `json:"error"`
		Details struct {
			InvalidRelations  []string `json:"invalidRelations"`
			InvalidProperties []string `json:"invalidProperties"`
		} `json:"details"`
	}
	if json.Unmarshal([]byte(matches[1]), &body) != nil {
		return nil, nil
	}
	if body.Error != "invalid_permissions" {
		return nil, nil
	}
	return body.Details.InvalidRelations, body.Details.InvalidProperties
}

// SanitizePermissions returns a deep copy of perms with the named relation
// and property keys removed. Invalid relations are stripped from top-level
// keys and from entities.updateRelations; invalid properties are stripped
// from top-level keys and from entities.updateProperties.
func SanitizePermissions(perms api.Permissions, invalidRelations, invalidProperties []string) api.Permissions {
	relSet := make(map[string]bool, len(invalidRelations))
	for _, r := range invalidRelations {
		relSet[r] = true
	}
	propSet := make(map[string]bool, len(invalidProperties))
	for _, p := range invalidProperties {
		propSet[p] = true
	}

	// Deep-copy and strip top-level keys
	cleaned := make(api.Permissions, len(perms))
	for k, v := range perms {
		if relSet[k] || propSet[k] {
			continue
		}
		cleaned[k] = v
	}

	// Strip nested relation/property keys inside entities.updateRelations
	// and entities.updateProperties where the API actually validates them.
	entities, ok := cleaned["entities"].(map[string]interface{})
	if !ok {
		return cleaned
	}
	entitiesCopy := make(map[string]interface{}, len(entities))
	for k, v := range entities {
		entitiesCopy[k] = v
	}

	if ur, ok := entitiesCopy["updateRelations"].(map[string]interface{}); ok && len(relSet) > 0 {
		urCopy := make(map[string]interface{}, len(ur))
		for k, v := range ur {
			if !relSet[k] {
				urCopy[k] = v
			}
		}
		entitiesCopy["updateRelations"] = urCopy
	}

	if up, ok := entitiesCopy["updateProperties"].(map[string]interface{}); ok && len(propSet) > 0 {
		upCopy := make(map[string]interface{}, len(up))
		for k, v := range up {
			if !propSet[k] {
				upCopy[k] = v
			}
		}
		entitiesCopy["updateProperties"] = upCopy
	}

	cleaned["entities"] = entitiesCopy
	return cleaned
}

// actionAuditFields are the audit/internal fields that must be stripped before
// sending an action or automation to the Port API.
var actionAuditFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}

// CleanActionForCreate returns a copy of the action with audit fields removed.
func CleanActionForCreate(action api.Action) api.Action {
	return api.Action(cleanSystemFields(map[string]interface{}(action), actionAuditFields))
}

// importIntegrations imports integrations (update config only).
func (i *Importer) importIntegrations(ctx context.Context, integrations []api.Integration, result *Result, pool *WorkerPool) {
	for _, integration := range integrations {
		integration := integration
		pool.Go(func() {
			integrationID, ok := integration["identifier"].(string)
			if !ok || integrationID == "" {
				i.errors.Add(fmt.Errorf("integration is missing identifier field, skipping"), "integration", "<unknown>")
				return
			}

			// The integration config endpoint expects {"config": {...}} wrapper
			config, ok := integration["config"].(map[string]interface{})
			if !ok || config == nil {
				// No config to update — report so the user knows this integration was skipped
				i.errors.Add(fmt.Errorf("integration has no config field to update, skipping"), "integration", integrationID)
				return
			}

			// Wrap the config in the expected format
			payload := map[string]interface{}{
				"config": config,
			}

			_, err := i.client.UpdateIntegrationConfig(ctx, integrationID, payload)

			i.mu.Lock()
			if err != nil {
				i.errors.Add(err, "integration", integrationID)
			} else {
				result.IntegrationsUpdated++
			}
			i.mu.Unlock()
		})
	}
}

// importPermissions applies blueprint and action permission changes from a DiffResult.
// Permissions are applied after all other resources have been imported so that the
// underlying blueprints, actions, and pages are guaranteed to exist.
// When the API rejects a payload due to orphaned relations or properties (422
// invalid_permissions), the offending keys are stripped and the request is retried.
// Returns the counts of successfully updated permissions and any sanitization warnings.
func (i *Importer) importPermissions(ctx context.Context, diff *DiffResult) (bpUpdated, actionUpdated, pageUpdated int, warnings []string) {
	if diff == nil {
		return
	}

	// Import blueprint permissions
	for _, change := range diff.BlueprintPermissions {
		perms := change.Permissions
		_, err := i.client.UpdateBlueprintPermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdateBlueprintPermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update blueprint permissions for %s: %w", change.Identifier, err), "blueprint_permissions", change.Identifier)
		} else {
			bpUpdated++
		}
	}

	// Import action permissions
	for _, change := range diff.ActionPermissions {
		perms := change.Permissions
		_, err := i.client.UpdateActionPermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s action permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdateActionPermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update action permissions for %s: %w", change.Identifier, err), "action_permissions", change.Identifier)
		} else {
			actionUpdated++
		}
	}

	// Import page permissions
	for _, change := range diff.PagePermissions {
		perms := change.Permissions
		_, err := i.client.UpdatePagePermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s page permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdatePagePermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update page permissions for %s: %w", change.Identifier, err), "page_permissions", change.Identifier)
		} else {
			pageUpdated++
		}
	}
	return
}

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

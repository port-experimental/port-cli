package import_module

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
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
	InputPath              string
	DryRun                 bool
	SkipEntities           bool
	SkipSystemBlueprints   bool // skip _* blueprint schemas and their entities
	IncludeResources       []string
	ExcludeBlueprints      []string // deep: exclude blueprint schema + all its resources
	ExcludeBlueprintSchema []string // shallow: exclude only the blueprint schema, keep resources
	Verbose                bool
	ShowPagesPipeline      bool
	ProgressCallback       ProgressCallback
	LogCallback            func(string)
}

// ValidationWarning represents a pre-import validation warning.
type ValidationWarning struct {
	Type    string // "cycle", "missing_dependency", "protected_resource"
	Message string
	Details []string
}

// Result represents the result of an import operation.
type Result struct {
	Success             bool
	Message             string
	BlueprintsCreated   int
	BlueprintsUpdated   int
	EntitiesCreated     int
	EntitiesUpdated     int
	ScorecardsCreated   int
	ScorecardsUpdated   int
	ActionsCreated      int
	ActionsUpdated      int
	TeamsCreated        int
	TeamsUpdated        int
	UsersCreated        int
	UsersUpdated        int
	PagesCreated        int
	PagesUpdated        int
	IntegrationsUpdated int
	Errors              []string
	ErrorsByCategory    map[string][]string // Categorized errors for verbose output
	Warnings            []ValidationWarning // Pre-import validation warnings
	DiffResult          *DiffResult
	SidebarPipeline     []string
}

type SidebarPipelineOperation struct {
	ResourceType string
	Identifier   string
	Folder       api.Folder
	Page         api.Page
}

type SidebarPipelineStep struct {
	Operations []SidebarPipelineOperation
}

// Execute performs the import operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Load data
	loader := NewLoader()
	data, err := loader.LoadData(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	// Apply blueprint exclusions before diffing/importing
	applyDataExclusion(data, opts.ExcludeBlueprints, opts.ExcludeBlueprintSchema, opts.SkipSystemBlueprints)

	// Validate data
	if err := loader.ValidateData(data); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Diff validation (always enabled)
	comparer := NewDiffComparer(m.client)
	diffResult, err := comparer.Compare(ctx, data, opts)
	if err != nil {
		return nil, fmt.Errorf("diff comparison failed: %w", err)
	}

	// Use diff result to filter data
	data = diffResult.FilterData(data)

	sidebarPipeline := PlanSidebarPipeline(data.Folders, data.Pages)

	// Dry run - show what would happen
	if opts.DryRun {
		result := m.generateDryRunResult(data, diffResult, opts)
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
	result, err := importer.Import(ctx, data, opts)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	// Import permissions (blueprint and action permissions depend on resources existing)
	importer.importPermissions(ctx, diffResult)

	// Merge any permission errors into result
	result.Errors = importer.errors.ToStringSlice()

	result.Success = true
	result.Message = "Successfully imported data"
	result.DiffResult = diffResult
	result.SidebarPipeline = DescribeSidebarPipeline(sidebarPipeline)
	return result, nil
}

// generateDryRunResult generates a dry run result with accurate predictions.
func (m *Module) generateDryRunResult(data *export.Data, diffResult *DiffResult, _ Options) *Result {
	if diffResult != nil {
		return &Result{
			Success:             true,
			Message:             "Validation passed (dry run - no changes applied)",
			BlueprintsCreated:   len(diffResult.BlueprintsToCreate),
			BlueprintsUpdated:   len(diffResult.BlueprintsToUpdate),
			EntitiesCreated:     len(diffResult.EntitiesToCreate),
			EntitiesUpdated:     len(diffResult.EntitiesToUpdate),
			ScorecardsCreated:   len(diffResult.ScorecardsToCreate),
			ScorecardsUpdated:   len(diffResult.ScorecardsToUpdate),
			ActionsCreated:      len(diffResult.ActionsToCreate),
			ActionsUpdated:      len(diffResult.ActionsToUpdate),
			TeamsCreated:        len(diffResult.TeamsToCreate),
			TeamsUpdated:        len(diffResult.TeamsToUpdate),
			UsersCreated:        len(diffResult.UsersToCreate),
			UsersUpdated:        len(diffResult.UsersToUpdate),
			PagesCreated:        len(diffResult.PagesToCreate),
			PagesUpdated:        len(diffResult.PagesToUpdate),
			IntegrationsUpdated: len(diffResult.IntegrationsToUpdate),
			DiffResult:          diffResult,
		}
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

// protectedBlueprints are system blueprints that don't allow entity creation via API.
var protectedBlueprints = map[string]bool{
	"_rule_result": true,
}

// isProtectedBlueprint checks if a blueprint is protected (entities can't be created).
func isProtectedBlueprint(blueprintID string) bool {
	// Check explicit list
	if protectedBlueprints[blueprintID] {
		return true
	}
	// Also skip any blueprint starting with underscore followed by specific patterns
	// that are known to be system-managed
	if strings.HasPrefix(blueprintID, "_rule") {
		return true
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
	client   *api.Client
	errors   *ErrorCollector
	mu       sync.Mutex
	log      func(string)
	verbose  bool
	progress ProgressCallback
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

	// Import blueprints with three-phase approach
	if shouldImport("blueprints", opts.IncludeResources) {
		if err := i.importBlueprints(ctx, data.Blueprints, result); err != nil {
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

// importBlueprints imports blueprints using a multi-phase approach:
// Phase 1: Create non-system blueprints with relations and dependent fields stripped
// Phase 2a: Add relations back to all blueprints
// Phase 2b: Add calculationProperties (self-contained, no cross-blueprint dependencies)
// Phase 2c: Add mirrorProperties (depend on relations existing)
// Phase 2d: Add aggregationProperties (depend on properties existing on OTHER blueprints)
// Phase 3: Update system blueprints
func (i *Importer) importBlueprints(ctx context.Context, blueprints []api.Blueprint, result *Result) error {
	// Separate system and non-system blueprints
	nonSystemBPs, systemBPs := SeparateSystemBlueprints(blueprints)

	// Build existing blueprints set (system blueprints are assumed to exist)
	existingBPs := make(map[string]bool)
	for _, bp := range systemBPs {
		if id, ok := bp["identifier"].(string); ok {
			existingBPs[id] = true
		}
	}
	// Also add common system blueprints that might not be in export
	for _, id := range CommonSystemBlueprints() {
		existingBPs[id] = true
	}

	// Store each field type separately for ordered updates in Phase 2
	storedRelations := make(map[string]map[string]interface{})
	storedCalcProps := make(map[string]map[string]interface{})
	storedMirrorProps := make(map[string]map[string]interface{})
	storedAggProps := make(map[string]map[string]interface{})
	storedOwnership := make(map[string]map[string]interface{})
	strippedBPs := make([]api.Blueprint, 0, len(nonSystemBPs))

	for _, bp := range nonSystemBPs {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
			i.errors.Add(fmt.Errorf("blueprint is missing identifier field, skipping"), "blueprint", "<unknown>")
			continue
		}

		// Extract and store relations
		if relations, ok := bp["relations"].(map[string]interface{}); ok && len(relations) > 0 {
			storedRelations[id] = relations
		}

		// Extract and store each dependent field type separately
		if calcProps, ok := bp["calculationProperties"].(map[string]interface{}); ok && len(calcProps) > 0 {
			storedCalcProps[id] = calcProps
		}
		if mirrorProps, ok := bp["mirrorProperties"].(map[string]interface{}); ok && len(mirrorProps) > 0 {
			storedMirrorProps[id] = mirrorProps
		}
		if aggProps, ok := bp["aggregationProperties"].(map[string]interface{}); ok && len(aggProps) > 0 {
			storedAggProps[id] = aggProps
		}
		if ownership, ok := bp["ownership"].(map[string]interface{}); ok && len(ownership) > 0 {
			storedOwnership[id] = ownership
		}

		// Strip both relations AND dependent fields for phase 1
		stripped := StripDependentFields(bp)
		stripped = StripRelations(stripped)
		strippedBPs = append(strippedBPs, stripped)
	}

	// Topological sort
	levels, cyclic := TopologicalSort(strippedBPs, existingBPs)

	// Add warning about cyclic blueprints
	if len(cyclic) > 0 {
		cyclicIDs := make([]string, 0, len(cyclic))
		for _, bp := range cyclic {
			if id, ok := bp["identifier"].(string); ok {
				cyclicIDs = append(cyclicIDs, id)
			}
		}
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:    "cycle",
			Message: fmt.Sprintf("Detected %d blueprints with circular dependencies", len(cyclic)),
			Details: cyclicIDs,
		})
	}

	// Track successfully created blueprints
	successfulBPs := make(map[string]bool)
	for id := range existingBPs {
		successfulBPs[id] = true
	}

	// Phase 1: Create non-system blueprints in dependency order
	pool := NewWorkerPool(BlueprintConcurrency)
	totalBPs := len(FlattenLevels(levels)) + len(cyclic)
	createdCount := 0

	for levelIdx, level := range levels {
		i.reportProgress(fmt.Sprintf("Blueprints (level %d/%d)", levelIdx+1, len(levels)), createdCount, totalBPs)

		var levelMu sync.Mutex
		for _, bp := range level {
			bp := bp
			pool.Go(func() {
				id := bp["identifier"].(string)
				created, updated, err := i.createOrUpdateBlueprint(ctx, bp)

				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				} else {
					if created {
						result.BlueprintsCreated++
					} else if updated {
						result.BlueprintsUpdated++
					}
					levelMu.Lock()
					successfulBPs[id] = true
					levelMu.Unlock()
				}
				createdCount++
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Handle cyclic blueprints (best effort)
	if len(cyclic) > 0 {
		i.reportProgress("Blueprints (cyclic)", createdCount, totalBPs)
		for _, bp := range cyclic {
			bp := bp
			pool.Go(func() {
				id := bp["identifier"].(string)
				created, updated, err := i.createOrUpdateBlueprint(ctx, bp)

				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				} else {
					if created {
						result.BlueprintsCreated++
					} else if updated {
						result.BlueprintsUpdated++
					}
					successfulBPs[id] = true
				}
				createdCount++
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Fetch ALL existing blueprints from target for validation
	allExistingBPs := make(map[string]bool)
	for id := range successfulBPs {
		allExistingBPs[id] = true
	}
	targetBlueprints, err := i.client.GetBlueprints(ctx)
	if err == nil {
		for _, bp := range targetBlueprints {
			if id, ok := bp["identifier"].(string); ok && id != "" {
				allExistingBPs[id] = true
			}
		}
	}

	// Phase 2a: Add relations back to all blueprints
	if len(storedRelations) > 0 {
		i.reportProgress("Blueprints (adding relations)", 0, len(storedRelations))
		count := 0
		for id, relations := range storedRelations {
			if !allExistingBPs[id] {
				continue
			}
			id, relations := id, relations
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"relations": relations})
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding relations)", count, len(storedRelations))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Phase 2b: Add calculationProperties (self-contained, no cross-blueprint deps)
	if len(storedCalcProps) > 0 {
		i.reportProgress("Blueprints (adding calculationProperties)", 0, len(storedCalcProps))
		count := 0
		for id, calcProps := range storedCalcProps {
			if !allExistingBPs[id] {
				continue
			}
			id, calcProps := id, calcProps
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"calculationProperties": calcProps})
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding calculationProperties)", count, len(storedCalcProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Phase 2c: Add mirrorProperties (depend on relations existing)
	if len(storedMirrorProps) > 0 {
		i.reportProgress("Blueprints (adding mirrorProperties)", 0, len(storedMirrorProps))
		count := 0
		for id, mirrorProps := range storedMirrorProps {
			if !allExistingBPs[id] {
				continue
			}
			id, mirrorProps := id, mirrorProps
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"mirrorProperties": mirrorProps})
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding mirrorProperties)", count, len(storedMirrorProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Phase 2d: Add aggregationProperties (depend on properties on OTHER blueprints).
	// Run twice: some aggregations reference aggregationProperties on other blueprints
	// that are created in the same pass, so a second pass resolves those dependencies.
	if len(storedAggProps) > 0 {
		for pass := 1; pass <= 2; pass++ {
			label := fmt.Sprintf("Blueprints (adding aggregationProperties, pass %d/2)", pass)
			i.reportProgress(label, 0, len(storedAggProps))
			count := 0
			var failedIDs []string
			var failedMu sync.Mutex
			for id, aggProps := range storedAggProps {
				if !allExistingBPs[id] {
					continue
				}
				id, aggProps := id, aggProps
				pool.Go(func() {
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"aggregationProperties": aggProps})
					i.mu.Lock()
					count++
					i.reportProgress(label, count, len(storedAggProps))
					i.mu.Unlock()
					if err != nil && pass == 1 {
						failedMu.Lock()
						failedIDs = append(failedIDs, id)
						failedMu.Unlock()
					} else if err != nil {
						i.mu.Lock()
						i.errors.Add(err, "blueprint", id)
						i.mu.Unlock()
					}
				})
			}
			pool.Wait()

			// Second pass only retries blueprints that failed in the first pass.
			if pass == 1 {
				next := make(map[string]map[string]interface{}, len(failedIDs))
				for _, id := range failedIDs {
					next[id] = storedAggProps[id]
				}
				storedAggProps = next
			}
		}
	}

	// Phase 2e: Add ownership (inherited ownership depends on relations existing)
	if len(storedOwnership) > 0 {
		var ownershipBlueprints []api.Blueprint
		for _, bp := range append(nonSystemBPs, systemBPs...) {
			id, ok := bp["identifier"].(string)
			if !ok || id == "" {
				continue
			}
			if ownership, ok := storedOwnership[id]; ok && len(ownership) > 0 {
				ownershipBlueprints = append(ownershipBlueprints, bp)
			}
		}

		levels, cyclic := TopologicalSortOwnership(ownershipBlueprints)
		totalOwnership := len(FlattenLevels(levels)) + len(cyclic)
		appliedCount := 0

		for levelIdx, level := range levels {
			i.reportProgress(fmt.Sprintf("Blueprints (adding ownership level %d/%d)", levelIdx+1, len(levels)), appliedCount, totalOwnership)
			for _, bp := range level {
				id := bp["identifier"].(string)
				if !allExistingBPs[id] {
					continue
				}
				ownership := storedOwnership[id]
				pool.Go(func() {
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"ownership": ownership})
					i.mu.Lock()
					if err != nil {
						i.errors.Add(err, "blueprint", id)
					}
					appliedCount++
					i.mu.Unlock()
				})
			}
			pool.Wait()
		}

		if len(cyclic) > 0 {
			i.reportProgress("Blueprints (adding ownership cyclic)", appliedCount, totalOwnership)
			for _, bp := range cyclic {
				id := bp["identifier"].(string)
				if !allExistingBPs[id] {
					continue
				}
				ownership := storedOwnership[id]
				pool.Go(func() {
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"ownership": ownership})
					i.mu.Lock()
					if err != nil {
						i.errors.Add(err, "blueprint", id)
					}
					appliedCount++
					i.mu.Unlock()
				})
			}
			pool.Wait()
		}
	}

	// Phase 3: Update system blueprints
	if len(systemBPs) > 0 {
		i.reportProgress("System blueprints", 0, len(systemBPs))
		sysCount := 0

		for _, bp := range systemBPs {
			bp := bp
			pool.Go(func() {
				id := bp["identifier"].(string)
				_, updated, err := i.createOrUpdateBlueprint(ctx, bp)

				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				} else if updated {
					result.BlueprintsUpdated++
				}
				sysCount++
				i.reportProgress("System blueprints", sysCount, len(systemBPs))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	return nil
}

// createOrUpdateBlueprint creates or updates a single blueprint.
// Returns (created, updated, error).
func (i *Importer) createOrUpdateBlueprint(ctx context.Context, bp api.Blueprint) (bool, bool, error) {
	id, _ := bp["identifier"].(string)

	// Try create first
	_, err := i.client.CreateBlueprint(ctx, bp)
	if err == nil {
		return true, false, nil
	}

	// If conflict, try update
	if isConflictError(err) {
		_, updateErr := i.client.UpdateBlueprint(ctx, id, bp)
		if updateErr != nil {
			return false, false, updateErr
		}
		return false, true, nil
	}

	return false, false, err
}

// updateBlueprintFields updates a blueprint with dependent fields (relations, mirrorProperties, etc.).
// Deprecated: Use updateBlueprintFieldsDirect instead for phased updates.
func (i *Importer) updateBlueprintFields(ctx context.Context, id string, fields map[string]interface{}, existingBPs map[string]bool) error {
	// Validate dependencies before update
	tempBP := api.Blueprint(fields)
	missing := ValidateAllDependencies(tempBP, existingBPs)
	if len(missing) > 0 {
		return fmt.Errorf("cannot add dependent fields - missing blueprints: %v", missing)
	}

	// Fetch existing blueprint
	existing, err := i.client.GetBlueprint(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to fetch blueprint: %w", err)
	}

	// Merge in the dependent fields
	for k, v := range fields {
		existing[k] = v
	}

	// Update
	_, err = i.client.UpdateBlueprint(ctx, id, existing)
	if err != nil {
		return fmt.Errorf("failed to update with dependent fields: %w", err)
	}

	return nil
}

// updateBlueprintFieldsDirect updates a blueprint by merging in specific fields.
// This fetches the existing blueprint and merges the new fields, properly handling
// nested maps (like adding new properties to existing calculationProperties).
func (i *Importer) updateBlueprintFieldsDirect(ctx context.Context, id string, fields map[string]interface{}) error {
	// Fetch existing blueprint
	existing, err := i.client.GetBlueprint(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to fetch blueprint: %w", err)
	}

	// Merge in the new fields
	// For nested maps (relations, calculationProperties, etc.), merge the contents
	for k, v := range fields {
		if newMap, ok := v.(map[string]interface{}); ok {
			// Check if existing has this field as a map
			if existingMap, ok := existing[k].(map[string]interface{}); ok {
				// Merge: add new items to existing map
				for itemKey, itemVal := range newMap {
					existingMap[itemKey] = itemVal
				}
				existing[k] = existingMap
			} else {
				// No existing value or not a map, just set it
				existing[k] = v
			}
		} else {
			existing[k] = v
		}
	}

	// Update
	_, err = i.client.UpdateBlueprint(ctx, id, existing)
	if err != nil {
		return fmt.Errorf("failed to update blueprint fields: %w", err)
	}

	return nil
}

// importOtherResources imports non-blueprint resources with bounded concurrency.
func (i *Importer) importOtherResources(ctx context.Context, data *export.Data, opts Options, result *Result) error {
	// Import entities
	if !opts.SkipEntities && shouldImport("entities", opts.IncludeResources) {
		if err := i.importEntities(ctx, data.Entities, result); err != nil {
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
		i.importUsers(ctx, data.Users, result, pool)
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

func (i *Importer) importSidebarPipeline(ctx context.Context, pipeline []SidebarPipelineStep, result *Result) {
	for _, step := range pipeline {
		pool := NewWorkerPool(DefaultConcurrency)
		for _, op := range step.Operations {
			op := op
			pool.Go(func() {
				switch op.ResourceType {
				case "folder":
					folderID := op.Identifier
					postedFolder := CleanFolderForCreate(op.Folder)
					if err := i.client.CreateFolder(ctx, postedFolder); err != nil && !isConflictError(err) {
						i.mu.Lock()
						i.errors.Add(err, "folder", folderID)
						i.mu.Unlock()
						return
					}
					i.logFolderCreateMismatch(ctx, folderID, postedFolder)
				case "page":
					i.importPage(ctx, op.Page, result)
				}
			})
		}
		pool.Wait()
	}
}

// importEntities imports entities with two-phase approach and bounded concurrency.
// Phase 1: Create all entities with relations stripped (to avoid missing entity references)
// Phase 2: Update entities that have relations to add them back
func (i *Importer) importEntities(ctx context.Context, entities []api.Entity, result *Result) error {
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
		if isProtectedBlueprint(blueprintID) {
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

	// Phase 1: Create/update all entities with relations stripped
	i.reportProgress(fmt.Sprintf("Entities Phase 1%s", skippedMsg), 0, total)
	pool := NewWorkerPool(EntityConcurrency)
	processedCount := 0
	successfulEntities := make(map[string]bool)
	var successMu sync.Mutex

	for _, entity := range filteredEntities {
		entity := entity
		pool.Go(func() {
			blueprintID, ok1 := entity["blueprint"].(string)
			entityID, ok2 := entity["identifier"].(string)
			if !ok1 || !ok2 || blueprintID == "" || entityID == "" {
				return
			}

			// Strip relations for phase 1
			strippedEntity := StripEntityRelations(entity)
			created, updated, err := i.createOrUpdateEntity(ctx, blueprintID, entityID, strippedEntity)

			i.mu.Lock()
			if err != nil {
				i.errors.Add(err, "entity", entityID)
			} else {
				if created {
					result.EntitiesCreated++
				} else if updated {
					result.EntitiesUpdated++
				}
				successMu.Lock()
				successfulEntities[fmt.Sprintf("%s:%s", blueprintID, entityID)] = true
				successMu.Unlock()
			}
			processedCount++
			if processedCount%100 == 0 || processedCount == total {
				i.reportProgress("Entities Phase 1", processedCount, total)
			}
			i.mu.Unlock()
		})
	}

	pool.Wait()

	// Phase 2: Update entities that have relations
	if len(entitiesWithRelations) > 0 {
		i.reportProgress("Entities Phase 2 (relations)", 0, len(entitiesWithRelations))
		pool2 := NewWorkerPool(EntityConcurrency)
		phase2Count := 0

		for _, entity := range entitiesWithRelations {
			entity := entity
			pool2.Go(func() {
				blueprintID, _ := entity["blueprint"].(string)
				entityID, _ := entity["identifier"].(string)
				key := fmt.Sprintf("%s:%s", blueprintID, entityID)

				// Only update if phase 1 succeeded
				successMu.Lock()
				wasSuccessful := successfulEntities[key]
				successMu.Unlock()

				if !wasSuccessful {
					return
				}

				// Update with full entity (including relations)
				_, updateErr := i.client.UpdateEntity(ctx, blueprintID, entityID, entity)

				i.mu.Lock()
				if updateErr != nil {
					i.errors.Add(updateErr, "entity", entityID)
				}
				phase2Count++
				if phase2Count%100 == 0 || phase2Count == len(entitiesWithRelations) {
					i.reportProgress("Entities Phase 2 (relations)", phase2Count, len(entitiesWithRelations))
				}
				i.mu.Unlock()
			})
		}

		pool2.Wait()
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

// importScorecards imports scorecards grouped by blueprint.
func (i *Importer) importScorecards(ctx context.Context, scorecards []api.Scorecard, result *Result, pool *WorkerPool) {
	// Group by blueprint
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
			for _, sc := range scs {
				scID := sc["identifier"].(string)
				_, err := i.client.CreateScorecard(ctx, bpID, sc)

				i.mu.Lock()
				if err == nil {
					result.ScorecardsCreated++
				} else if isConflictError(err) {
					// Try update via bulk endpoint
					_, updateErr := i.client.UpdateScorecards(ctx, bpID, []api.Scorecard{sc})
					if updateErr != nil {
						i.errors.Add(updateErr, "scorecard", scID)
					} else {
						result.ScorecardsUpdated++
					}
				} else {
					i.errors.Add(err, "scorecard", scID)
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

// importUsers imports users.
func (i *Importer) importUsers(ctx context.Context, users []api.User, result *Result, pool *WorkerPool) {
	for _, user := range users {
		user := user
		pool.Go(func() {
			userEmail, ok := user["email"].(string)
			if !ok || userEmail == "" {
				return
			}

			cleaned := cleanSystemFields(user, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
			apiUser := api.User(cleaned)

			_, err := i.client.InviteUser(ctx, apiUser)

			i.mu.Lock()
			if err == nil {
				result.UsersCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateUser(ctx, userEmail, apiUser)
				if updateErr != nil {
					i.errors.Add(updateErr, "user", userEmail)
				} else {
					result.UsersUpdated++
				}
			} else {
				i.errors.Add(err, "user", userEmail)
			}
			i.mu.Unlock()
		})
	}
}

// isSidebarParentNotFound returns true when Port rejects a page because its parent
// sidebar item does not exist in the target organisation.
func isSidebarParentNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Sidebar item")
}

// IsSidebarParentNotFound is the exported form for use by the migrate package.
func IsSidebarParentNotFound(err error) bool {
	return isSidebarParentNotFound(err)
}

// IsAfterItemNotInParent returns true when Port rejects page creation because the
// `after` sibling item doesn't exist inside the specified parent folder.
func IsAfterItemNotInParent(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "after_item_not_in_parent") ||
		strings.Contains(err.Error(), "is not in the parent folder") ||
		strings.Contains(err.Error(), "Sidebar item with after")
}

// IsAgentIdentifierError returns true when the Port API rejects a request because
// a widget is missing the required agentIdentifier field.
func IsAgentIdentifierError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "agentIdentifier")
}

// isAdditionalPropertyError returns true when Port rejects a request because a field
// is not allowed for that page type (e.g. sidebar/requiredQueryParams on entity pages).
func isAdditionalPropertyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "must NOT have additional properties")
}

var additionalPropertyPattern = regexp.MustCompile(`additional property: (?:\\\")?([^"\\]+)(?:\\\")?`)

func extractAdditionalProperty(err error) string {
	if err == nil {
		return ""
	}
	matches := additionalPropertyPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

// IsAdditionalPropertyError is the exported form for use by the migrate package.
func IsAdditionalPropertyError(err error) bool {
	return isAdditionalPropertyError(err)
}

// actionAuditFields are the audit/internal fields that must be stripped before
// sending an action or automation to the Port API.
var actionAuditFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}

// CleanActionForCreate returns a copy of the action with audit fields removed.
func CleanActionForCreate(action api.Action) api.Action {
	return api.Action(cleanSystemFields(map[string]interface{}(action), actionAuditFields))
}

// CleanPageForCreateNoNav is like CleanPageForCreate but also strips navigation
// fields (after, sidebar, parent, section, requiredQueryParams). Used as a fallback
// when the target org is missing the sidebar parent.
func CleanPageForCreateNoNav(page api.Page) api.Page {
	strip := append(pageMetaFields, pageNavFields...)
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// MergeWidgetAgentIdentifiers copies agentIdentifier values from existing
// widgets into new widgets so that Port's required-field validation passes.
func MergeWidgetAgentIdentifiers(newWidgets, existingWidgets []interface{}) []interface{} {
	return mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
}

// SortPagesByAfterDeps is the exported version of sortPagesByAfterDeps for use by migrate.
func SortPagesByAfterDeps(pages []api.Page) []api.Page {
	return sortPagesByAfterDeps(pages)
}

func SortFoldersByAfterLevels(folders []api.Folder) [][]api.Folder {
	pipeline := PlanSidebarPipeline(folders, nil)
	levels := make([][]api.Folder, 0, len(pipeline))
	for _, step := range pipeline {
		level := make([]api.Folder, 0, len(step.Operations))
		for _, op := range step.Operations {
			if op.ResourceType == "folder" {
				level = append(level, op.Folder)
			}
		}
		if len(level) > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

func PlanSidebarPipeline(folders []api.Folder, pages []api.Page) []SidebarPipelineStep {
	opsByID := make(map[string]SidebarPipelineOperation, len(folders)+len(pages))
	inDegree := make(map[string]int, len(folders)+len(pages))
	dependents := make(map[string][]string, len(folders)+len(pages))

	for _, folder := range folders {
		id, _ := folder["identifier"].(string)
		if id == "" {
			continue
		}
		opsByID[id] = SidebarPipelineOperation{
			ResourceType: "folder",
			Identifier:   id,
			Folder:       folder,
		}
		inDegree[id] = 0
	}
	for _, page := range pages {
		id, _ := page["identifier"].(string)
		if id == "" {
			continue
		}
		opsByID[id] = SidebarPipelineOperation{
			ResourceType: "page",
			Identifier:   id,
			Page:         page,
		}
		inDegree[id] = 0
	}

	addDependency := func(id, dep string) {
		if id == "" || dep == "" || id == dep {
			return
		}
		if _, exists := opsByID[dep]; !exists {
			return
		}
		inDegree[id]++
		dependents[dep] = append(dependents[dep], id)
	}

	for _, folder := range folders {
		id, _ := folder["identifier"].(string)
		if id == "" {
			continue
		}
		deps := make(map[string]bool)
		if parent, _ := folder["parent"].(string); parent != "" {
			deps[parent] = true
		}
		if after, _ := folder["after"].(string); after != "" {
			deps[after] = true
		}
		for dep := range deps {
			addDependency(id, dep)
		}
	}
	for _, page := range pages {
		id, _ := page["identifier"].(string)
		if id == "" {
			continue
		}
		deps := make(map[string]bool)
		if parent, _ := page["parent"].(string); parent != "" {
			deps[parent] = true
		}
		if after, _ := page["after"].(string); after != "" {
			deps[after] = true
		}
		for dep := range deps {
			addDependency(id, dep)
		}
	}

	remaining := make(map[string]bool, len(opsByID))
	for id := range opsByID {
		remaining[id] = true
	}

	var steps []SidebarPipelineStep
	for len(remaining) > 0 {
		readyIDs := make([]string, 0, len(remaining))
		for id := range remaining {
			if inDegree[id] == 0 {
				readyIDs = append(readyIDs, id)
			}
		}
		if len(readyIDs) == 0 {
			for id := range remaining {
				readyIDs = append(readyIDs, id)
			}
		}
		sort.Strings(readyIDs)

		step := SidebarPipelineStep{
			Operations: make([]SidebarPipelineOperation, 0, len(readyIDs)),
		}
		for _, id := range readyIDs {
			step.Operations = append(step.Operations, opsByID[id])
		}
		steps = append(steps, step)

		for _, id := range readyIDs {
			delete(remaining, id)
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
	}

	return steps
}

func DescribeSidebarPipeline(steps []SidebarPipelineStep) []string {
	lines := make([]string, 0, len(steps))
	for idx, step := range steps {
		var folders []string
		var pages []string
		for _, op := range step.Operations {
			switch op.ResourceType {
			case "folder":
				folders = append(folders, op.Identifier)
			case "page":
				pages = append(pages, op.Identifier)
			}
		}
		sort.Strings(folders)
		sort.Strings(pages)

		parts := make([]string, 0, 2)
		if len(folders) > 0 {
			parts = append(parts, fmt.Sprintf("folders [%s]", strings.Join(folders, ", ")))
		}
		if len(pages) > 0 {
			parts = append(parts, fmt.Sprintf("pages [%s]", strings.Join(pages, ", ")))
		}
		lines = append(lines, fmt.Sprintf("Step %d: %s", idx+1, strings.Join(parts, " | ")))
	}
	return lines
}

// sortPagesByAfterDeps returns pages sorted so that if page B has after=A and A is
// also in the list, A comes before B.
func sortPagesByAfterDeps(pages []api.Page) []api.Page {
	pageSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		if id, ok := p["identifier"].(string); ok {
			pageSet[id] = true
		}
	}

	result := make([]api.Page, 0, len(pages))
	placed := make(map[string]bool, len(pages))
	remaining := make([]api.Page, len(pages))
	copy(remaining, pages)

	for len(remaining) > 0 {
		added := 0
		var next []api.Page
		for _, p := range remaining {
			after, _ := p["after"].(string)
			if !pageSet[after] || placed[after] {
				result = append(result, p)
				if id, ok := p["identifier"].(string); ok {
					placed[id] = true
				}
				added++
			} else {
				next = append(next, p)
			}
		}
		remaining = next
		if added == 0 {
			result = append(result, remaining...)
			break
		}
	}
	return result
}

func CleanFolderForCreate(folder api.Folder) api.Folder {
	cleaned := make(api.Folder)
	for _, key := range []string{"identifier", "title", "after", "parent"} {
		if value, ok := folder[key]; ok && value != nil {
			cleaned[key] = value
		}
	}
	return cleaned
}

// sortPagesByAfterLevels groups pages into levels where all pages within a level
// have no after-dependencies on each other. Pages in the same level can be
// processed concurrently; levels must be processed sequentially.
func sortPagesByAfterLevels(pages []api.Page) [][]api.Page {
	pageSet := make(map[string]bool, len(pages))
	pageByID := make(map[string]api.Page, len(pages))
	for _, p := range pages {
		if id, ok := p["identifier"].(string); ok && id != "" {
			pageSet[id] = true
			pageByID[id] = p
		}
	}

	inDegree := make(map[string]int, len(pages))
	dependents := make(map[string][]string, len(pages))
	for _, p := range pages {
		id, _ := p["identifier"].(string)
		if id == "" {
			continue
		}
		inDegree[id] = 0
	}
	for _, p := range pages {
		id, _ := p["identifier"].(string)
		after, _ := p["after"].(string)
		if id == "" || after == "" || !pageSet[after] {
			continue
		}
		inDegree[id]++
		dependents[after] = append(dependents[after], id)
	}

	remaining := make(map[string]bool, len(pages))
	for id := range inDegree {
		remaining[id] = true
	}

	var levels [][]api.Page
	for len(remaining) > 0 {
		var level []api.Page
		for id := range remaining {
			if inDegree[id] == 0 {
				level = append(level, pageByID[id])
			}
		}
		if len(level) == 0 {
			// Cycle — flush all remaining to break deadlock.
			for id := range remaining {
				level = append(level, pageByID[id])
			}
		}
		for _, p := range level {
			id, _ := p["identifier"].(string)
			delete(remaining, id)
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
		levels = append(levels, level)
	}
	return levels
}

// applyPageOrdering applies the `after` field for pages that have one, sequentially
// and in topological dependency order. This is called after the concurrent page
// content pass so that sidebar ordering is set without race conditions.
func (i *Importer) applyPageOrdering(ctx context.Context, pages []api.Page, result *Result) {
	// Collect pages that have a non-empty after value.
	var toOrder []api.Page
	for _, p := range pages {
		if after, ok := p["after"].(string); ok && after != "" {
			toOrder = append(toOrder, p)
		}
	}
	if len(toOrder) == 0 {
		return
	}

	sorted := sortPagesByAfterDeps(toOrder)
	for _, p := range sorted {
		pageID, ok := p["identifier"].(string)
		if !ok || pageID == "" {
			continue
		}
		after := p["after"].(string)
		_, err := i.client.UpdatePage(ctx, pageID, api.Page{"identifier": pageID, "after": after})
		if err != nil {
			// A missing sibling is benign — the page exists, just not in the exact spot.
			if !isSidebarParentNotFound(err) {
				i.errors.Add(err, "page", pageID)
			}
		}
	}
}

// importPages imports pages in topological `after` order.
// Pages are grouped into levels: all pages in a level are independent of each
// other (no after-dependency between them) and can run concurrently. Levels are
// processed sequentially so that `after` targets are always present before their
// dependents. This avoids race conditions without a separate second pass.
func (i *Importer) importPages(ctx context.Context, pages []api.Page, result *Result) {
	levels := sortPagesByAfterLevels(pages)
	for _, level := range levels {
		pool := NewWorkerPool(DefaultConcurrency)
		for _, page := range level {
			page := page
			pool.Go(func() {
				i.importPage(ctx, page, result)
			})
		}
		pool.Wait()
	}
}

// importPage imports a single page.
func (i *Importer) importPage(ctx context.Context, page api.Page, result *Result) {
	pageID, ok := page["identifier"].(string)
	if !ok || pageID == "" {
		return
	}

	// metaFields are always stripped (audit metadata, internal IDs).
	metaFields := []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected"}
	// navFields control sidebar placement.
	navFields := []string{"after", "section", "sidebar", "parent", "requiredQueryParams"}

	// buildPage strips the given extra fields and recursively cleans widget metadata.
	buildPage := func(extra []string) api.Page {
		strip := append(metaFields, extra...)
		cleaned := cleanSystemFields(page, strip)
		if widgets, ok := cleaned["widgets"].([]interface{}); ok {
			cleaned["widgets"] = cleanWidgetsRecursive(widgets)
		}
		return api.Page(cleaned)
	}

	// pageForCreate keeps `type` and sidebar placement fields, but strips
	// requiredQueryParams because Port rejects it for some page types on create.
	pageForCreate := buildPage([]string{"requiredQueryParams"})
	// pageForUpdate keeps navigation fields (including `after`) so Port places the
	// page in the correct sidebar position. `type` is stripped because the PATCH
	// endpoint rejects it. Null string nav fields are stripped to avoid clearing
	// existing values in the target.
	pageForUpdate := buildPage([]string{"type", "requiredQueryParams", "sidebar"})
	for _, field := range navFields {
		if v, exists := pageForUpdate[field]; exists && v == nil {
			delete(pageForUpdate, field)
		}
	}
	var (
		createPosted api.Page
		createdPage  api.Page
		err          error
	)

	createPosted = pageForCreate
	createdPage, err = i.client.CreatePage(ctx, createPosted)

	needsUpdate := false
	i.mu.Lock()
	if err == nil {
		result.PagesCreated++
		i.mu.Unlock()
		i.logPageCreateMismatch(ctx, pageID, pageForCreate, createPosted, createdPage)
		return
	} else if IsAfterItemNotInParent(err) || extractAdditionalProperty(err) != "" {
		createPosted, createdPage, retryErr := i.retryCreatePageWithNarrowFallbacks(ctx, pageForCreate, err)
		if retryErr == nil {
			result.PagesCreated++
			i.mu.Unlock()
			i.logPageCreateMismatch(ctx, pageID, pageForCreate, createPosted, createdPage)
			return
		} else if isConflictError(retryErr) {
			needsUpdate = true
		}
	} else if isConflictError(err) {
		needsUpdate = true
	} else if strings.Contains(err.Error(), "agentIdentifier") {
		// Create failed with agentIdentifier — check if the page already exists.
		existingPage, fetchErr := i.client.GetPage(ctx, pageID)
		if fetchErr == nil && existingPage != nil {
			pageWithoutWidgets := make(api.Page)
			for k, v := range pageForUpdate {
				if k != "widgets" {
					pageWithoutWidgets[k] = v
				}
			}
			_, updateErr := i.client.UpdatePage(ctx, pageID, pageWithoutWidgets)
			if updateErr != nil {
				i.errors.Add(updateErr, "page", pageID)
			} else {
				result.PagesUpdated++
			}
		} else {
			i.errors.Add(err, "page", pageID)
		}
	} else {
		i.errors.Add(err, "page", pageID)
	}

	if needsUpdate {
		// Fetch existing page to preserve fields like agentIdentifier.
		existingPage, fetchErr := i.client.GetPage(ctx, pageID)
		if fetchErr == nil && existingPage != nil {
			if existingWidgets, ok := existingPage["widgets"].([]interface{}); ok {
				if newWidgets, ok := pageForUpdate["widgets"].([]interface{}); ok {
					pageForUpdate["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
				}
			}
		}

		_, updateErr := i.client.UpdatePage(ctx, pageID, pageForUpdate)
		if updateErr != nil {
			if IsAfterItemNotInParent(updateErr) || extractAdditionalProperty(updateErr) != "" {
				_, retryErr := i.retryUpdatePageWithNarrowFallbacks(ctx, pageID, pageForUpdate, updateErr)
				if retryErr != nil {
					i.errors.Add(retryErr, "page", pageID)
				} else {
					result.PagesUpdated++
				}
			} else if strings.Contains(updateErr.Error(), "agentIdentifier") {
				// Fetch existing page to merge agentIdentifiers from its widgets, then retry.
				if existingPage, fetchErr := i.client.GetPage(ctx, pageID); fetchErr == nil && existingPage != nil {
					if existingWidgets, ok := existingPage["widgets"].([]interface{}); ok {
						if newWidgets, ok := pageForUpdate["widgets"].([]interface{}); ok {
							pageForUpdate["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
						}
					}
				}
				_, retryErr := i.client.UpdatePage(ctx, pageID, pageForUpdate)
				if retryErr != nil {
					// Last resort: update without widgets.
					pageWithoutWidgets := make(api.Page)
					for k, v := range pageForUpdate {
						if k != "widgets" {
							pageWithoutWidgets[k] = v
						}
					}
					_, lastErr := i.client.UpdatePage(ctx, pageID, pageWithoutWidgets)
					if lastErr != nil {
						i.errors.Add(lastErr, "page", pageID)
					} else {
						result.PagesUpdated++
					}
				} else {
					result.PagesUpdated++
				}
			} else {
				i.errors.Add(updateErr, "page", pageID)
			}
		} else {
			result.PagesUpdated++
		}
	}
	i.mu.Unlock()
}

func (i *Importer) logPageCreateMismatch(ctx context.Context, pageID string, intended api.Page, posted api.Page, created api.Page) {
	if !i.verbose {
		return
	}
	actualPage, err := i.client.GetPage(ctx, pageID)
	if err == nil && actualPage != nil {
		created = actualPage
	}

	normalizedIntended := normalizePageForLog(intended)
	normalizedPosted := normalizePageForLog(posted)
	normalizedCreated := normalizePageForLog(created)

	if subsetEqual(normalizedPosted, normalizedCreated) && subsetEqual(normalizedIntended, normalizedCreated) {
		return
	}

	lines := []string{
		fmt.Sprintf("Page create mismatch for %s", pageID),
	}
	if !subsetEqual(normalizedIntended, normalizedPosted) {
		lines = append(lines, fmt.Sprintf("  intended: %s", mustJSON(normalizedIntended)))
	}
	lines = append(lines,
		fmt.Sprintf("  posted: %s", mustJSON(normalizedPosted)),
		fmt.Sprintf("  created: %s", mustJSON(normalizedCreated)),
	)
	i.logLines(lines)
}

func (i *Importer) logFolderCreateMismatch(ctx context.Context, folderID string, posted api.Folder) {
	if !i.verbose {
		return
	}
	folders, err := i.client.GetFolders(ctx)
	if err != nil {
		i.logLines([]string{
			fmt.Sprintf("Folder create mismatch check failed for %s", folderID),
			fmt.Sprintf("  posted: %s", mustJSON(normalizeFolderForLog(posted))),
			fmt.Sprintf("  error: %v", err),
		})
		return
	}

	var actual api.Folder
	for _, folder := range folders {
		if identifier, _ := folder["identifier"].(string); identifier == folderID {
			actual = folder
			break
		}
	}
	if actual == nil {
		i.logLines([]string{
			fmt.Sprintf("Folder create mismatch for %s", folderID),
			fmt.Sprintf("  posted: %s", mustJSON(normalizeFolderForLog(posted))),
			"  created: null",
		})
		return
	}

	normalizedPosted := normalizeFolderForLog(posted)
	normalizedActual := normalizeFolderForLog(actual)
	if subsetEqual(normalizedPosted, normalizedActual) {
		return
	}

	i.logLines([]string{
		fmt.Sprintf("Folder create mismatch for %s", folderID),
		fmt.Sprintf("  posted: %s", mustJSON(normalizedPosted)),
		fmt.Sprintf("  created: %s", mustJSON(normalizedActual)),
	})
}

func (i *Importer) logLines(lines []string) {
	if i.log == nil {
		return
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	for _, line := range lines {
		i.log(line)
	}
}

func normalizePageForLog(page api.Page) api.Page {
	if page == nil {
		return nil
	}
	normalized := make(api.Page)
	for _, key := range []string{"identifier", "title", "type", "after", "parent", "section", "sidebar", "showInSidebar", "requiredQueryParams"} {
		if value, ok := page[key]; ok && value != nil {
			normalized[key] = value
		}
	}
	return normalized
}

func clonePage(page api.Page) api.Page {
	if page == nil {
		return nil
	}

	cloned := make(api.Page, len(page))
	for key, value := range page {
		cloned[key] = value
	}
	return cloned
}

func (i *Importer) retryCreatePageWithNarrowFallbacks(ctx context.Context, base api.Page, initialErr error) (api.Page, api.Page, error) {
	candidate := clonePage(base)
	currentErr := initialErr

	for {
		nextCandidate, changed := removeSingleFailingPageField(candidate, currentErr)
		if !changed {
			return candidate, nil, currentErr
		}

		createdPage, retryErr := i.client.CreatePage(ctx, nextCandidate)
		if retryErr == nil {
			return nextCandidate, createdPage, nil
		}
		if isConflictError(retryErr) {
			return nextCandidate, nil, retryErr
		}

		candidate = nextCandidate
		currentErr = retryErr
	}
}

func (i *Importer) retryUpdatePageWithNarrowFallbacks(ctx context.Context, pageID string, base api.Page, initialErr error) (api.Page, error) {
	candidate := clonePage(base)
	currentErr := initialErr

	for {
		nextCandidate, changed := removeSingleFailingPageField(candidate, currentErr)
		if !changed {
			return candidate, currentErr
		}

		updatedPage, retryErr := i.client.UpdatePage(ctx, pageID, nextCandidate)
		if retryErr == nil {
			return updatedPage, nil
		}

		candidate = nextCandidate
		currentErr = retryErr
	}
}

func removeSingleFailingPageField(page api.Page, err error) (api.Page, bool) {
	candidate := clonePage(page)

	if IsAfterItemNotInParent(err) {
		if _, exists := candidate["after"]; exists {
			delete(candidate, "after")
			return candidate, true
		}
	}

	if invalidProperty := extractAdditionalProperty(err); invalidProperty != "" {
		if _, exists := candidate[invalidProperty]; exists {
			delete(candidate, invalidProperty)
			return candidate, true
		}
	}

	return page, false
}

func normalizeFolderForLog(folder api.Folder) api.Folder {
	if folder == nil {
		return nil
	}

	normalized := make(api.Folder)
	for _, key := range []string{"identifier", "title", "after", "parent", "sidebar", "section", "showInSidebar"} {
		if value, ok := folder[key]; ok && value != nil {
			normalized[key] = value
		}
	}
	return normalized
}

func subsetEqual(expected, actual interface{}) bool {
	switch actualTyped := actual.(type) {
	case api.Page:
		return subsetEqual(expected, map[string]interface{}(actualTyped))
	case api.Folder:
		return subsetEqual(expected, map[string]interface{}(actualTyped))
	}

	switch expectedTyped := expected.(type) {
	case map[string]interface{}:
		actualTyped, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for key, expectedValue := range expectedTyped {
			actualValue, exists := actualTyped[key]
			if !exists || !subsetEqual(expectedValue, actualValue) {
				return false
			}
		}
		return true
	case api.Page:
		return subsetEqual(map[string]interface{}(expectedTyped), actual)
	case api.Folder:
		return subsetEqual(map[string]interface{}(expectedTyped), actual)
	case []interface{}:
		actualTyped, ok := actual.([]interface{})
		if !ok || len(expectedTyped) != len(actualTyped) {
			return false
		}
		for idx := range expectedTyped {
			if !subsetEqual(expectedTyped[idx], actualTyped[idx]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(expected, actual)
	}
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// pageMetaFields are the audit/internal fields always stripped before sending a page to Port.
var pageMetaFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected"}

// pageNavFields control sidebar placement; Port rejects them when the referenced parent
// doesn't exist in the target org.
var pageNavFields = []string{"after", "section", "sidebar", "parent", "requiredQueryParams"}

// CleanPageForCreate returns a copy of page with audit/internal fields removed.
// Sidebar placement fields are preserved, but requiredQueryParams is stripped
// because Port rejects it for some page types on create.
func CleanPageForCreate(page api.Page) api.Page {
	strip := append(pageMetaFields, "requiredQueryParams")
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// CleanPageForUpdate returns a copy of page with audit/internal fields and `type`
// removed. Navigation fields are kept so Port can move the page to the correct
// sidebar position, except requiredQueryParams and sidebar which are stripped by
// default because Port rejects them for some page types on update. Nav fields
// that are nil/null are also stripped — sending null would clear the page's
// existing navigation context in Port.
func CleanPageForUpdate(page api.Page) api.Page {
	strip := append(pageMetaFields, "type", "requiredQueryParams", "sidebar")
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	for _, field := range pageNavFields {
		if v, exists := cleaned[field]; exists && v == nil {
			delete(cleaned, field)
		}
	}
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// CleanPageForUpdateNoNav is the fallback for CleanPageForUpdate when Port rejects
// the update because the parent page doesn't exist in the target org.
func CleanPageForUpdateNoNav(page api.Page) api.Page {
	strip := append(pageMetaFields, append(pageNavFields, "type")...)
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// cleanWidgetsRecursive removes system fields from widgets and their nested widgets.
// It also fixes widget configurations that would cause validation errors.
func cleanWidgetsRecursive(widgets []interface{}) []interface{} {
	systemFields := map[string]bool{
		"createdBy": true, "updatedBy": true, "createdAt": true, "updatedAt": true,
	}

	result := make([]interface{}, 0, len(widgets))
	for _, w := range widgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			result = append(result, w)
			continue
		}

		// Clean system fields from this widget
		cleaned := make(map[string]interface{})
		for k, v := range widget {
			if systemFields[k] {
				continue
			}
			// Recursively clean nested widgets
			if k == "widgets" {
				if nestedWidgets, ok := v.([]interface{}); ok {
					cleaned[k] = cleanWidgetsRecursive(nestedWidgets)
					continue
				}
			}
			// Recursively clean groups (which contain widgets)
			if k == "groups" {
				if groups, ok := v.([]interface{}); ok {
					cleanedGroups := make([]interface{}, 0, len(groups))
					for _, g := range groups {
						if group, ok := g.(map[string]interface{}); ok {
							cleanedGroup := make(map[string]interface{})
							for gk, gv := range group {
								if gk == "widgets" {
									if groupWidgets, ok := gv.([]interface{}); ok {
										cleanedGroup[gk] = cleanWidgetsRecursive(groupWidgets)
										continue
									}
								}
								cleanedGroup[gk] = gv
							}
							cleanedGroups = append(cleanedGroups, cleanedGroup)
						} else {
							cleanedGroups = append(cleanedGroups, g)
						}
					}
					cleaned[k] = cleanedGroups
					continue
				}
			}
			cleaned[k] = v
		}

		// Fix table-entities-explorer widgets that have dataset but no blueprint
		// The API requires either a blueprint property or a blueprint rule in the dataset
		widgetType, _ := cleaned["type"].(string)
		if widgetType == "table-entities-explorer" {
			_, hasBlueprint := cleaned["blueprint"]
			_, hasDataset := cleaned["dataset"]
			if hasDataset && !hasBlueprint {
				// Add empty blueprint to indicate cross-blueprint dataset query
				cleaned["blueprint"] = ""
			}
		}

		result = append(result, cleaned)
	}
	return result
}

// mergeWidgetAgentIdentifiers copies agentIdentifier from existing widgets to new widgets.
// This is needed because the API now requires agentIdentifier on certain widget types,
// but exported data may not have it.
func mergeWidgetAgentIdentifiers(newWidgets, existingWidgets []interface{}) []interface{} {
	// Build a map of existing widgets by ID for quick lookup
	existingByID := make(map[string]map[string]interface{})
	for _, w := range existingWidgets {
		if widget, ok := w.(map[string]interface{}); ok {
			if id, ok := widget["id"].(string); ok && id != "" {
				existingByID[id] = widget
			}
		}
	}

	result := make([]interface{}, 0, len(newWidgets))
	for idx, w := range newWidgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			result = append(result, w)
			continue
		}

		// Try to find matching existing widget by ID
		var existingWidget map[string]interface{}
		if id, ok := widget["id"].(string); ok && id != "" {
			existingWidget = existingByID[id]
		}
		// Fallback to index-based matching if no ID match
		if existingWidget == nil && idx < len(existingWidgets) {
			if ew, ok := existingWidgets[idx].(map[string]interface{}); ok {
				existingWidget = ew
			}
		}

		// Copy agentIdentifier from existing widget if present and not in new widget
		if existingWidget != nil {
			if agentID, ok := existingWidget["agentIdentifier"]; ok {
				if _, hasAgentID := widget["agentIdentifier"]; !hasAgentID {
					widget["agentIdentifier"] = agentID
				}
			}
		}

		// Recursively merge nested widgets
		if newNestedWidgets, ok := widget["widgets"].([]interface{}); ok {
			var existingNestedWidgets []interface{}
			if existingWidget != nil {
				existingNestedWidgets, _ = existingWidget["widgets"].([]interface{})
			}
			if existingNestedWidgets != nil {
				widget["widgets"] = mergeWidgetAgentIdentifiers(newNestedWidgets, existingNestedWidgets)
			}
		}

		// Recursively merge groups
		if newGroups, ok := widget["groups"].([]interface{}); ok {
			var existingGroups []interface{}
			if existingWidget != nil {
				existingGroups, _ = existingWidget["groups"].([]interface{})
			}
			if existingGroups != nil {
				widget["groups"] = mergeGroupAgentIdentifiers(newGroups, existingGroups)
			}
		}

		result = append(result, widget)
	}
	return result
}

// mergeGroupAgentIdentifiers merges agentIdentifier for widgets within groups.
func mergeGroupAgentIdentifiers(newGroups, existingGroups []interface{}) []interface{} {
	// Build a map of existing groups by title for matching
	existingByTitle := make(map[string]map[string]interface{})
	for _, g := range existingGroups {
		if group, ok := g.(map[string]interface{}); ok {
			if title, ok := group["title"].(string); ok && title != "" {
				existingByTitle[title] = group
			}
		}
	}

	result := make([]interface{}, 0, len(newGroups))
	for idx, g := range newGroups {
		group, ok := g.(map[string]interface{})
		if !ok {
			result = append(result, g)
			continue
		}

		// Try to find matching existing group by title
		var existingGroup map[string]interface{}
		if title, ok := group["title"].(string); ok && title != "" {
			existingGroup = existingByTitle[title]
		}
		// Fallback to index-based matching
		if existingGroup == nil && idx < len(existingGroups) {
			if eg, ok := existingGroups[idx].(map[string]interface{}); ok {
				existingGroup = eg
			}
		}

		// Recursively merge widgets within the group
		if newWidgets, ok := group["widgets"].([]interface{}); ok {
			var existingWidgets []interface{}
			if existingGroup != nil {
				existingWidgets, _ = existingGroup["widgets"].([]interface{})
			}
			if existingWidgets != nil {
				group["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
			}
		}

		result = append(result, group)
	}
	return result
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
// underlying blueprints and actions are guaranteed to exist.
func (i *Importer) importPermissions(ctx context.Context, diff *DiffResult) {
	if diff == nil {
		return
	}

	// Import blueprint permissions
	for _, change := range diff.BlueprintPermissions {
		if _, err := i.client.UpdateBlueprintPermissions(ctx, change.Identifier, change.Permissions); err != nil {
			i.errors.Add(fmt.Errorf("failed to update blueprint permissions for %s: %w", change.Identifier, err), "blueprint_permissions", change.Identifier)
		}
	}

	// Import action permissions
	for _, change := range diff.ActionPermissions {
		if _, err := i.client.UpdateActionPermissions(ctx, change.Identifier, change.Permissions); err != nil {
			i.errors.Add(fmt.Errorf("failed to update action permissions for %s: %w", change.Identifier, err), "action_permissions", change.Identifier)
		}
	}
}

// applyDataExclusion filters data in-place before diffing/importing.
// excludeDeep removes the blueprint schema AND all its entities/scorecards/actions.
// excludeSchema removes only the blueprint schema; resources for that blueprint are kept.
func applyDataExclusion(data *export.Data, excludeDeep, excludeSchema []string, skipSystemBlueprints bool) {
	// Pre-pass: remove system blueprint schemas and their entities (shallow skip).
	// Scorecards, actions, and permissions are kept.
	if skipSystemBlueprints {
		filteredBPs := data.Blueprints[:0:0]
		for _, bp := range data.Blueprints {
			id, _ := bp["identifier"].(string)
			if strings.HasPrefix(id, "_") {
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

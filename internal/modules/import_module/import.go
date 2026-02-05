package import_module

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

// Module handles importing data to Port.
type Module struct {
	client *api.Client
}

// NewModule creates a new import module.
func NewModule(orgConfig *config.OrganizationConfig) *Module {
	client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
	return &Module{
		client: client,
	}
}

// Options represents import options.
type Options struct {
	InputPath        string
	DryRun           bool
	SkipEntities     bool
	IncludeResources []string
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
	DiffResult          *DiffResult
}

// Execute performs the import operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Load data
	loader := NewLoader()
	data, err := loader.LoadData(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

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

	// Dry run - show what would happen
	if opts.DryRun {
		return m.generateDryRunResult(data, diffResult, opts), nil
	}

	// Import data using new reliable importer
	importer := NewImporter(m.client)
	result, err := importer.Import(ctx, data, opts)
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	result.Success = true
	result.Message = "Successfully imported data"
	result.DiffResult = diffResult
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
	progress ProgressCallback
}

// ProgressCallback is called to report import progress.
type ProgressCallback func(phase string, current, total int)

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

// reportProgress reports progress if a callback is set.
func (i *Importer) reportProgress(phase string, current, total int) {
	if i.progress != nil {
		i.progress(phase, current, total)
	}
}

// Import imports data to Port with proper dependency ordering.
func (i *Importer) Import(ctx context.Context, data *export.Data, opts Options) (*Result, error) {
	result := &Result{
		Errors: []string{},
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
	strippedBPs := make([]api.Blueprint, 0, len(nonSystemBPs))

	for _, bp := range nonSystemBPs {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
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

		// Strip both relations AND dependent fields for phase 1
		stripped := StripDependentFields(bp)
		stripped = StripRelations(stripped)
		strippedBPs = append(strippedBPs, stripped)
	}

	// Topological sort
	levels, cyclic := TopologicalSort(strippedBPs, existingBPs)

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

	// Phase 2d: Add aggregationProperties (depend on properties on OTHER blueprints)
	if len(storedAggProps) > 0 {
		i.reportProgress("Blueprints (adding aggregationProperties)", 0, len(storedAggProps))
		count := 0
		for id, aggProps := range storedAggProps {
			if !allExistingBPs[id] {
				continue
			}
			id, aggProps := id, aggProps
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"aggregationProperties": aggProps})
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding aggregationProperties)", count, len(storedAggProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
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
	if shouldImport("teams", opts.IncludeResources) {
		i.importTeams(ctx, data.Teams, result, pool)
	}

	// Import users
	if shouldImport("users", opts.IncludeResources) {
		i.importUsers(ctx, data.Users, result, pool)
	}

	// Import pages
	if shouldImport("pages", opts.IncludeResources) {
		i.importPages(ctx, data.Pages, result, pool)
	}

	// Import integrations
	if shouldImport("integrations", opts.IncludeResources) {
		i.importIntegrations(ctx, data.Integrations, result, pool)
	}

	pool.Wait()
	return nil
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

// importTeams imports teams.
func (i *Importer) importTeams(ctx context.Context, teams []api.Team, result *Result, pool *WorkerPool) {
	for _, team := range teams {
		team := team
		pool.Go(func() {
			teamName, ok := team["name"].(string)
			if !ok || teamName == "" {
				return
			}

			_, err := i.client.CreateTeam(ctx, team)

			i.mu.Lock()
			if err == nil {
				result.TeamsCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateTeam(ctx, teamName, team)
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

// importPages imports pages.
func (i *Importer) importPages(ctx context.Context, pages []api.Page, result *Result, pool *WorkerPool) {
	for _, page := range pages {
		page := page
		pool.Go(func() {
			pageID, ok := page["identifier"].(string)
			if !ok || pageID == "" {
				return
			}

			// Strip system fields and sidebar-related fields to avoid ordering issues
			// Note: parent/after/sidebar must be stripped together to avoid "has_sidebar_location_without_sidebar"
			// Note: "type" must be stripped as the update API doesn't accept it
			cleaned := cleanSystemFields(page, []string{
				"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected",
				"after", "section", "sidebar", "parent", // Strip all sidebar positioning fields together
				"requiredQueryParams", "type",
			})

			// Recursively clean system fields from nested widgets
			if widgets, ok := cleaned["widgets"].([]interface{}); ok {
				cleaned["widgets"] = cleanWidgetsRecursive(widgets)
			}

			apiPage := api.Page(cleaned)

			_, err := i.client.CreatePage(ctx, apiPage)

			i.mu.Lock()
			if err == nil {
				result.PagesCreated++
			} else if isConflictError(err) {
				// Fetch existing page to preserve fields like agentIdentifier
				existingPage, fetchErr := i.client.GetPage(ctx, pageID)
				if fetchErr == nil && existingPage != nil {
					// Merge agentIdentifier from existing widgets into new widgets
					if existingWidgets, ok := existingPage["widgets"].([]interface{}); ok {
						if newWidgets, ok := apiPage["widgets"].([]interface{}); ok {
							apiPage["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
						}
					}
				}

				_, updateErr := i.client.UpdatePage(ctx, pageID, apiPage)
				if updateErr != nil {
					// If update fails due to agentIdentifier requirement, retry without widgets
					// This preserves existing widget configuration for pages that require agentIdentifier
					if strings.Contains(updateErr.Error(), "agentIdentifier") {
						pageWithoutWidgets := make(api.Page)
						for k, v := range apiPage {
							if k != "widgets" {
								pageWithoutWidgets[k] = v
							}
						}
						_, retryErr := i.client.UpdatePage(ctx, pageID, pageWithoutWidgets)
						if retryErr != nil {
							i.errors.Add(retryErr, "page", pageID)
						} else {
							result.PagesUpdated++
						}
					} else {
						i.errors.Add(updateErr, "page", pageID)
					}
				} else {
					result.PagesUpdated++
				}
			} else if strings.Contains(err.Error(), "agentIdentifier") {
				// Create failed with agentIdentifier error - check if page exists and try update without widgets
				existingPage, fetchErr := i.client.GetPage(ctx, pageID)
				if fetchErr == nil && existingPage != nil {
					// Page exists, update without widgets (preserves existing widget config)
					pageWithoutWidgets := make(api.Page)
					for k, v := range apiPage {
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
					// Page doesn't exist, this is a genuine create failure
					i.errors.Add(err, "page", pageID)
				}
			} else {
				i.errors.Add(err, "page", pageID)
			}
			i.mu.Unlock()
		})
	}
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
				return
			}

			// The integration config endpoint expects {"config": {...}} wrapper
			config, ok := integration["config"].(map[string]interface{})
			if !ok || config == nil {
				// No config to update
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

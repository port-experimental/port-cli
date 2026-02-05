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

// importBlueprints imports blueprints using the three-phase approach:
// Phase 1: Create non-system blueprints with dependent fields stripped (in topological order)
// Phase 2: Update non-system blueprints to add back dependent fields
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

	// Store relations and dependent fields for phase 2
	// We strip BOTH relations and dependent fields to handle circular dependencies
	storedRelations := make(map[string]map[string]interface{})
	dependentFields := make(map[string]map[string]interface{})
	strippedBPs := make([]api.Blueprint, 0, len(nonSystemBPs))

	for _, bp := range nonSystemBPs {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
			continue
		}

		// Extract and store relations (for phase 2)
		relations := ExtractRelations(bp)
		if len(relations) > 0 {
			storedRelations[id] = relations
		}

		// Extract and store dependent fields (for phase 2)
		fields := ExtractDependentFields(bp)
		if len(fields) > 0 {
			dependentFields[id] = fields
		}

		// Strip both relations AND dependent fields for phase 1
		stripped := StripDependentFields(bp)
		stripped = StripRelations(stripped) // Also strip relations to handle cycles
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

	// Process each level sequentially (levels are in dependency order)
	// but blueprints within a level can be processed concurrently
	for levelIdx, level := range levels {
		i.reportProgress(fmt.Sprintf("Blueprints (level %d/%d)", levelIdx+1, len(levels)), createdCount, totalBPs)

		var levelMu sync.Mutex
		for _, bp := range level {
			bp := bp // capture
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

	// Handle cyclic blueprints (best effort - create them anyway)
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

	// Phase 2: Update non-system blueprints with relations and dependent fields
	blueprintsToUpdate := make(map[string]map[string]interface{})
	for id := range storedRelations {
		blueprintsToUpdate[id] = make(map[string]interface{})
	}
	for id := range dependentFields {
		if _, exists := blueprintsToUpdate[id]; !exists {
			blueprintsToUpdate[id] = make(map[string]interface{})
		}
	}

	// Merge relations and dependent fields
	for id, relations := range storedRelations {
		blueprintsToUpdate[id]["relations"] = relations
	}
	for id, fields := range dependentFields {
		for k, v := range fields {
			blueprintsToUpdate[id][k] = v
		}
	}

	if len(blueprintsToUpdate) > 0 {
		// Fetch ALL existing blueprints from target to validate dependencies
		// This is needed because successfulBPs only contains blueprints from THIS import,
		// not blueprints that already existed in the target (e.g., skipped by diff)
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

		i.reportProgress("Blueprints (adding relations)", 0, len(blueprintsToUpdate))
		updateCount := 0

		for id, fields := range blueprintsToUpdate {
			// Skip if blueprint wasn't successfully created/doesn't exist
			if !allExistingBPs[id] {
				continue
			}

			id := id
			fields := fields
			pool.Go(func() {
				err := i.updateBlueprintFields(ctx, id, fields, allExistingBPs)

				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				updateCount++
				i.reportProgress("Blueprints (adding relations)", updateCount, len(blueprintsToUpdate))
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

			cleaned := cleanSystemFields(page, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected", "after", "section", "sidebar", "requiredQueryParams", "type"})
			apiPage := api.Page(cleaned)

			_, err := i.client.CreatePage(ctx, apiPage)

			i.mu.Lock()
			if err == nil {
				result.PagesCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdatePage(ctx, pageID, apiPage)
				if updateErr != nil {
					i.errors.Add(updateErr, "page", pageID)
				} else {
					result.PagesUpdated++
				}
			} else {
				i.errors.Add(err, "page", pageID)
			}
			i.mu.Unlock()
		})
	}
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

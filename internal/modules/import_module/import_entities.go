package import_module

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/entities"
)

// Entity apply helpers (extracted from import.go): two-phase bulk upsert,
// inherited-ownership filtering, and protected-blueprint skips.

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

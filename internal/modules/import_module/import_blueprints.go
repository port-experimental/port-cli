package import_module

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
	systemblueprints "github.com/port-experimental/port-cli/internal/modules/system_blueprints"
)

// importBlueprints imports blueprints using a multi-phase approach:
// Phase 1: Create non-system blueprints with relations and dependent fields stripped
// Phase 2a: Add relations back to all blueprints
// Phase 2b: Add calculationProperties (self-contained, no cross-blueprint dependencies)
// Phase 2c: Add mirrorProperties (depend on relations existing)
// Phase 2d: Add aggregationProperties (depend on properties existing on OTHER blueprints)
// Phase 3: Update system blueprints
func (i *Importer) importBlueprints(ctx context.Context, blueprints []api.Blueprint, result *Result, errorHandlingOpts ...ErrorHandlingOptions) error {
	var errorHandling ErrorHandlingOptions
	if len(errorHandlingOpts) > 0 {
		errorHandling = errorHandlingOpts[0]
	}
	errorHandling.AddWarning = i.handledErrorWarningCallback(result, errorHandling.AddWarning)
	updater := NewBlueprintUpdater(i.client, errorHandling)

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
				created, updated, err := i.createOrUpdateBlueprint(ctx, bp, result, updater)

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
				created, updated, err := i.createOrUpdateBlueprint(ctx, bp, result, updater)

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

	// Fetch ALL existing blueprints from target for relation-target validation.
	// This must not be limited to blueprints this run touched: a scoped migration
	// may omit a relation target from its diff even though it already exists in
	// the target org.
	allExistingBPs := make(map[string]bool)
	for id := range successfulBPs {
		allExistingBPs[id] = true
	}
	for _, id := range CommonSystemBlueprints() {
		allExistingBPs[id] = true
	}
	if len(storedRelations) > 0 {
		targetBlueprints, err := i.client.GetBlueprints(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch target blueprints for relation validation: %w", err)
		}
		for _, bp := range targetBlueprints {
			if id, ok := bp["identifier"].(string); ok && id != "" {
				allExistingBPs[id] = true
			}
		}
	}

	// Phase 2a: Add relations back to all blueprints
	if len(storedRelations) > 0 {
		i.reportProgress("Blueprints (adding relations)", 0, len(storedRelations))
		validRelations := make(map[string]map[string]interface{}, len(storedRelations))
		for id, relations := range storedRelations {
			if !allExistingBPs[id] {
				continue
			}
			missing := ValidateRelationTargets(api.Blueprint{"relations": relations}, allExistingBPs)
			if len(missing) > 0 {
				i.errors.Add(fmt.Errorf("Blueprint %s (relations): missing target blueprints: %v", id, missing), "blueprint", id)
				continue
			}
			validRelations[id] = relations
		}
		count := 0
		for id, relations := range validRelations {
			id, relations := id, relations
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"relations": relations}, result, updater)
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
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"calculationProperties": calcProps}, result, updater)
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

	// failedMirrorProps collects Phase 2c failures for a second pass after Phase 2d,
	// because some mirror props reference agg props that don't exist until Phase 2d.
	failedMirrorProps := make(map[string]map[string]interface{})
	var failedMirrorMu sync.Mutex

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
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"mirrorProperties": mirrorProps}, result, updater)
				if err != nil {
					failedMirrorMu.Lock()
					failedMirrorProps[id] = mirrorProps
					failedMirrorMu.Unlock()
				}
				i.mu.Lock()
				count++
				i.reportProgress("Blueprints (adding mirrorProperties)", count, len(storedMirrorProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	// Phase 2d: Add aggregationProperties in topological order so that agg props
	// referencing another blueprint's agg props are applied after their dependencies
	// (e.g. businessApplication.codeQualityBugs must run after component.codeQualityBugs).
	// Failures are retried after Phase 3 (system blueprint updates) because some agg props
	// use path filters through system blueprint relations (e.g. _rule_result._githubBranch)
	// that don't exist until Phase 3 applies the system blueprint schema.
	failedAggProps := make(map[string]map[string]interface{})
	var failedAggMu sync.Mutex

	if len(storedAggProps) > 0 {
		levels := TopologicalSortAggProps(storedAggProps)
		for levelIdx, level := range levels {
			label := fmt.Sprintf("Blueprints (adding aggregationProperties, level %d/%d)", levelIdx+1, len(levels))
			i.reportProgress(label, 0, len(level))
			count := 0
			for _, id := range level {
				if !allExistingBPs[id] {
					continue
				}
				id, aggProps := id, storedAggProps[id]
				pool.Go(func() {
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"aggregationProperties": aggProps}, result, updater)
					if err != nil {
						failedAggMu.Lock()
						failedAggProps[id] = aggProps
						failedAggMu.Unlock()
					}
					i.mu.Lock()
					count++
					i.reportProgress(label, count, len(level))
					i.mu.Unlock()
				})
			}
			pool.Wait()
		}
	}

	// Phase 2e: Retry mirror properties that failed in Phase 2c. Some mirror props
	// reference aggregation properties on related blueprints that now exist after Phase 2d.
	if len(failedMirrorProps) > 0 {
		i.reportProgress("Blueprints (adding mirrorProperties, pass 2/2)", 0, len(failedMirrorProps))
		count := 0
		for id, mirrorProps := range failedMirrorProps {
			if !allExistingBPs[id] {
				continue
			}
			id, mirrorProps := id, mirrorProps
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"mirrorProperties": mirrorProps}, result, updater)
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding mirrorProperties, pass 2/2)", count, len(failedMirrorProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
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
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"ownership": ownership}, result, updater)
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
					err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"ownership": ownership}, result, updater)
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
				_, updated, err := i.createOrUpdateBlueprint(ctx, bp, result, updater)

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

	// Phase 4: Retry aggregationProperties that failed in Phase 2d. Some agg props
	// reference path filters through system blueprint relations (e.g. _rule_result._githubBranch)
	// that only exist after Phase 3 updates the system blueprint schema.
	if len(failedAggProps) > 0 {
		i.reportProgress("Blueprints (adding aggregationProperties, pass 2/2)", 0, len(failedAggProps))
		count := 0
		for id, aggProps := range failedAggProps {
			if !allExistingBPs[id] {
				continue
			}
			id, aggProps := id, aggProps
			pool.Go(func() {
				err := i.updateBlueprintFieldsDirect(ctx, id, map[string]interface{}{"aggregationProperties": aggProps}, result, updater)
				i.mu.Lock()
				if err != nil {
					i.errors.Add(err, "blueprint", id)
				}
				count++
				i.reportProgress("Blueprints (adding aggregationProperties, pass 2/2)", count, len(failedAggProps))
				i.mu.Unlock()
			})
		}
		pool.Wait()
	}

	if len(result.IgnoredRuleResultTargetRelationKeys) > 0 {
		sort.Strings(result.IgnoredRuleResultTargetRelationKeys)
	}

	return nil
}

func (i *Importer) handledErrorWarningCallback(result *Result, existing func(string)) func(string) {
	return func(message string) {
		i.mu.Lock()
		defer i.mu.Unlock()
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:    "handled_error",
			Message: message,
		})
		if existing != nil {
			existing(message)
		}
	}
}

// createOrUpdateBlueprint creates or updates a single blueprint.
// Returns (created, updated, error).
func (i *Importer) createOrUpdateBlueprint(ctx context.Context, bp api.Blueprint, result *Result, updater *BlueprintUpdater) (bool, bool, error) {
	id, _ := bp["identifier"].(string)
	sendBP := bp
	if rels, ok := bp["relations"].(map[string]interface{}); ok && len(rels) > 0 {
		kept, ignored := systemblueprints.FilterManagedRelations(id, rels)
		i.recordRuleResultIgnoredRelations(ignored, result)
		if len(ignored) > 0 {
			sendBP = systemblueprints.BlueprintWithRelations(bp, kept)
		}
	}

	_, err := i.client.CreateBlueprint(ctx, sendBP)
	if err == nil {
		return true, false, nil
	}

	if isConflictError(err) {
		var updateErr error
		if systemblueprints.PrefersPatchUpdate(id) {
			updateErr = updater.Update(ctx, id, sendBP, BlueprintUpdatePATCH)
		} else {
			// Fetch existing blueprint and merge to avoid destroying fields
			// (like relations) that were stripped for Phase 1 ordering.
			existing, fetchErr := i.client.GetBlueprint(ctx, id)
			if fetchErr != nil {
				return false, false, fetchErr
			}
			for k, v := range sendBP {
				existing[k] = v
			}
			existing = api.Blueprint(cleanSystemFields(map[string]interface{}(existing),
				[]string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}))
			updateErr = updater.Update(ctx, id, existing, BlueprintUpdatePUT)
		}
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

	existing = api.Blueprint(cleanSystemFields(map[string]interface{}(existing),
		[]string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}))

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
func (i *Importer) updateBlueprintFieldsDirect(ctx context.Context, id string, fields map[string]interface{}, result *Result, updater *BlueprintUpdater) error {
	existing, err := i.client.GetBlueprint(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to fetch blueprint: %w", err)
	}

	for k, v := range fields {
		if k == "relations" {
			if newMap, ok := v.(map[string]interface{}); ok {
				kept, ignored := systemblueprints.FilterManagedRelations(id, newMap)
				i.recordRuleResultIgnoredRelations(ignored, result)
				if len(kept) == 0 {
					continue
				}
				v = kept
			}
		}
		if newMap, ok := v.(map[string]interface{}); ok {
			if existingMap, ok := existing[k].(map[string]interface{}); ok {
				for itemKey, itemVal := range newMap {
					existingMap[itemKey] = itemVal
				}
				existing[k] = existingMap
			} else {
				existing[k] = v
			}
		} else {
			existing[k] = v
		}
	}

	existing = api.Blueprint(cleanSystemFields(map[string]interface{}(existing),
		[]string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}))

	var updateErr error
	if systemblueprints.PrefersPatchUpdate(id) {
		updateErr = updater.Update(ctx, id, existing, BlueprintUpdatePATCH)
	} else {
		updateErr = updater.Update(ctx, id, existing, BlueprintUpdatePUT)
	}
	if updateErr != nil {
		return fmt.Errorf("failed to update blueprint fields: %w", updateErr)
	}

	return nil
}

package import_module

import (
	"regexp"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// DependentFields are blueprint fields that may reference other blueprints.
// Note: "relations" is NOT included because the relation schema must be created
// with the blueprint - you can't add relations to a blueprint after creation.
// The topological sort ensures relation targets exist before blueprint creation.
var DependentFields = []string{
	"mirrorProperties",
	"calculationProperties",
	"aggregationProperties",
}

// ExtractRelations extracts the relations field from a blueprint.
func ExtractRelations(bp api.Blueprint) map[string]interface{} {
	relations, ok := bp["relations"].(map[string]interface{})
	if !ok {
		return nil
	}
	return relations
}

// StripRelations creates a copy of the blueprint without the relations field.
func StripRelations(bp api.Blueprint) api.Blueprint {
	stripped := make(api.Blueprint)
	for k, v := range bp {
		if k != "relations" {
			stripped[k] = v
		}
	}
	return stripped
}

// ExtractDependentFields extracts all dependent fields from a blueprint.
// Returns a map of field name to field value for fields that were present.
func ExtractDependentFields(bp api.Blueprint) map[string]interface{} {
	extracted := make(map[string]interface{})
	for _, field := range DependentFields {
		if val, ok := bp[field]; ok && val != nil {
			extracted[field] = val
		}
	}
	return extracted
}

// StripDependentFields creates a copy of the blueprint without any dependent fields.
func StripDependentFields(bp api.Blueprint) api.Blueprint {
	stripped := make(api.Blueprint)
	fieldSet := make(map[string]bool)
	for _, f := range DependentFields {
		fieldSet[f] = true
	}

	for k, v := range bp {
		if !fieldSet[k] {
			stripped[k] = v
		}
	}
	return stripped
}

// IsRelationError checks if an error is related to missing relation targets.
// This detects common error patterns from the Port API when a relation target doesn't exist.
func IsRelationError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// More specific error patterns for relation issues to avoid false positives
	relationErrorPatterns := []string{
		"relation target",
		"target blueprint",
		"blueprint not found",
		"invalid relation target",
		"missing blueprint",
		"relation does not exist",
		"target does not exist",
		"cannot find blueprint",
		"blueprint identifier",
		"relation reference",
	}

	// Also check for generic patterns but only if they appear in context
	genericPatterns := []string{
		"relation",
		"target",
	}

	// First check specific patterns
	for _, pattern := range relationErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Then check generic patterns but require additional context
	for _, pattern := range genericPatterns {
		if strings.Contains(errStr, pattern) {
			if strings.Contains(errStr, "blueprint") || strings.Contains(errStr, "relation") {
				return true
			}
		}
	}

	return false
}

// GetAllDependencies extracts all blueprint identifiers that this blueprint depends on.
// This includes targets from relations, mirrorProperties, calculationProperties, and aggregationProperties.
func GetAllDependencies(bp api.Blueprint) []string {
	deps := make(map[string]bool)

	// Get relation targets
	for _, target := range getRelationTargets(bp) {
		deps[target] = true
	}

	// Get mirror property targets
	for _, target := range getMirrorPropertyTargets(bp) {
		deps[target] = true
	}

	// Get calculation property targets
	for _, target := range getCalculationPropertyTargets(bp) {
		deps[target] = true
	}

	// Get aggregation property targets
	for _, target := range getAggregationPropertyTargets(bp) {
		deps[target] = true
	}

	// Convert to slice
	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result
}

// getRelationTargets extracts all relation target blueprint identifiers from a blueprint.
func getRelationTargets(bp api.Blueprint) []string {
	targets := []string{}
	relations, ok := bp["relations"].(map[string]interface{})
	if !ok {
		return targets
	}

	for _, relation := range relations {
		relationMap, ok := relation.(map[string]interface{})
		if !ok {
			continue
		}

		target, ok := relationMap["target"].(string)
		if ok && target != "" {
			targets = append(targets, target)
		}
	}

	return targets
}

// getMirrorPropertyTargets extracts blueprint identifiers from mirror properties.
// Mirror properties have a "path" field like "relationName.propertyName" where
// relationName refers to a relation whose target is another blueprint.
func getMirrorPropertyTargets(bp api.Blueprint) []string {
	targets := []string{}
	mirrorProps, ok := bp["mirrorProperties"].(map[string]interface{})
	if !ok {
		return targets
	}

	// Build map of relation name -> target blueprint
	relationTargets := make(map[string]string)
	relations, ok := bp["relations"].(map[string]interface{})
	if ok {
		for relName, rel := range relations {
			relMap, ok := rel.(map[string]interface{})
			if !ok {
				continue
			}
			if target, ok := relMap["target"].(string); ok {
				relationTargets[relName] = target
			}
		}
	}

	// For each mirror property, find the target blueprint via the relation
	for _, prop := range mirrorProps {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		path, ok := propMap["path"].(string)
		if !ok || path == "" {
			continue
		}

		// Path format: "relationName.propertyName" or "relationName.nestedRelation.property"
		parts := strings.Split(path, ".")
		if len(parts) >= 1 {
			relName := parts[0]
			if target, ok := relationTargets[relName]; ok {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// getCalculationPropertyTargets extracts blueprint identifiers from calculation properties.
// Calculation formulas may reference other blueprints via relations.
func getCalculationPropertyTargets(bp api.Blueprint) []string {
	targets := []string{}
	calcProps, ok := bp["calculationProperties"].(map[string]interface{})
	if !ok {
		return targets
	}

	// Build map of relation name -> target blueprint
	relationTargets := make(map[string]string)
	relations, ok := bp["relations"].(map[string]interface{})
	if ok {
		for relName, rel := range relations {
			relMap, ok := rel.(map[string]interface{})
			if !ok {
				continue
			}
			if target, ok := relMap["target"].(string); ok {
				relationTargets[relName] = target
			}
		}
	}

	// Regex to find relation references in calculations
	// Format: relations.relationName.property or .relations.relationName
	relPattern := regexp.MustCompile(`\.?relations\.(\w+)`)

	for _, prop := range calcProps {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		calc, ok := propMap["calculation"].(string)
		if !ok || calc == "" {
			continue
		}

		// Find all relation references in the calculation
		matches := relPattern.FindAllStringSubmatch(calc, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				relName := match[1]
				if target, ok := relationTargets[relName]; ok {
					targets = append(targets, target)
				}
			}
		}
	}

	return targets
}

// getAggregationPropertyTargets extracts blueprint identifiers from aggregation properties.
func getAggregationPropertyTargets(bp api.Blueprint) []string {
	targets := []string{}
	aggProps, ok := bp["aggregationProperties"].(map[string]interface{})
	if !ok {
		return targets
	}

	for _, prop := range aggProps {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		// Aggregation properties have a "target" field pointing to the blueprint to aggregate from
		target, ok := propMap["target"].(string)
		if ok && target != "" {
			targets = append(targets, target)
		}
	}

	return targets
}

// CreateBlueprintWithRelations creates a blueprint payload with only the relations field.
// This is used for the second pass update.
func CreateBlueprintWithRelations(identifier string, relations map[string]interface{}) api.Blueprint {
	return api.Blueprint{
		"identifier": identifier,
		"relations":  relations,
	}
}

// ValidateRelationTargets checks if all relation targets exist in the provided blueprint set.
func ValidateRelationTargets(bp api.Blueprint, existingBlueprints map[string]bool) []string {
	missing := []string{}
	targets := getRelationTargets(bp)

	for _, target := range targets {
		if !existingBlueprints[target] {
			missing = append(missing, target)
		}
	}

	return missing
}

// ValidateAllDependencies checks if all dependencies exist in the provided blueprint set.
func ValidateAllDependencies(bp api.Blueprint, existingBlueprints map[string]bool) []string {
	missing := []string{}
	deps := GetAllDependencies(bp)

	for _, dep := range deps {
		if !existingBlueprints[dep] {
			missing = append(missing, dep)
		}
	}

	return missing
}

// IsSystemBlueprint returns true if the blueprint identifier indicates a system blueprint.
// System blueprints start with underscore (_user, _team, _rule, etc.)
func IsSystemBlueprint(identifier string) bool {
	return strings.HasPrefix(identifier, "_")
}

// TopologicalSort sorts blueprints in dependency order using Kahn's algorithm.
// Returns blueprints grouped by dependency level (level 0 has no dependencies, etc.)
// Also returns any blueprints involved in cycles (which couldn't be sorted).
func TopologicalSort(blueprints []api.Blueprint, existingBlueprints map[string]bool) ([][]api.Blueprint, []api.Blueprint) {
	// Build identifier -> blueprint map
	bpMap := make(map[string]api.Blueprint)
	for _, bp := range blueprints {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
			continue
		}
		bpMap[id] = bp
	}

	// Build dependency graph
	// inDegree[id] = number of dependencies this blueprint has (that are in our set)
	// dependents[id] = blueprints that depend on this one
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for id := range bpMap {
		inDegree[id] = 0
		dependents[id] = []string{}
	}

	for id, bp := range bpMap {
		deps := GetAllDependencies(bp)
		for _, dep := range deps {
			// Only count dependencies that are in our blueprint set
			// (not system blueprints or already existing ones)
			if _, inSet := bpMap[dep]; inSet {
				inDegree[id]++
				dependents[dep] = append(dependents[dep], id)
			}
		}
	}

	// Kahn's algorithm
	var levels [][]api.Blueprint
	processed := make(map[string]bool)

	for len(processed) < len(bpMap) {
		// Find all blueprints with no remaining dependencies
		var currentLevel []api.Blueprint
		for id, degree := range inDegree {
			if degree == 0 && !processed[id] {
				currentLevel = append(currentLevel, bpMap[id])
				processed[id] = true
			}
		}

		// If no blueprints can be processed, we have a cycle
		if len(currentLevel) == 0 {
			break
		}

		levels = append(levels, currentLevel)

		// Reduce in-degree for dependents
		for _, bp := range currentLevel {
			id := bp["identifier"].(string)
			for _, dependent := range dependents[id] {
				inDegree[dependent]--
			}
		}
	}

	// Collect any blueprints in cycles (not processed)
	var cyclic []api.Blueprint
	for id, bp := range bpMap {
		if !processed[id] {
			cyclic = append(cyclic, bp)
		}
	}

	return levels, cyclic
}

// FlattenLevels converts leveled blueprints to a flat slice in dependency order.
func FlattenLevels(levels [][]api.Blueprint) []api.Blueprint {
	var result []api.Blueprint
	for _, level := range levels {
		result = append(result, level...)
	}
	return result
}

// SeparateSystemBlueprints splits blueprints into system and non-system blueprints.
func SeparateSystemBlueprints(blueprints []api.Blueprint) (nonSystem, system []api.Blueprint) {
	for _, bp := range blueprints {
		id, ok := bp["identifier"].(string)
		if !ok || id == "" {
			continue
		}
		if IsSystemBlueprint(id) {
			system = append(system, bp)
		} else {
			nonSystem = append(nonSystem, bp)
		}
	}
	return
}

// BuildExistingBlueprintsSet creates a set of blueprint identifiers that are considered "existing".
// This includes system blueprints and any explicitly provided identifiers.
func BuildExistingBlueprintsSet(additionalExisting []string) map[string]bool {
	existing := make(map[string]bool)

	// Add provided existing blueprints
	for _, id := range additionalExisting {
		existing[id] = true
	}

	return existing
}

// CommonSystemBlueprints returns identifiers of commonly available system blueprints.
func CommonSystemBlueprints() []string {
	return []string{
		"_user",
		"_team",
		"_rule",
	}
}

// ExtractEntityRelations extracts the relations field from an entity.
func ExtractEntityRelations(entity api.Entity) map[string]interface{} {
	relations, ok := entity["relations"].(map[string]interface{})
	if !ok {
		return nil
	}
	return relations
}

// StripEntityRelations creates a copy of the entity without the relations field.
func StripEntityRelations(entity api.Entity) api.Entity {
	stripped := make(api.Entity)
	for k, v := range entity {
		if k != "relations" {
			stripped[k] = v
		}
	}
	return stripped
}

// HasEntityRelations checks if an entity has any relation values set.
func HasEntityRelations(entity api.Entity) bool {
	relations, ok := entity["relations"].(map[string]interface{})
	if !ok || relations == nil {
		return false
	}
	return len(relations) > 0
}

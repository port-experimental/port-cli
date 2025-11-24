package import_module

import (
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

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

// buildDependencyGraph builds a dependency graph from blueprints.
// Returns a map where keys are blueprint identifiers and values are slices of blueprint identifiers they depend on.
func buildDependencyGraph(blueprints []api.Blueprint) map[string][]string {
	graph := make(map[string][]string)

	for _, bp := range blueprints {
		identifier, ok := bp["identifier"].(string)
		if !ok || identifier == "" {
			continue
		}

		deps := []string{}
		relations, ok := bp["relations"].(map[string]interface{})
		if ok {
			for _, relation := range relations {
				relationMap, ok := relation.(map[string]interface{})
				if !ok {
					continue
				}

				target, ok := relationMap["target"].(string)
				if ok && target != "" {
					deps = append(deps, target)
				}
			}
		}

		if len(deps) > 0 {
			graph[identifier] = deps
		}
	}

	return graph
}

// IsRelationError checks if an error is related to missing relation targets.
// This detects common error patterns from the Port API when a relation target doesn't exist.
func IsRelationError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// More specific error patterns for relation issues to avoid false positives
	// Check for combinations that are more likely to be relation-specific
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
			// Additional validation: check if error mentions blueprint or relation context
			if strings.Contains(errStr, "blueprint") || strings.Contains(errStr, "relation") {
				return true
			}
		}
	}

	return false
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

// hasRelations checks if a blueprint has any relations defined.
func hasRelations(bp api.Blueprint) bool {
	relations, ok := bp["relations"].(map[string]interface{})
	if !ok {
		return false
	}
	return len(relations) > 0
}

// CreateBlueprintWithRelations creates a blueprint payload with only the relations field.
// This is used for the second pass update.
func CreateBlueprintWithRelations(identifier string, relations map[string]interface{}) api.Blueprint {
	return api.Blueprint{
		"identifier": identifier,
		"relations":  relations,
	}
}

// mergeRelations merges relations back into a blueprint.
// This creates a new blueprint with relations added.
func mergeRelations(bp api.Blueprint, relations map[string]interface{}) api.Blueprint {
	merged := make(api.Blueprint)
	for k, v := range bp {
		merged[k] = v
	}
	if relations != nil && len(relations) > 0 {
		merged["relations"] = relations
	}
	return merged
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

// formatRelationError creates a formatted error message for relation issues.
func formatRelationError(identifier string, missingTargets []string) string {
	if len(missingTargets) == 0 {
		return fmt.Sprintf("Blueprint %s: relation error", identifier)
	}
	return fmt.Sprintf("Blueprint %s: missing relation targets: %v", identifier, missingTargets)
}


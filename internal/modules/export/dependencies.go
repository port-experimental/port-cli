package export

import "github.com/port-experimental/port-cli/internal/api"

// ResolveBlueprintDependencies returns selectedBlueprints plus any blueprint
// relation targets required by the selection.
func ResolveBlueprintDependencies(allBlueprints, selectedBlueprints []api.Blueprint) []api.Blueprint {
	selectedIDs := make(map[string]bool)
	allBlueprintsMap := make(map[string]api.Blueprint)

	for _, bp := range allBlueprints {
		if identifier, ok := bp["identifier"].(string); ok {
			allBlueprintsMap[identifier] = bp
		}
	}

	for _, bp := range selectedBlueprints {
		if identifier, ok := bp["identifier"].(string); ok {
			selectedIDs[identifier] = true
		}
	}

	result := make([]api.Blueprint, len(selectedBlueprints))
	copy(result, selectedBlueprints)

	toCheck := make([]string, 0, len(selectedIDs))
	for id := range selectedIDs {
		toCheck = append(toCheck, id)
	}

	checked := make(map[string]bool)

	for len(toCheck) > 0 {
		blueprintID := toCheck[len(toCheck)-1]
		toCheck = toCheck[:len(toCheck)-1]

		if checked[blueprintID] {
			continue
		}
		checked[blueprintID] = true

		blueprint, ok := allBlueprintsMap[blueprintID]
		if !ok {
			continue
		}

		relations, ok := blueprint["relations"].(map[string]interface{})
		if !ok {
			continue
		}

		for _, relation := range relations {
			relationMap, ok := relation.(map[string]interface{})
			if !ok {
				continue
			}

			target, ok := relationMap["target"].(string)
			if !ok || target == "" {
				continue
			}

			if !selectedIDs[target] {
				if depBlueprint, exists := allBlueprintsMap[target]; exists {
					result = append(result, depBlueprint)
					selectedIDs[target] = true
					toCheck = append(toCheck, target)
				}
			}
		}
	}

	return result
}

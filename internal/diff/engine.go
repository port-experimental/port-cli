package diff

import (
	"reflect"
	"sort"

	"github.com/port-experimental/port-cli/internal/resources"
)

// Config controls how two resource slices are compared.
type Config struct {
	Kind resources.ResourceKind
	// Equal compares two items with the same identity. Defaults to registry normalization.
	Equal func(source, target map[string]interface{}) bool
	// FieldDiff returns field-level differences for compare output. Defaults to excluded-field walk.
	FieldDiff func(source, target map[string]interface{}) []FieldChange
}

// DiffMaps compares source (baseline) and target (desired) resource slices.
func DiffMaps(source, target []map[string]interface{}, cfg Config) Result {
	desc := resources.MustGet(cfg.Kind)
	equal := cfg.Equal
	if equal == nil {
		equal = func(a, b map[string]interface{}) bool {
			return resources.ResourcesEqual(a, b, desc.ServerManagedFields)
		}
	}
	fieldDiff := cfg.FieldDiff
	if fieldDiff == nil {
		fieldDiff = diffFieldsExcluded
	}

	result := Result{}
	sourceMap := indexByIdentity(source, desc.Identity)
	targetMap := indexByIdentity(target, desc.Identity)

	for id, targetItem := range targetMap {
		if _, exists := sourceMap[id]; !exists {
			result.Added = append(result.Added, ItemChange{
				Identifier: id,
				Target:     targetItem,
			})
		}
	}

	for id, sourceItem := range sourceMap {
		if _, exists := targetMap[id]; !exists {
			result.Removed = append(result.Removed, ItemChange{
				Identifier: id,
				Source:     sourceItem,
			})
		}
	}

	for id, sourceItem := range sourceMap {
		targetItem, exists := targetMap[id]
		if !exists {
			continue
		}
		if equal(sourceItem, targetItem) {
			continue
		}
		result.Modified = append(result.Modified, ItemChange{
			Identifier: id,
			Source:     sourceItem,
			Target:     targetItem,
			Fields:     fieldDiff(sourceItem, targetItem),
		})
	}

	sortItemChanges(result.Added)
	sortItemChanges(result.Modified)
	sortItemChanges(result.Removed)

	result.Summary = Summary{
		Added:    len(result.Added),
		Modified: len(result.Modified),
		Removed:  len(result.Removed),
	}
	return result
}

func indexByIdentity(items []map[string]interface{}, identity resources.IdentityFunc) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		if id, ok := identity(item); ok {
			result[id] = item
		}
	}
	return result
}

func sortItemChanges(changes []ItemChange) {
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Identifier < changes[j].Identifier
	})
}

func diffFieldsExcluded(source, target map[string]interface{}) []FieldChange {
	return diffFields(source, target, "", resources.CompareExcludedFields)
}

func diffFields(source, target map[string]interface{}, prefix string, excluded map[string]bool) []FieldChange {
	var diffs []FieldChange

	allKeys := make(map[string]bool)
	for k := range source {
		allKeys[k] = true
	}
	for k := range target {
		allKeys[k] = true
	}

	for key := range allKeys {
		if excluded[key] {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		sourceVal, sourceExists := source[key]
		targetVal, targetExists := target[key]

		switch {
		case !sourceExists:
			diffs = append(diffs, FieldChange{Path: path, SourceValue: nil, TargetValue: targetVal})
		case !targetExists:
			diffs = append(diffs, FieldChange{Path: path, SourceValue: sourceVal, TargetValue: nil})
		case !reflect.DeepEqual(sourceVal, targetVal):
			sourceMap, sourceIsMap := sourceVal.(map[string]interface{})
			targetMap, targetIsMap := targetVal.(map[string]interface{})
			if sourceIsMap && targetIsMap {
				diffs = append(diffs, diffFields(sourceMap, targetMap, path, excluded)...)
			} else {
				diffs = append(diffs, FieldChange{Path: path, SourceValue: sourceVal, TargetValue: targetVal})
			}
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})
	return diffs
}

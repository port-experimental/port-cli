package diff

import (
	"github.com/port-experimental/port-cli/internal/resources"
)

// ImportConfig controls import-style diffing (current vs desired).
type ImportConfig struct {
	Kind resources.ResourceKind
	// Equal compares desired vs current for items with the same identity.
	// Defaults to registry normalization when nil.
	Equal func(desired, current map[string]interface{}) bool
	// ShouldSkip marks a desired item as skip without create/update.
	ShouldSkip func(desired map[string]interface{}) bool
	// IgnoreMissing drops desired items not present in current instead of creating them.
	IgnoreMissing bool
}

// ImportOutcome holds create/update/skip buckets for import dry-run.
type ImportOutcome[T ~map[string]interface{}] struct {
	ToCreate []T
	ToUpdate []T
	ToSkip   []T
}

// DiffForImport compares current org state with desired import data.
// Items only in current are ignored (import does not delete resources).
func DiffForImport[T ~map[string]interface{}](current, desired []T, cfg ImportConfig) ImportOutcome[T] {
	desc := resources.MustGet(cfg.Kind)
	equal := cfg.Equal
	if equal == nil {
		equal = func(desired, current map[string]interface{}) bool {
			return resources.ResourcesEqual(desired, current, desc.ServerManagedFields)
		}
	}

	currentMap := make(map[string]T, len(current))
	for _, item := range current {
		if id, ok := desc.Identity(item); ok {
			currentMap[id] = item
		}
	}

	outcome := ImportOutcome[T]{}
	for _, item := range desired {
		m := map[string]interface{}(item)
		if cfg.ShouldSkip != nil && cfg.ShouldSkip(m) {
			outcome.ToSkip = append(outcome.ToSkip, item)
			continue
		}

		id, ok := desc.Identity(m)
		if !ok {
			continue
		}

		currentItem, exists := currentMap[id]
		if !exists {
			if cfg.IgnoreMissing {
				continue
			}
			outcome.ToCreate = append(outcome.ToCreate, item)
		} else if !equal(m, currentItem) {
			outcome.ToUpdate = append(outcome.ToUpdate, item)
		} else {
			outcome.ToSkip = append(outcome.ToSkip, item)
		}
	}
	return outcome
}

// ImportSummary returns create/update counts from an import outcome.
func ImportSummary[T ~map[string]interface{}](outcome ImportOutcome[T]) Summary {
	return Summary{
		Added:    len(outcome.ToCreate),
		Modified: len(outcome.ToUpdate),
		Removed:  0,
	}
}

// PermissionChange is a permissions entry that should be applied on import.
type PermissionChange struct {
	Identifier string
	Desired    map[string]interface{}
}

// DiffPermissions compares current and desired permission maps keyed by resource identifier.
func DiffPermissions(current, desired map[string]map[string]interface{}, kind resources.ResourceKind) []PermissionChange {
	currentSlice := permissionMapToSlice(current)
	desiredSlice := permissionMapToSlice(desired)
	outcome := DiffForImport(currentSlice, desiredSlice, ImportConfig{Kind: kind})
	changes := make([]PermissionChange, 0, len(outcome.ToCreate)+len(outcome.ToUpdate))
	for _, item := range outcome.ToCreate {
		changes = append(changes, permissionChangeFromMap(item))
	}
	for _, item := range outcome.ToUpdate {
		changes = append(changes, permissionChangeFromMap(item))
	}
	return changes
}

func permissionMapToSlice(perms map[string]map[string]interface{}) []map[string]interface{} {
	if len(perms) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(perms))
	for id, p := range perms {
		entry := make(map[string]interface{}, len(p)+1)
		for k, v := range p {
			entry[k] = v
		}
		entry["identifier"] = id
		result = append(result, entry)
	}
	return result
}

func permissionChangeFromMap(item map[string]interface{}) PermissionChange {
	id, _ := item["identifier"].(string)
	desired := make(map[string]interface{}, len(item)-1)
	for k, v := range item {
		if k == "identifier" {
			continue
		}
		desired[k] = v
	}
	return PermissionChange{Identifier: id, Desired: desired}
}

package diff

import (
	"github.com/port-experimental/port-cli/internal/resources"
)

// ImportConfig controls import-style diffing (current vs desired).
type ImportConfig struct {
	Kind resources.ResourceKind
	// Equal defaults to registry normalization when nil.
	Equal func(current, desired map[string]interface{}) bool
	// ShouldSkip marks a desired item as skip without create/update.
	ShouldSkip func(desired map[string]interface{}) bool
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
		equal = func(current, desired map[string]interface{}) bool {
			return resources.ResourcesEqual(current, desired, desc.ServerManagedFields)
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

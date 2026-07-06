package compare

import (
	"github.com/port-experimental/port-cli/internal/diff"
)

func fromDiffResult(r diff.Result) ResourceDiff {
	return ResourceDiff{
		Summary: DiffSummary{
			Added:    r.Summary.Added,
			Modified: r.Summary.Modified,
			Removed:  r.Summary.Removed,
		},
		Added:    toResourceChanges(r.Added, false),
		Modified: toResourceChanges(r.Modified, true),
		Removed:  toResourceChanges(r.Removed, false),
	}
}

func toResourceChanges(items []diff.ItemChange, includeFieldDiffs bool) []ResourceChange {
	changes := make([]ResourceChange, len(items))
	for i, item := range items {
		change := ResourceChange{
			Identifier: item.Identifier,
			SourceData: item.Source,
			TargetData: item.Target,
		}
		if includeFieldDiffs {
			change.FieldDiffs = make([]FieldDiff, len(item.Fields))
			for j, f := range item.Fields {
				change.FieldDiffs[j] = FieldDiff{
					Path:        f.Path,
					SourceValue: f.SourceValue,
					TargetValue: f.TargetValue,
				}
			}
		}
		changes[i] = change
	}
	return changes
}

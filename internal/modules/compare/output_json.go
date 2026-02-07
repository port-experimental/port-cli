package compare

import (
	"encoding/json"
	"io"
)

// JSONOutput represents the JSON output structure.
type JSONOutput struct {
	Source    string                      `json:"source"`
	Target    string                      `json:"target"`
	Timestamp string                      `json:"timestamp"`
	Summary   JSONSummary                 `json:"summary"`
	Identical bool                        `json:"identical"`
	Diffs     map[string]JSONResourceDiff `json:"differences"`
}

// JSONSummary represents the overall summary.
type JSONSummary struct {
	TotalAdded    int `json:"total_added"`
	TotalModified int `json:"total_modified"`
	TotalRemoved  int `json:"total_removed"`
}

// JSONResourceDiff represents differences for a resource type in JSON.
type JSONResourceDiff struct {
	Added    []JSONResourceChange `json:"added,omitempty"`
	Modified []JSONResourceChange `json:"modified,omitempty"`
	Removed  []JSONResourceChange `json:"removed,omitempty"`
}

// JSONResourceChange represents a single changed resource in JSON.
type JSONResourceChange struct {
	Identifier string                 `json:"identifier"`
	Data       map[string]interface{} `json:"data,omitempty"`
	SourceData map[string]interface{} `json:"source,omitempty"`
	TargetData map[string]interface{} `json:"target,omitempty"`
	Changes    []JSONFieldDiff        `json:"changes,omitempty"`
}

// JSONFieldDiff represents a field-level diff in JSON.
type JSONFieldDiff struct {
	Path        string      `json:"path"`
	SourceValue interface{} `json:"source_value"`
	TargetValue interface{} `json:"target_value"`
}

// JSONFormatter formats comparison results as JSON.
type JSONFormatter struct {
	w io.Writer
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

// Format outputs the comparison result as JSON.
func (f *JSONFormatter) Format(result *CompareResult) error {
	output := JSONOutput{
		Source:    result.Source,
		Target:    result.Target,
		Timestamp: result.Timestamp,
		Identical: result.Identical,
		Summary: JSONSummary{
			TotalAdded:    f.sumAdded(result),
			TotalModified: f.sumModified(result),
			TotalRemoved:  f.sumRemoved(result),
		},
		Diffs: make(map[string]JSONResourceDiff),
	}

	// Add each resource type
	f.addResourceDiff(output.Diffs, "blueprints", result.Blueprints)
	f.addResourceDiff(output.Diffs, "actions", result.Actions)
	f.addResourceDiff(output.Diffs, "scorecards", result.Scorecards)
	f.addResourceDiff(output.Diffs, "pages", result.Pages)
	f.addResourceDiff(output.Diffs, "integrations", result.Integrations)
	f.addResourceDiff(output.Diffs, "teams", result.Teams)
	f.addResourceDiff(output.Diffs, "users", result.Users)
	f.addResourceDiff(output.Diffs, "automations", result.Automations)

	encoder := json.NewEncoder(f.w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (f *JSONFormatter) addResourceDiff(diffs map[string]JSONResourceDiff, name string, rd ResourceDiff) {
	if rd.Summary.Added == 0 && rd.Summary.Modified == 0 && rd.Summary.Removed == 0 {
		return
	}

	jrd := JSONResourceDiff{}

	for _, change := range rd.Added {
		jrd.Added = append(jrd.Added, JSONResourceChange{
			Identifier: change.Identifier,
			Data:       change.TargetData,
		})
	}

	for _, change := range rd.Modified {
		jc := JSONResourceChange{
			Identifier: change.Identifier,
			SourceData: change.SourceData,
			TargetData: change.TargetData,
		}
		for _, fd := range change.FieldDiffs {
			jc.Changes = append(jc.Changes, JSONFieldDiff{
				Path:        fd.Path,
				SourceValue: fd.SourceValue,
				TargetValue: fd.TargetValue,
			})
		}
		jrd.Modified = append(jrd.Modified, jc)
	}

	for _, change := range rd.Removed {
		jrd.Removed = append(jrd.Removed, JSONResourceChange{
			Identifier: change.Identifier,
			Data:       change.SourceData,
		})
	}

	diffs[name] = jrd
}

func (f *JSONFormatter) sumAdded(r *CompareResult) int {
	return r.Blueprints.Summary.Added + r.Actions.Summary.Added +
		r.Scorecards.Summary.Added + r.Pages.Summary.Added +
		r.Integrations.Summary.Added + r.Teams.Summary.Added +
		r.Users.Summary.Added + r.Automations.Summary.Added
}

func (f *JSONFormatter) sumModified(r *CompareResult) int {
	return r.Blueprints.Summary.Modified + r.Actions.Summary.Modified +
		r.Scorecards.Summary.Modified + r.Pages.Summary.Modified +
		r.Integrations.Summary.Modified + r.Teams.Summary.Modified +
		r.Users.Summary.Modified + r.Automations.Summary.Modified
}

func (f *JSONFormatter) sumRemoved(r *CompareResult) int {
	return r.Blueprints.Summary.Removed + r.Actions.Summary.Removed +
		r.Scorecards.Summary.Removed + r.Pages.Summary.Removed +
		r.Integrations.Summary.Removed + r.Teams.Summary.Removed +
		r.Users.Summary.Removed + r.Automations.Summary.Removed
}

// Package diff provides a shared diff engine for compare, import, and migrate.
package diff

// Summary counts changes by category.
type Summary struct {
	Added    int
	Modified int
	Removed  int
}

// FieldChange is a single field-level difference.
type FieldChange struct {
	Path        string
	SourceValue interface{}
	TargetValue interface{}
}

// ItemChange represents one resource that was added, modified, or removed.
type ItemChange struct {
	Identifier string
	Source     map[string]interface{}
	Target     map[string]interface{}
	Fields     []FieldChange
}

// Result is the outcome of comparing two resource slices.
type Result struct {
	Summary  Summary
	Added    []ItemChange
	Modified []ItemChange
	Removed  []ItemChange
}

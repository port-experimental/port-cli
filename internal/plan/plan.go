// Package plan models migration/import execution as ordered create/update/skip steps.
package plan

import (
	"github.com/port-experimental/port-cli/internal/resources"
)

// Operation describes what should happen to a single resource.
type Operation string

const (
	OpCreate           Operation = "create"
	OpUpdate           Operation = "update"
	OpSkip             Operation = "skip"
	OpPermissionUpdate Operation = "permission-update"
)

// Step is one planned change for a resource.
type Step struct {
	Kind       resources.ResourceKind
	Operation  Operation
	Identifier string
	Payload    map[string]interface{}
}

// ExecutionPlan is the full set of steps derived from a diff.
type ExecutionPlan struct {
	Steps []Step
}

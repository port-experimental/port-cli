package plan

import (
	"sort"

	"github.com/port-experimental/port-cli/internal/resources"
)

// Summary holds per-kind operation counts derived from an execution plan.
type Summary struct {
	Created           map[resources.ResourceKind]int
	Updated           map[resources.ResourceKind]int
	Skipped           map[resources.ResourceKind]int
	PermissionUpdates map[resources.ResourceKind]int
}

// Summarize counts operations in an execution plan.
func Summarize(p *ExecutionPlan) Summary {
	s := Summary{
		Created:           make(map[resources.ResourceKind]int),
		Updated:           make(map[resources.ResourceKind]int),
		Skipped:           make(map[resources.ResourceKind]int),
		PermissionUpdates: make(map[resources.ResourceKind]int),
	}
	if p == nil {
		return s
	}
	for _, step := range p.Steps {
		switch step.Operation {
		case OpCreate:
			s.Created[step.Kind]++
		case OpUpdate:
			s.Updated[step.Kind]++
		case OpSkip:
			s.Skipped[step.Kind]++
		case OpPermissionUpdate:
			s.PermissionUpdates[step.Kind]++
		}
	}
	return s
}

// Identifiers returns sorted identifiers for steps matching kind and operation.
func Identifiers(p *ExecutionPlan, kind resources.ResourceKind, op Operation) []string {
	if p == nil {
		return nil
	}
	var ids []string
	for _, step := range p.Steps {
		if step.Kind == kind && step.Operation == op && step.Identifier != "" {
			ids = append(ids, step.Identifier)
		}
	}
	sort.Strings(ids)
	return ids
}

// CreateUpdateSets returns lookup maps for create and update steps of a resource kind.
func CreateUpdateSets(p *ExecutionPlan, kind resources.ResourceKind) (create, update map[string]bool) {
	create = make(map[string]bool)
	update = make(map[string]bool)
	if p == nil {
		return create, update
	}
	for _, step := range p.Steps {
		if step.Kind != kind {
			continue
		}
		switch step.Operation {
		case OpCreate:
			create[step.Identifier] = true
		case OpUpdate:
			update[step.Identifier] = true
		}
	}
	return create, update
}

// Count returns the number of steps for a kind and operation.
func Count(p *ExecutionPlan, kind resources.ResourceKind, op Operation) int {
	if p == nil {
		return 0
	}
	n := 0
	for _, step := range p.Steps {
		if step.Kind == kind && step.Operation == op {
			n++
		}
	}
	return n
}

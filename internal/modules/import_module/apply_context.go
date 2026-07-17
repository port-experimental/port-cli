package import_module

import (
	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/plan"
	"github.com/port-experimental/port-cli/internal/resources"
)

// ApplyContext carries the diff-derived metadata that apply still needs after
// resource payloads have been filtered into export.Data. Built from an
// ExecutionPlan so ApplyFiltered does not require the full DiffResult.
type ApplyContext struct {
	BlueprintPermissions []PermissionsChange
	ActionPermissions    []PermissionsChange
	PagePermissions      []PermissionsChange
	// UserUpdateEmails are emails that should upsert directly (skip create-first).
	UserUpdateEmails map[string]bool
}

// ApplyContextFromPlan extracts permission updates and user-update emails from
// an execution plan.
func ApplyContextFromPlan(p *plan.ExecutionPlan) ApplyContext {
	if p == nil {
		return ApplyContext{}
	}

	ctx := ApplyContext{}
	for _, step := range p.Steps {
		switch {
		case step.Operation == plan.OpPermissionUpdate:
			change := PermissionsChange{
				Identifier:  step.Identifier,
				Permissions: api.Permissions(step.Payload),
			}
			switch step.Kind {
			case resources.KindBlueprintPermissions:
				ctx.BlueprintPermissions = append(ctx.BlueprintPermissions, change)
			case resources.KindActionPermissions:
				ctx.ActionPermissions = append(ctx.ActionPermissions, change)
			case resources.KindPagePermissions:
				ctx.PagePermissions = append(ctx.PagePermissions, change)
			}
		case step.Kind == resources.KindUsers && step.Operation == plan.OpUpdate:
			if step.Identifier == "" {
				continue
			}
			if ctx.UserUpdateEmails == nil {
				ctx.UserUpdateEmails = make(map[string]bool)
			}
			ctx.UserUpdateEmails[step.Identifier] = true
		}
	}
	return ctx
}

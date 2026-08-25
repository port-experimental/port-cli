// Package compare provides functionality for comparing two Port organizations.
package compare

import (
	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/diff"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/resources"
)

// ExcludedFields contains fields to exclude from comparison.
// These fields are typically auto-generated or organization-specific.
var ExcludedFields = resources.CompareExcludedFields

// Differ computes differences between two organizations.
type Differ struct {
	excludedFields map[string]bool
}

// NewDiffer creates a new differ with default excluded fields.
func NewDiffer() *Differ {
	return &Differ{
		excludedFields: ExcludedFields,
	}
}

// Diff compares source and target org data, optionally filtered to specific resource types.
// An empty include slice means all resource types are compared.
func (d *Differ) Diff(source, target *export.Data, include []string) *CompareResult {
	result := &CompareResult{
		Identical: true,
	}

	// Compare each resource type, skipping those not in the include list
	if shouldInclude("blueprints", include) {
		result.Blueprints = d.diffBlueprints(source.Blueprints, target.Blueprints)
	}
	if shouldInclude("actions", include) || shouldInclude("automations", include) {
		result.Actions = d.diffActions(source.Actions, target.Actions)
	}
	if shouldInclude("scorecards", include) {
		result.Scorecards = d.diffScorecards(source.Scorecards, target.Scorecards)
	}
	if shouldInclude("pages", include) {
		result.Pages = d.diffPages(source.Pages, target.Pages)
	}
	if shouldInclude("integrations", include) {
		result.Integrations = d.diffIntegrations(source.Integrations, target.Integrations)
	}
	if shouldInclude("teams", include) {
		result.Teams = d.diffTeams(source.Teams, target.Teams)
	}
	if shouldInclude("users", include) {
		result.Users = d.diffUsers(source.Users, target.Users)
	}
	// Note: automations are fetched via the collector's "automations" keyword but merged
	// into Actions in export.Data; result.Automations remains a zero-value placeholder.

	if shouldInclude("blueprint-permissions", include) {
		result.BlueprintPermissions = d.diffPermissions(source.BlueprintPermissions, target.BlueprintPermissions, resources.KindBlueprintPermissions)
	}
	if shouldInclude("action-permissions", include) {
		result.ActionPermissions = d.diffPermissions(source.ActionPermissions, target.ActionPermissions, resources.KindActionPermissions)
	}
	if shouldIncludeEntities(include) {
		result.Entities = d.diffEntities(source.Entities, target.Entities)
	}

	// Check if any differences exist
	result.Identical = d.isIdentical(result)

	return result
}

// shouldInclude reports whether a resource type should be compared.
// It returns true when the include list is empty (compare all) or contains the resource.
func shouldInclude(resource string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	for _, r := range include {
		if r == resource {
			return true
		}
	}
	return false
}

// shouldIncludeEntities checks whether entities should be compared.
// Unlike other resource types, entities are never compared by default (opt-in only).
func shouldIncludeEntities(include []string) bool {
	for _, r := range include {
		if r == "entities" {
			return true
		}
	}
	return false
}

func (d *Differ) isIdentical(r *CompareResult) bool {
	checks := []DiffSummary{
		r.Blueprints.Summary, r.Actions.Summary, r.Scorecards.Summary,
		r.Pages.Summary, r.Integrations.Summary, r.Teams.Summary, r.Users.Summary,
		r.BlueprintPermissions.Summary, r.ActionPermissions.Summary, r.Entities.Summary,
	}
	for _, s := range checks {
		if s.Added > 0 || s.Modified > 0 || s.Removed > 0 {
			return false
		}
	}
	return true
}

func (d *Differ) diffBlueprints(source, target []api.Blueprint) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindBlueprints}))
}

func (d *Differ) diffActions(source, target []api.Action) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindActions}))
}

func (d *Differ) diffScorecards(source, target []api.Scorecard) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindScorecards}))
}

func (d *Differ) diffPages(source, target []api.Page) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{
		Kind:  resources.KindPages,
		Equal: resources.PagesEqual,
	}))
}

func (d *Differ) diffIntegrations(source, target []api.Integration) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindIntegrations}))
}

func (d *Differ) diffTeams(source, target []api.Team) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindTeams}))
}

func (d *Differ) diffUsers(source, target []api.User) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindUsers}))
}

func (d *Differ) diffEntities(source, target []api.Entity) ResourceDiff {
	return fromDiffResult(diff.DiffMaps(toMaps(source), toMaps(target), diff.Config{Kind: resources.KindEntities}))
}

func (d *Differ) diffPermissions(source, target map[string]api.Permissions, kind resources.ResourceKind) ResourceDiff {
	toSlice := func(m map[string]api.Permissions) []map[string]interface{} {
		var result []map[string]interface{}
		for id, perms := range m {
			entry := make(map[string]interface{})
			for k, v := range perms {
				entry[k] = v
			}
			entry["identifier"] = id
			result = append(result, entry)
		}
		return result
	}
	return fromDiffResult(diff.DiffMaps(toSlice(source), toSlice(target), diff.Config{Kind: kind}))
}

// toMaps converts a slice of typed maps to []map[string]interface{}.
func toMaps[T ~map[string]interface{}](items []T) []map[string]interface{} {
	result := make([]map[string]interface{}, len(items))
	for i, item := range items {
		result[i] = map[string]interface{}(item)
	}
	return result
}

// FormatPath formats a diff path for display.
func FormatPath(path string) string {
	if len(path) > 0 && path[0] == '.' {
		return path[1:]
	}
	return path
}

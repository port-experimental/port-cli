package snapshot

import (
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/resources"
)

// CollectPlan describes what to collect from an organization.
type CollectPlan struct {
	// IncludeResources limits collection to these resource type names.
	// An empty slice collects all resource types (entities remain opt-in via IncludeEntities).
	IncludeResources   []string
	IncludeEntities    bool
	IncludeRuleResults bool

	ExcludeBlueprints      []string
	ExcludeBlueprintSchema []string

	SkipSystemBlueprints          bool
	SkipSystemBlueprintProperties bool
	AutoScopeBlueprints           bool

	Filters Filters
}

// Filters holds per-resource identifier filters applied after bulk fetch.
type Filters struct {
	Blueprints   []string
	Entities     []string
	Scorecards   []string
	Actions      []string
	Pages        []string
	Integrations []string
	Teams        []string
	Users        []string
}

// ExportMetadataCollectPlan builds a collection plan for export metadata gathering.
// Entity bodies are streamed separately by export.Module when enabled.
func ExportMetadataCollectPlan(opts export.Options) CollectPlan {
	return CollectPlan{
		IncludeResources:              append([]string(nil), opts.IncludeResources...),
		IncludeEntities:               false,
		IncludeRuleResults:            opts.IncludeRuleResults,
		ExcludeBlueprints:             append([]string(nil), opts.ExcludeBlueprints...),
		ExcludeBlueprintSchema:        append([]string(nil), opts.ExcludeBlueprintSchema...),
		SkipSystemBlueprints:          opts.SkipSystemBlueprints,
		SkipSystemBlueprintProperties: opts.SkipSystemBlueprintProperties,
		AutoScopeBlueprints:           opts.AutoScopeBlueprints,
		Filters: Filters{
			Blueprints:   append([]string(nil), opts.Blueprints...),
			Entities:     append([]string(nil), opts.Entities...),
			Scorecards:   append([]string(nil), opts.Scorecards...),
			Actions:      append([]string(nil), opts.Actions...),
			Pages:        append([]string(nil), opts.Pages...),
			Integrations: append([]string(nil), opts.Integrations...),
			Teams:        append([]string(nil), opts.Teams...),
			Users:        append([]string(nil), opts.Users...),
		},
	}
}

// ImportDiffCollectPlan builds a collection plan for import/migrate diff against current org state.
func ImportDiffCollectPlan(skipEntities, includeRuleResults bool, includeResources, excludeBlueprints, excludeBlueprintSchema []string) CollectPlan {
	return CollectPlan{
		IncludeResources:       append([]string(nil), includeResources...),
		IncludeEntities:        !skipEntities,
		IncludeRuleResults:     includeRuleResults,
		ExcludeBlueprints:      append([]string(nil), excludeBlueprints...),
		ExcludeBlueprintSchema: append([]string(nil), excludeBlueprintSchema...),
	}
}

// MigrateCollectPlan builds a collection plan for migrate source export.
// Entity bodies are never collected inline; migrate streams them separately.
// Teams and users are collected in migrate after snapshot collection when appropriate.
func MigrateCollectPlan(
	includeRuleResults bool,
	includeResources, excludeBlueprints, excludeBlueprintSchema []string,
	skipSystemBlueprints, skipSystemBlueprintProperties, autoScopeBlueprints bool,
	filters Filters,
) CollectPlan {
	return CollectPlan{
		IncludeResources:              append([]string(nil), includeResources...),
		IncludeEntities:               false,
		IncludeRuleResults:            includeRuleResults,
		ExcludeBlueprints:             append([]string(nil), excludeBlueprints...),
		ExcludeBlueprintSchema:        append([]string(nil), excludeBlueprintSchema...),
		SkipSystemBlueprints:          skipSystemBlueprints,
		SkipSystemBlueprintProperties: skipSystemBlueprintProperties,
		AutoScopeBlueprints:           autoScopeBlueprints,
		Filters:                       filters,
	}
}

// CompareCollectPlan builds a collection plan from compare --include values.
func CompareCollectPlan(includeResources []string) CollectPlan {
	includeEntities := false
	for _, r := range includeResources {
		if r == string(resources.KindEntities) {
			includeEntities = true
			break
		}
	}
	return CollectPlan{
		IncludeResources:   includeResources,
		IncludeEntities:    includeEntities,
		IncludeRuleResults: true,
	}
}

// IncludesPermissions reports whether permission resources are in scope.
func (p CollectPlan) IncludesPermissions() bool {
	if len(p.IncludeResources) == 0 {
		return true
	}
	for _, r := range p.IncludeResources {
		switch resources.ResourceKind(r) {
		case resources.KindBlueprintPermissions, resources.KindActionPermissions, resources.KindPagePermissions:
			return true
		}
	}
	return false
}

// ExportOptions converts the plan into export collector options.
func (p CollectPlan) ExportOptions() export.Options {
	return export.Options{
		Blueprints:                    append([]string(nil), p.Filters.Blueprints...),
		SkipEntities:                  !p.IncludeEntities,
		IncludeRuleResults:            p.IncludeRuleResults,
		IncludeResources:              append([]string(nil), p.IncludeResources...),
		ExcludeBlueprints:             append([]string(nil), p.ExcludeBlueprints...),
		ExcludeBlueprintSchema:        append([]string(nil), p.ExcludeBlueprintSchema...),
		SkipSystemBlueprints:          p.SkipSystemBlueprints,
		SkipSystemBlueprintProperties: p.SkipSystemBlueprintProperties,
		AutoScopeBlueprints:           p.AutoScopeBlueprints,
		Entities:                      append([]string(nil), p.Filters.Entities...),
		Scorecards:                    append([]string(nil), p.Filters.Scorecards...),
		Actions:                       append([]string(nil), p.Filters.Actions...),
		Pages:                         append([]string(nil), p.Filters.Pages...),
		Integrations:                  append([]string(nil), p.Filters.Integrations...),
		Teams:                         append([]string(nil), p.Filters.Teams...),
		Users:                         append([]string(nil), p.Filters.Users...),
	}
}

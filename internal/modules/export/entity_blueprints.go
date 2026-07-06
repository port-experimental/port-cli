package export

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	systemblueprints "github.com/port-experimental/port-cli/internal/modules/system_blueprints"
)

// BlueprintsForEntityStreaming returns blueprints eligible for per-blueprint entity
// iteration (after blueprint filter, exclusions, and system blueprint skips).
func BlueprintsForEntityStreaming(ctx context.Context, client *api.Client, opts Options) ([]api.Blueprint, error) {
	allBlueprints, err := client.GetBlueprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blueprints: %w", err)
	}

	blueprints := allBlueprints
	if len(opts.Blueprints) > 0 {
		blueprintSet := make(map[string]bool, len(opts.Blueprints))
		for _, bpID := range opts.Blueprints {
			blueprintSet[bpID] = true
		}
		blueprints = nil
		for _, bp := range allBlueprints {
			if identifier, ok := bp["identifier"].(string); ok && blueprintSet[identifier] {
				blueprints = append(blueprints, bp)
			}
		}
	}

	excludeDeep := append([]string{}, opts.ExcludeBlueprints...)
	if !opts.IncludeRuleResults {
		excludeDeep = append(excludeDeep, "_rule_result")
	}
	if opts.SkipSystemBlueprints {
		for _, bp := range blueprints {
			id, _ := bp["identifier"].(string)
			if strings.HasPrefix(id, "_") {
				excludeDeep = append(excludeDeep, id)
			}
		}
	}
	iterBlueprints, _ := systemblueprints.ApplyExclusions(
		blueprints,
		excludeDeep,
		opts.ExcludeBlueprintSchema,
		opts.SkipSystemBlueprints,
		opts.SkipSystemBlueprintProperties,
	)

	out := make([]api.Blueprint, 0, len(iterBlueprints))
	for _, blueprint := range iterBlueprints {
		bpID, _ := blueprint["identifier"].(string)
		if bpID == "" {
			continue
		}
		if opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_") {
			continue
		}
		out = append(out, blueprint)
	}
	return out, nil
}

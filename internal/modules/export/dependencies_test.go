package export

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestResolveBlueprintDependencies_PullsRelationTargets(t *testing.T) {
	all := []api.Blueprint{
		{
			"identifier": "service",
			"relations": map[string]interface{}{
				"domain_rel": map[string]interface{}{"target": "domain"},
			},
		},
		{"identifier": "domain"},
	}
	selected := []api.Blueprint{all[0]}

	resolved := ResolveBlueprintDependencies(all, selected)
	if len(resolved) != 2 {
		t.Fatalf("expected 2 blueprints after resolving dependencies, got %d", len(resolved))
	}
	ids := map[string]bool{}
	for _, bp := range resolved {
		id, _ := bp["identifier"].(string)
		ids[id] = true
	}
	if !ids["service"] || !ids["domain"] {
		t.Errorf("expected service and domain, got %v", ids)
	}
}

package import_module

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestApplyDataExclusion_Deep(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service"},
			{"identifier": "_rule_result"},
		},
		Entities: []api.Entity{
			{"identifier": "e1", "blueprint": "service"},
			{"identifier": "e2", "blueprint": "_rule_result"},
		},
		Scorecards: []api.Scorecard{
			{"identifier": "sc1", "blueprintIdentifier": "_rule_result"},
		},
		Actions: []api.Action{
			{"identifier": "a1", "blueprint": "_rule_result"},
		},
		BlueprintPermissions: map[string]api.Permissions{
			"_rule_result": {"read": []string{"everyone"}},
			"service":      {"read": []string{"everyone"}},
		},
	}

	applyDataExclusion(data, []string{"_rule_result"}, nil)

	if len(data.Blueprints) != 1 {
		t.Errorf("expected 1 blueprint, got %d", len(data.Blueprints))
	}
	if len(data.Entities) != 1 {
		t.Errorf("expected 1 entity (deep removes resources too), got %d", len(data.Entities))
	}
	if len(data.Scorecards) != 0 {
		t.Errorf("expected 0 scorecards, got %d", len(data.Scorecards))
	}
	if len(data.Actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(data.Actions))
	}
	if _, ok := data.BlueprintPermissions["_rule_result"]; ok {
		t.Error("expected BlueprintPermissions entry for excluded blueprint '_rule_result' to be removed")
	}
	if _, ok := data.BlueprintPermissions["service"]; !ok {
		t.Error("expected BlueprintPermissions entry for non-excluded blueprint 'service' to be present")
	}
}

func TestApplyDataExclusion_SchemaOnly(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{
			{"identifier": "service"},
			{"identifier": "_rule_result"},
		},
		Entities: []api.Entity{
			{"identifier": "e1", "blueprint": "service"},
			{"identifier": "e2", "blueprint": "_rule_result"},
		},
		Scorecards: []api.Scorecard{
			{"identifier": "sc1", "blueprintIdentifier": "_rule_result"},
		},
		Actions: []api.Action{
			{"identifier": "a1", "blueprint": "_rule_result"},
		},
	}

	applyDataExclusion(data, nil, []string{"_rule_result"})

	if len(data.Blueprints) != 1 {
		t.Errorf("expected 1 blueprint (schema removed), got %d", len(data.Blueprints))
	}
	// Schema-only: entities/scorecards/actions for _rule_result are KEPT
	if len(data.Entities) != 2 {
		t.Errorf("expected 2 entities (schema-only keeps resources), got %d", len(data.Entities))
	}
	if len(data.Scorecards) != 1 {
		t.Errorf("expected 1 scorecard (schema-only keeps resources), got %d", len(data.Scorecards))
	}
	if len(data.Actions) != 1 {
		t.Errorf("expected 1 action (schema-only keeps resources), got %d", len(data.Actions))
	}
}

func TestApplyDataExclusion_NoExclude(t *testing.T) {
	data := &export.Data{
		Blueprints: []api.Blueprint{{"identifier": "service"}},
		Entities:   []api.Entity{{"identifier": "e1", "blueprint": "service"}},
	}
	applyDataExclusion(data, nil, nil)
	if len(data.Blueprints) != 1 || len(data.Entities) != 1 {
		t.Error("empty exclusion lists should leave data unchanged")
	}
}

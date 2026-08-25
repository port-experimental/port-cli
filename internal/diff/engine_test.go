package diff

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/resources"
)

func TestDiffMaps_Identical(t *testing.T) {
	items := []map[string]interface{}{
		{"identifier": "deploy", "title": "Deploy"},
	}
	result := DiffMaps(items, items, Config{Kind: resources.KindActions})
	if result.Summary.Added != 0 || result.Summary.Modified != 0 || result.Summary.Removed != 0 {
		t.Fatalf("expected no changes, got %#v", result.Summary)
	}
}

func TestDiffMaps_AddedRemovedModified(t *testing.T) {
	source := []map[string]interface{}{
		{"identifier": "a", "title": "A"},
		{"identifier": "b", "title": "B"},
	}
	target := []map[string]interface{}{
		{"identifier": "a", "title": "A changed"},
		{"identifier": "c", "title": "C"},
	}

	result := DiffMaps(source, target, Config{Kind: resources.KindActions})
	if result.Summary.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Summary.Added)
	}
	if result.Summary.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", result.Summary.Removed)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Summary.Modified)
	}
}

func TestDiffMaps_ExcludedFieldsIgnored(t *testing.T) {
	source := []map[string]interface{}{
		{"identifier": "a", "title": "Same", "createdAt": "2024-01-01"},
	}
	target := []map[string]interface{}{
		{"identifier": "a", "title": "Same", "createdAt": "2025-01-01"},
	}

	result := DiffMaps(source, target, Config{Kind: resources.KindActions})
	if result.Summary.Modified != 0 {
		t.Errorf("expected excluded fields to be ignored, got %d modified", result.Summary.Modified)
	}
}

func TestDiffForImport_CreateUpdateSkip(t *testing.T) {
	current := []map[string]interface{}{
		{"name": "platform", "description": "Old"},
		{"name": "infra", "description": "Same"},
	}
	desired := []map[string]interface{}{
		{"name": "platform", "description": "New"},
		{"name": "infra", "description": "Same"},
		{"name": "security", "description": "New team"},
	}

	outcome := DiffForImport(current, desired, ImportConfig{Kind: resources.KindTeams})
	if len(outcome.ToCreate) != 1 {
		t.Errorf("expected 1 create, got %d", len(outcome.ToCreate))
	}
	if len(outcome.ToUpdate) != 1 {
		t.Errorf("expected 1 update, got %d", len(outcome.ToUpdate))
	}
	if len(outcome.ToSkip) != 1 {
		t.Errorf("expected 1 skip, got %d", len(outcome.ToSkip))
	}
}

func TestDiffForImport_ShouldSkip(t *testing.T) {
	desired := []map[string]interface{}{
		{"identifier": "home", "protected": true},
	}
	outcome := DiffForImport(
		[]map[string]interface{}{},
		desired,
		ImportConfig{
			Kind: resources.KindPages,
			ShouldSkip: func(m map[string]interface{}) bool {
				protected, _ := m["protected"].(bool)
				return protected
			},
		},
	)
	if len(outcome.ToSkip) != 1 || len(outcome.ToCreate) != 0 {
		t.Fatalf("expected protected page skipped, got %#v", outcome)
	}
}

func TestDiffForImport_IgnoreMissing(t *testing.T) {
	desired := []map[string]interface{}{
		{"installationId": "github", "config": map[string]interface{}{"org": "acme"}},
	}
	outcome := DiffForImport(
		[]map[string]interface{}{},
		desired,
		ImportConfig{
			Kind:          resources.KindIntegrations,
			IgnoreMissing: true,
		},
	)
	if len(outcome.ToCreate) != 0 || len(outcome.ToUpdate) != 0 || len(outcome.ToSkip) != 0 {
		t.Fatalf("expected missing integration to be ignored, got %#v", outcome)
	}
}

func TestDiffPermissions_NewAndChanged(t *testing.T) {
	current := map[string]map[string]interface{}{
		"service": {"entities": map[string]interface{}{"view": []interface{}{"$team"}}},
	}
	desired := map[string]map[string]interface{}{
		"service": {"entities": map[string]interface{}{"view": []interface{}{"$admin"}}},
		"deploy":  {"execute": map[string]interface{}{"users": []interface{}{}}},
	}

	changes := DiffPermissions(current, desired, resources.KindBlueprintPermissions)
	if len(changes) != 2 {
		t.Fatalf("expected 2 permission changes, got %d", len(changes))
	}
}

package diff

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/resources"
)

func toMaps(items []map[string]interface{}) []map[string]interface{} {
	return items
}

func TestCompareImportSummaryAgreement_Actions(t *testing.T) {
	assertAgreement(t, resources.KindActions, []map[string]interface{}{
		{"identifier": "deploy", "title": "Deploy"},
	})
}

func TestCompareImportSummaryAgreement_Teams(t *testing.T) {
	assertAgreement(t, resources.KindTeams, []map[string]interface{}{
		{"name": "platform", "description": "Platform"},
	})
}

func TestCompareImportSummaryAgreement_Users(t *testing.T) {
	assertAgreement(t, resources.KindUsers, []map[string]interface{}{
		{"email": "user@example.com", "firstName": "Test"},
	})
}

func assertAgreement(t *testing.T, kind resources.ResourceKind, baseline []map[string]interface{}) {
	t.Helper()

	// Identical snapshots
	compareResult := DiffMaps(baseline, baseline, Config{Kind: kind})
	importResult := DiffForImport(baseline, baseline, ImportConfig{Kind: kind})
	if compareResult.Summary.Added != 0 || compareResult.Summary.Modified != 0 || compareResult.Summary.Removed != 0 {
		t.Fatalf("identical compare: expected zero summary, got %#v", compareResult.Summary)
	}
	importSummary := ImportSummary(importResult)
	if importSummary.Added != 0 || importSummary.Modified != 0 {
		t.Fatalf("identical import: expected zero create/update, got %#v", importSummary)
	}
	if len(importResult.ToSkip) != len(baseline) {
		t.Fatalf("identical import: expected %d skipped, got %d", len(baseline), len(importResult.ToSkip))
	}

	// One modified item
	if len(baseline) == 0 {
		t.Fatal("baseline fixture must not be empty")
	}
	modified := cloneSlice(baseline)
	modified[0] = cloneMap(modified[0])
	for k := range modified[0] {
		if k == "identifier" || k == "name" || k == "email" {
			continue
		}
		modified[0][k] = "changed"
		break
	}

	compareModified := DiffMaps(baseline, modified, Config{Kind: kind})
	importModified := DiffForImport(baseline, modified, ImportConfig{Kind: kind})
	if compareModified.Summary.Modified != 1 {
		t.Errorf("modified compare: expected 1 modified, got %d", compareModified.Summary.Modified)
	}
	if len(importModified.ToUpdate) != 1 {
		t.Errorf("modified import: expected 1 update, got %d", len(importModified.ToUpdate))
	}

	// One added item in target/desired
	added := append(cloneSlice(baseline), map[string]interface{}{"identifier": "new-action", "title": "New"})
	if kind == resources.KindTeams {
		added[len(added)-1] = map[string]interface{}{"name": "new-team", "description": "New"}
	}
	if kind == resources.KindUsers {
		added[len(added)-1] = map[string]interface{}{"email": "new@example.com", "firstName": "New"}
	}

	compareAdded := DiffMaps(baseline, added, Config{Kind: kind})
	importAdded := DiffForImport(baseline, added, ImportConfig{Kind: kind})
	if compareAdded.Summary.Added != 1 {
		t.Errorf("added compare: expected 1 added, got %d", compareAdded.Summary.Added)
	}
	if len(importAdded.ToCreate) != 1 {
		t.Errorf("added import: expected 1 create, got %d", len(importAdded.ToCreate))
	}
}

func cloneSlice(in []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, len(in))
	for i, item := range in {
		out[i] = cloneMap(item)
	}
	return out
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

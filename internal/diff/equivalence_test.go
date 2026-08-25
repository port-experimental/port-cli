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

func TestCompareImportSummaryAgreement_Entities(t *testing.T) {
	assertAgreement(t, resources.KindEntities, []map[string]interface{}{
		{"blueprint": "service", "identifier": "svc", "title": "Service"},
	})
}

func TestCompareImportSummaryAgreement_Scorecards(t *testing.T) {
	assertAgreement(t, resources.KindScorecards, []map[string]interface{}{
		{"blueprintIdentifier": "service", "identifier": "quality", "title": "Quality"},
	})
}

func TestCompareImportSummaryAgreement_Integrations(t *testing.T) {
	kind := resources.KindIntegrations
	baseline := []map[string]interface{}{
		{"installationId": "github", "config": map[string]interface{}{"org": "acme"}},
	}

	compareResult := DiffMaps(baseline, baseline, Config{Kind: kind})
	importResult := DiffForImport(baseline, baseline, ImportConfig{Kind: kind, IgnoreMissing: true})
	if compareResult.Summary.Modified != 0 || importSummaryMismatch(compareResult.Summary, importResult) {
		t.Fatalf("identical integration fixture mismatch: compare=%#v import=%#v", compareResult.Summary, ImportSummary(importResult))
	}

	modified := cloneSlice(baseline)
	modified[0] = cloneMap(modified[0])
	modified[0]["config"] = map[string]interface{}{"org": "other"}

	compareModified := DiffMaps(baseline, modified, Config{Kind: kind})
	importModified := DiffForImport(baseline, modified, ImportConfig{Kind: kind, IgnoreMissing: true})
	if compareModified.Summary.Modified != 1 || len(importModified.ToUpdate) != 1 {
		t.Fatalf("modified integration mismatch: compare=%#v import update=%d", compareModified.Summary, len(importModified.ToUpdate))
	}
}

func importSummaryMismatch(compare Summary, importOutcome ImportOutcome[map[string]interface{}]) bool {
	summary := ImportSummary(importOutcome)
	return summary.Added != compare.Added || summary.Modified != compare.Modified
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
	if _, ok := modified[0]["title"]; ok {
		modified[0]["title"] = "changed"
	} else if _, ok := modified[0]["description"]; ok {
		modified[0]["description"] = "changed"
	} else if _, ok := modified[0]["firstName"]; ok {
		modified[0]["firstName"] = "changed"
	} else if cfg, ok := modified[0]["config"].(map[string]interface{}); ok {
		cfg["org"] = "changed"
	} else {
		t.Fatal("baseline fixture needs a mutable non-identity field")
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
	added := cloneSlice(baseline)
	switch kind {
	case resources.KindActions:
		added = append(added, map[string]interface{}{"identifier": "new-action", "title": "New"})
	case resources.KindTeams:
		added = append(added, map[string]interface{}{"name": "new-team", "description": "New"})
	case resources.KindUsers:
		added = append(added, map[string]interface{}{"email": "new@example.com", "firstName": "New"})
	case resources.KindEntities:
		added = append(added, map[string]interface{}{"blueprint": "service", "identifier": "new-svc", "title": "New"})
	case resources.KindScorecards:
		added = append(added, map[string]interface{}{"blueprintIdentifier": "service", "identifier": "new-sc", "title": "New"})
	default:
		t.Fatalf("unsupported kind for added fixture: %s", kind)
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

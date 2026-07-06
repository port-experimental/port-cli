package resources

import "testing"

func TestPagesEqual_NullNavFieldIgnored(t *testing.T) {
	source := map[string]interface{}{
		"identifier": "overview",
		"title":      "Overview",
		"parent":     nil,
	}
	current := map[string]interface{}{
		"identifier": "overview",
		"title":      "Overview",
		"parent":     "root",
	}
	if !PagesEqual(source, current) {
		t.Fatal("expected null source parent to be ignored")
	}
}

func TestPagesEqual_NavFieldDifferenceDetected(t *testing.T) {
	source := map[string]interface{}{
		"identifier": "overview",
		"title":      "Overview",
		"parent":     "other",
	}
	current := map[string]interface{}{
		"identifier": "overview",
		"title":      "Overview",
		"parent":     "root",
	}
	if PagesEqual(source, current) {
		t.Fatal("expected different parent values to differ")
	}
}

func TestPagesEqual_EmptyRequiredQueryParamsIgnored(t *testing.T) {
	source := map[string]interface{}{
		"identifier":          "overview",
		"title":               "Overview",
		"requiredQueryParams": []interface{}{},
	}
	current := map[string]interface{}{
		"identifier":          "overview",
		"title":               "Overview",
		"requiredQueryParams": []interface{}{"foo"},
	}
	if !PagesEqual(source, current) {
		t.Fatal("expected empty source requiredQueryParams to be ignored")
	}
}

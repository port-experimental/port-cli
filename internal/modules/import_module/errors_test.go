package import_module

import (
	"errors"
	"testing"
)

func TestCategorizeError_Dependency(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"not found", errors.New("Blueprint with identifier \"foo\" was not found")},
		{"target not exist", errors.New("relation target does not exist")},
		{"missing blueprint", errors.New("missing blueprint reference")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := CategorizeError(tt.err, "blueprint", "test")
			if ie.Category != ErrDependency {
				t.Errorf("expected ErrDependency, got %s", ie.Category)
			}
			if !ie.Retryable {
				t.Error("dependency errors should be retryable")
			}
		})
	}
}

func TestCategorizeError_Auth(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"unauthorized", errors.New("401 Unauthorized")},
		{"forbidden", errors.New("403 Forbidden")},
		{"invalid credentials", errors.New("invalid_credentials")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := CategorizeError(tt.err, "blueprint", "test")
			if ie.Category != ErrAuth {
				t.Errorf("expected ErrAuth, got %s", ie.Category)
			}
			if ie.Retryable {
				t.Error("auth errors should not be retryable")
			}
		})
	}
}

func TestCategorizeError_RateLimit(t *testing.T) {
	err := errors.New("429 Too Many Requests")
	ie := CategorizeError(err, "entity", "test")
	if ie.Category != ErrRateLimit {
		t.Errorf("expected ErrRateLimit, got %s", ie.Category)
	}
	if !ie.Retryable {
		t.Error("rate limit errors should be retryable")
	}
}

func TestCategorizeError_Network(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"timeout", errors.New("i/o timeout")},
		{"connection refused", errors.New("connection refused")},
		{"context canceled", errors.New("context canceled")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := CategorizeError(tt.err, "entity", "test")
			if ie.Category != ErrNetwork {
				t.Errorf("expected ErrNetwork, got %s", ie.Category)
			}
			if !ie.Retryable {
				t.Error("network errors should be retryable")
			}
		})
	}
}

func TestCategorizeError_Conflict(t *testing.T) {
	err := errors.New("409 Conflict - resource already exists")
	ie := CategorizeError(err, "blueprint", "test")
	if ie.Category != ErrConflict {
		t.Errorf("expected ErrConflict, got %s", ie.Category)
	}
}

func TestCategorizeError_Validation(t *testing.T) {
	err := errors.New("400 Bad Request - validation failed: required field missing")
	ie := CategorizeError(err, "entity", "test")
	if ie.Category != ErrValidation {
		t.Errorf("expected ErrValidation, got %s", ie.Category)
	}
}

func TestCategorizeError_BlueprintConfig(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"inherited_ownership_enabled", errors.New(`{"error":"inherited_ownership_enabled"}`)},
		{"inherited ownership", errors.New("inherited ownership is enabled for this blueprint")},
		{"protected_resource", errors.New(`{"error":"protected_resource"}`)},
		{"protected entity", errors.New("protected entity violation")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := CategorizeError(tt.err, "entity", "test")
			if ie.Category != ErrBlueprintConfig {
				t.Errorf("expected ErrBlueprintConfig, got %s", ie.Category)
			}
			if ie.Retryable {
				t.Error("blueprint config errors should not be retryable")
			}
		})
	}
}

func TestCategorizeError_SchemaMismatch(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"blueprint_schema_mismatch", errors.New(`{"error":"blueprint_schema_mismatch","missing_required_property":"source"}`)},
		{"schema mismatch", errors.New("schema mismatch error")},
		{"missing required property", errors.New("missing required property 'name'")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := CategorizeError(tt.err, "entity", "test")
			if ie.Category != ErrSchemaMismatch {
				t.Errorf("expected ErrSchemaMismatch, got %s", ie.Category)
			}
			if ie.Retryable {
				t.Error("schema mismatch errors should not be retryable")
			}
		})
	}
}

func TestCategorizeError_Nil(t *testing.T) {
	ie := CategorizeError(nil, "blueprint", "test")
	if ie != nil {
		t.Error("expected nil for nil error")
	}
}

func TestErrorCollector_Basic(t *testing.T) {
	ec := NewErrorCollector()

	ec.Add(errors.New("not found"), "blueprint", "bp1")
	ec.Add(errors.New("unauthorized"), "entity", "e1")
	ec.Add(errors.New("rate limit"), "action", "a1")

	if ec.Count() != 3 {
		t.Errorf("expected 3 errors, got %d", ec.Count())
	}

	if !ec.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
}

func TestErrorCollector_ByCategory(t *testing.T) {
	ec := NewErrorCollector()

	ec.Add(errors.New("Blueprint foo was not found"), "blueprint", "bp1")
	ec.Add(errors.New("Blueprint bar was not found"), "blueprint", "bp2")
	ec.Add(errors.New("unauthorized"), "entity", "e1")

	depErrors := ec.GetByCategory(ErrDependency)
	if len(depErrors) != 2 {
		t.Errorf("expected 2 dependency errors, got %d", len(depErrors))
	}

	authErrors := ec.GetByCategory(ErrAuth)
	if len(authErrors) != 1 {
		t.Errorf("expected 1 auth error, got %d", len(authErrors))
	}
}

func TestErrorCollector_GetRetryable(t *testing.T) {
	ec := NewErrorCollector()

	ec.Add(errors.New("not found"), "blueprint", "bp1")       // retryable
	ec.Add(errors.New("unauthorized"), "entity", "e1")        // not retryable
	ec.Add(errors.New("connection timeout"), "action", "a1")  // retryable

	retryable := ec.GetRetryable()
	if len(retryable) != 2 {
		t.Errorf("expected 2 retryable errors, got %d", len(retryable))
	}
}

func TestErrorCollector_Summary(t *testing.T) {
	ec := NewErrorCollector()

	// Add multiple errors of different types
	for i := 0; i < 10; i++ {
		ec.Add(errors.New("Blueprint foo was not found"), "blueprint", "bp")
	}
	ec.Add(errors.New("unauthorized"), "entity", "e1")

	summary := ec.Summary(3)

	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Should contain category counts
	if !contains(summary, "DEPENDENCY (10)") {
		t.Error("summary should contain DEPENDENCY count")
	}
	if !contains(summary, "AUTH (1)") {
		t.Error("summary should contain AUTH count")
	}
	if !contains(summary, "and 7 more") {
		t.Error("summary should show remaining count for truncated category")
	}
}

func TestErrorCollector_ToStringSlice(t *testing.T) {
	ec := NewErrorCollector()
	ec.Add(errors.New("error 1"), "blueprint", "bp1")
	ec.Add(errors.New("error 2"), "entity", "e1")

	slice := ec.ToStringSlice()
	if len(slice) != 2 {
		t.Errorf("expected 2 strings, got %d", len(slice))
	}
}

func TestErrorCollector_Clear(t *testing.T) {
	ec := NewErrorCollector()
	ec.Add(errors.New("error"), "blueprint", "bp1")

	if ec.Count() != 1 {
		t.Error("expected 1 error before clear")
	}

	ec.Clear()

	if ec.Count() != 0 {
		t.Error("expected 0 errors after clear")
	}
	if ec.HasErrors() {
		t.Error("expected HasErrors to be false after clear")
	}
}

func TestErrorCollector_AddNil(t *testing.T) {
	ec := NewErrorCollector()
	ec.Add(nil, "blueprint", "bp1")

	if ec.Count() != 0 {
		t.Error("nil errors should not be added")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

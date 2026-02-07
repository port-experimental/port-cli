package compare

import (
	"testing"
)

func TestDetectInputType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{"org name", "production", "org"},
		{"tar.gz file", "/path/to/export.tar.gz", "file"},
		{"json file", "./backup.json", "file"},
		{"relative tar", "backup.tar.gz", "file"},
		{"relative path with dir", "../export.tar.gz", "file"},
		{"absolute path json", "/tmp/data.json", "file"},
		{"org name with dash", "my-org", "org"},
		{"org name with underscore", "my_org", "org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectInputType(tt.input)
			if got != tt.wantType {
				t.Errorf("detectInputType(%q) = %q, want %q", tt.input, got, tt.wantType)
			}
		})
	}
}

package skills

import (
	"testing"
)

func TestParseSkillLocation(t *testing.T) {
	tests := []struct {
		input string
		want  SkillLocation
	}{
		{"project", SkillLocationProject},
		{"global", SkillLocationGlobal},
		{"", SkillLocationGlobal},
		{"other", SkillLocationGlobal},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseSkillLocation(tt.input); got != tt.want {
				t.Errorf("parseSkillLocation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildSkillMD(t *testing.T) {
	t.Run("with instructions", func(t *testing.T) {
		md := buildSkillMD(Skill{Identifier: "s", Description: "desc", Instructions: "step 1\nstep 2"})
		for _, want := range []string{"name: s", "description: desc", "step 1"} {
			if !containsStr(md, want) {
				t.Errorf("missing %q in output", want)
			}
		}
	})
	t.Run("no instructions fallback", func(t *testing.T) {
		md := buildSkillMD(Skill{Identifier: "empty", Title: "Empty"})
		if !containsStr(md, "_No instructions provided._") {
			t.Error("expected fallback text")
		}
	})
}

func TestGroupName(t *testing.T) {
	groups := []SkillGroup{
		{Identifier: "grp-1", Title: "My Group"},
		{Identifier: "grp-2", Title: ""},
	}
	tests := []struct{ groupID, want string }{
		{"grp-1", "My Group"},
		{"grp-2", "grp-2"},
		{"unknown", "unknown"},
		{"", NoGroupDir},
	}
	for _, tt := range tests {
		t.Run(tt.groupID, func(t *testing.T) {
			if got := GroupName(groups, tt.groupID); got != tt.want {
				t.Errorf("GroupName(%q) = %q, want %q", tt.groupID, got, tt.want)
			}
		})
	}
}

func TestValidatePathComponent(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"my-skill", false},
		{"..", true},
		{".", true},
		{"a/b", true},
		{"a\\b", true},
		{"my.skill.v2", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validatePathComponent(tt.input)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

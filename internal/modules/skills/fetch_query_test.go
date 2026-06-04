package skills

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestBuildFetchSkillsQuery_ExcludeFlags(t *testing.T) {
	cfg := &config.SkillsConfig{SelectAllGroups: true}
	opts := &LoadSkillsOptions{ExcludeLegacySkills: true, ExcludeInternalSkills: true}
	q := buildFetchSkillsQuery(cfg, opts)
	if len(q.Exclude) != 2 {
		t.Fatalf("Exclude: %v", q.Exclude)
	}
	if q.Exclude[0] != "legacy" || q.Exclude[1] != "internal" {
		t.Fatalf("Exclude values: %v", q.Exclude)
	}
}

func TestBuildFetchSkillsQuery_DefaultIncludesLegacyAndInternal(t *testing.T) {
	cfg := &config.SkillsConfig{SelectAllGroups: true}
	q := buildFetchSkillsQuery(cfg, &LoadSkillsOptions{})
	if len(q.Exclude) != 0 {
		t.Fatalf("expected no exclude by default, got %v", q.Exclude)
	}
}

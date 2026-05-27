package skills

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
)

// DoctorResult summarizes skills blueprint resolution and catalog reachability.
type DoctorResult struct {
	Blueprints   api.SkillBlueprintSet
	Family       string // "prefixed" or "unprefixed"
	ContentModel string // "versioned" or "legacy"
	GroupCount   int
	SkillCount   int
}

// Doctor resolves skills blueprints against Port and probes entity counts.
func (m *Module) Doctor(ctx context.Context) (*DoctorResult, error) {
	bps, err := m.client.ResolveSkillBlueprints(ctx)
	if err != nil {
		return nil, err
	}

	result := &DoctorResult{
		Blueprints:   bps,
		Family:       bps.Family(),
		ContentModel: bps.ContentModel(),
	}

	groups, err := m.client.GetSkillGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s entities: %w", bps.SkillGroup, err)
	}
	result.GroupCount = len(groups)

	skills, err := m.client.GetSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s entities: %w", bps.Skill, err)
	}
	result.SkillCount = len(skills)

	return result, nil
}

package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SkillBlueprintSet holds resolved Port blueprint identifiers for the skills feature.
type SkillBlueprintSet struct {
	SkillGroup   string
	Skill        string
	SkillVersion string // empty when not present in the catalog
	SkillFile    string // empty when not present in the catalog
}

// HasVersionedBlueprints reports whether versioned skill content blueprints exist.
func (s SkillBlueprintSet) HasVersionedBlueprints() bool {
	return s.SkillVersion != "" && s.SkillFile != ""
}

// Family reports whether the resolved blueprint names use the Port system prefix.
func (s SkillBlueprintSet) Family() string {
	if strings.HasPrefix(s.SkillGroup, "_") {
		return "prefixed"
	}
	return "unprefixed"
}

// ContentModel reports how skill file content is loaded from Port.
func (s SkillBlueprintSet) ContentModel() string {
	if s.HasVersionedBlueprints() {
		return "versioned"
	}
	return "legacy"
}

type skillBlueprintCache struct {
	once sync.Once
	set  SkillBlueprintSet
	err  error
}

// ResolveSkillBlueprints discovers skills-related blueprint identifiers from the
// Port catalog. Prefixed system blueprints (_skill_*) take priority over
// unprefixed names; version/file IDs are set only when those blueprints exist.
func (c *Client) ResolveSkillBlueprints(ctx context.Context) (SkillBlueprintSet, error) {
	if c.skillBPs == nil {
		c.skillBPs = &skillBlueprintCache{}
	}
	c.skillBPs.once.Do(func() {
		c.skillBPs.set, c.skillBPs.err = resolveSkillBlueprints(ctx, c)
	})
	return c.skillBPs.set, c.skillBPs.err
}

func resolveSkillBlueprints(ctx context.Context, c *Client) (SkillBlueprintSet, error) {
	blueprints, err := c.GetBlueprints(ctx)
	if err != nil {
		return SkillBlueprintSet{}, fmt.Errorf("failed to list blueprints for skills resolution: %w", err)
	}
	ids := make(map[string]bool, len(blueprints))
	for _, bp := range blueprints {
		if id, ok := bp["identifier"].(string); ok && id != "" {
			ids[id] = true
		}
	}
	if set, ok := pickSkillBlueprintSet(ids, "_skill_group", "_skill", "_skill_version", "_skill_file"); ok {
		return set, nil
	}
	if set, ok := pickSkillBlueprintSet(ids, "skill_group", "skill", "skill_version", "skill_file"); ok {
		return set, nil
	}
	return SkillBlueprintSet{}, fmt.Errorf(
		"no skills blueprints found: expected _skill_group and _skill, or skill_group and skill",
	)
}

func pickSkillBlueprintSet(ids map[string]bool, group, skill, version, file string) (SkillBlueprintSet, bool) {
	if !ids[group] || !ids[skill] {
		return SkillBlueprintSet{}, false
	}
	set := SkillBlueprintSet{SkillGroup: group, Skill: skill}
	if ids[version] {
		set.SkillVersion = version
	}
	if ids[file] {
		set.SkillFile = file
	}
	return set, true
}

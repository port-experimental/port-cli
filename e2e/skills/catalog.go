//go:build e2e

package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/auth"
)

// ActiveSkillExpect is the active published skill content from ai-service (source of truth for disk assertions).
type ActiveSkillExpect struct {
	Identifier  string
	Version     string
	Description string
	SkillMD     string
}

func skillFileContent(files []aiservice.SkillFile, path string) string {
	for _, f := range files {
		p, _ := f.Properties["path"].(string)
		if p != path {
			continue
		}
		c, _ := f.Properties["content"].(string)
		return c
	}
	return ""
}

func buildActiveCatalog(ctx context.Context, client *aiservice.Client, token *auth.Token) (map[string]ActiveSkillExpect, error) {
	resp, err := client.GetSkillsGrouped(ctx, token, aiservice.GetSkillsQuery{Limit: 500})
	if err != nil {
		return nil, err
	}
	out := make(map[string]ActiveSkillExpect)
	add := func(s aiservice.SkillAtLatestVersion) {
		if s.Identifier == "" {
			return
		}
		out[s.Identifier] = ActiveSkillExpect{
			Identifier:  s.Identifier,
			Version:     strings.TrimSpace(s.Version),
			Description: strings.TrimSpace(s.Description),
			SkillMD:     skillFileContent(s.Files, "SKILL.md"),
		}
	}
	for _, g := range resp.Groups {
		for _, s := range g.Skills {
			add(s)
		}
	}
	for _, s := range resp.UngroupedSkills {
		add(s)
	}
	return out, nil
}

func catalogPropString(props map[string]interface{}, key string) string {
	if props == nil {
		return ""
	}
	raw, ok := props[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func findCatalogEntry(entries []aiservice.SkillCatalogEntry, identifier string) (aiservice.SkillCatalogEntry, bool) {
	for _, e := range entries {
		if e.Skill.Identifier == identifier {
			return e, true
		}
	}
	return aiservice.SkillCatalogEntry{}, false
}

func assertDiskMatchesActive(t testingT, portRoot string, exp ActiveSkillExpect) {
	t.Helper()
	disk, err := readSkillMD(portRoot, exp.Identifier)
	if err != nil {
		t.Fatalf("%v", err)
	}
	name := parseFrontmatterName(disk)
	if name != exp.Identifier {
		t.Fatalf("skill %q: frontmatter name is %q, want %q", exp.Identifier, name, exp.Identifier)
	}
	// Version semver is catalog metadata; only assert it on disk when the active SKILL.md includes it.
	if exp.Version != "" && strings.Contains(exp.SkillMD, exp.Version) && !strings.Contains(disk, exp.Version) {
		t.Fatalf("skill %q: disk missing active version %q from catalog SKILL.md", exp.Identifier, exp.Version)
	}
	if exp.Description != "" && !strings.Contains(disk, exp.Description) {
		t.Fatalf("skill %q: disk missing active description %q", exp.Identifier, exp.Description)
	}
	snippet := distinctiveSnippet(exp.SkillMD)
	if snippet != "" && !strings.Contains(disk, snippet) {
		t.Fatalf("skill %q: disk missing active body snippet %q", exp.Identifier, snippet)
	}
}

func assertDiskReflectsCatalog(t testingT, portRoot string, catalog map[string]ActiveSkillExpect, ids ...string) {
	t.Helper()
	for _, id := range ids {
		exp, ok := catalog[id]
		if !ok {
			t.Fatalf("skill %q not in active catalog (is demo seed loaded?)", id)
		}
		assertDiskMatchesActive(t, portRoot, exp)
	}
}

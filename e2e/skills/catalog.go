//go:build e2e

package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/auth"
	skillmod "github.com/port-experimental/port-cli/internal/modules/skills"
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

func groupedCatalogSkillIDs(ctx context.Context, client *aiservice.Client, token *auth.Token, exclude []string) (map[string]bool, error) {
	resp, err := client.GetSkillsGrouped(ctx, token, aiservice.GetSkillsQuery{
		Limit:   500,
		Exclude: append([]string(nil), exclude...),
	})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool)
	add := func(s aiservice.SkillAtLatestVersion) {
		if s.Identifier != "" {
			ids[s.Identifier] = true
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
	return ids, nil
}

func firstCatalogIDOnlyIn(full, subset map[string]bool) string {
	for id := range full {
		if subset[id] {
			continue
		}
		if strings.HasPrefix(id, "e2e-") {
			continue
		}
		return id
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

func publishedGroupedVersion(ctx context.Context, client *aiservice.Client, token *auth.Token, identifier string) (string, bool, error) {
	catalog, err := buildActiveCatalog(ctx, client, token)
	if err != nil {
		return "", false, err
	}
	exp, ok := catalog[identifier]
	if !ok {
		return "", false, nil
	}
	return exp.Version, true, nil
}

func summarySkillVersion(ctx context.Context, mod *skillmod.Module, identifier string, publishedOnly bool) (string, bool, error) {
	entries, err := mod.ListSkills(ctx, aiservice.GetSkillsSummaryQuery{
		SkillIdentifiers: []string{identifier},
		PublishedOnly:    publishedOnly,
		Limit:            10,
	})
	if err != nil {
		return "", false, err
	}
	entry, ok := findCatalogEntry(entries, identifier)
	if !ok || entry.Version == nil {
		return "", false, nil
	}
	version := catalogPropString(entry.Version.Properties, "version")
	if version == "" {
		return "", false, nil
	}
	return version, true, nil
}

func assertSummaryVersion(t testingT, ctx context.Context, mod *skillmod.Module, identifier, want string, publishedOnly bool) {
	t.Helper()
	got, ok, err := summarySkillVersion(ctx, mod, identifier, publishedOnly)
	if err != nil {
		t.Fatalf("summary %q published_only=%v: %v", identifier, publishedOnly, err)
	}
	if !ok {
		t.Fatalf("skill %q not in summary (published_only=%v)", identifier, publishedOnly)
	}
	if got != want {
		t.Fatalf("skill %q summary version: got %q want %q (published_only=%v)", identifier, got, want, publishedOnly)
	}
}

func assertNotInPublishedGrouped(t testingT, ctx context.Context, client *aiservice.Client, token *auth.Token, identifier string) {
	t.Helper()
	_, ok, err := publishedGroupedVersion(ctx, client, token, identifier)
	if err != nil {
		t.Fatalf("published grouped catalog: %v", err)
	}
	if ok {
		t.Fatalf("skill %q should not appear in published grouped catalog", identifier)
	}
}

func assertPublishedGroupedVersion(t testingT, ctx context.Context, client *aiservice.Client, token *auth.Token, identifier, want string) {
	t.Helper()
	got, ok, err := publishedGroupedVersion(ctx, client, token, identifier)
	if err != nil {
		t.Fatalf("published grouped catalog: %v", err)
	}
	if !ok {
		t.Fatalf("skill %q missing from published grouped catalog", identifier)
	}
	if got != want {
		t.Fatalf("skill %q published grouped version: got %q want %q", identifier, got, want)
	}
}

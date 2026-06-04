//go:build e2e

package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	skillmod "github.com/port-experimental/port-cli/internal/modules/skills"
)

// legacyMustNotAppear guards against writing superseded demo versions (secondary to API-active checks).
var legacyMustNotAppear = map[string][]string{
	DemoSkillOnboarding:   {"(v1.0.0) Minimal onboarding", "(v1.1.0) Onboarding with common-errors"},
	DemoSkillAPIGuide:     {"(v1.0.0) Legacy Port REST"},
	DemoSkillTroubleshoot: {"release_state: publish"},
}

func TestSkillsE2E(t *testing.T) {
	if os.Getenv("PORT_E2E_SKILLS") != "1" {
		t.Skip("set PORT_E2E_SKILLS=1 to run local skills E2E (make e2e-skills-local)")
	}

	h := newHarness(t)
	ctx := context.Background()
	catalog := h.activeCatalog(ctx)

	t.Run("Preflight", func(t *testing.T) {
		if err := curlHealth(strings.TrimSuffix(h.env.APIURL, "/v1")); err != nil {
			t.Fatalf("port-api: %v", err)
		}
		if err := curlHealth(strings.TrimSuffix(h.env.AIServiceURL, "/v1")); err != nil {
			t.Fatalf("ai-service: %v", err)
		}
		entries, err := h.mod.ListSkills(ctx, aiservice.GetSkillsSummaryQuery{PublishedOnly: true, Limit: 500})
		if err != nil {
			t.Fatalf("list skills: %v", err)
		}
		entry, ok := findCatalogEntry(entries, DemoSkillOnboarding)
		if !ok {
			t.Fatalf("catalog missing %s — run yarn seed:demo-skills in Port repo", DemoSkillOnboarding)
		}
		exp := catalog[DemoSkillOnboarding]
		if exp.Version == "" {
			t.Fatalf("catalog has no active version for %s", DemoSkillOnboarding)
		}
		got := catalogPropString(entry.Version.Properties, "version")
		if got != exp.Version {
			t.Fatalf("list version for %s: got %q want %q (from grouped catalog)", DemoSkillOnboarding, got, exp.Version)
		}
	})

	t.Run("Search", func(t *testing.T) {
		entries, err := h.mod.SearchSkills(ctx, aiservice.SearchSkillsQuery{Query: "demo", Limit: 50, PublishedOnly: true})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("search demo returned no matches")
		}
	})

	t.Run("SyncWithoutInit", func(t *testing.T) {
		homeDir := filepath.Join(h.env.ConfigDir, "sync-no-init-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := h.syncWithoutInit(t, ctx, homeDir); err != nil {
			t.Fatalf("sync without init: %v", err)
		}
		agentsRoot := portSkillsRootForBase(filepath.Join(homeDir, ".agents"))
		claudeRoot := portSkillsRootForBase(filepath.Join(homeDir, ".claude"))
		for _, root := range []string{agentsRoot, claudeRoot} {
			if !skillPresent(root, DemoSkillOnboarding) {
				t.Fatalf("expected %s under %s", DemoSkillOnboarding, root)
			}
		}
		assertDiskReflectsCatalog(t, agentsRoot, catalog, DemoSkillOnboarding)
	})

	t.Run("InitSavedIncludeExclude", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			TeamGroupDefaults: true,
			IncludeGroups:     []string{DemoGroupOptional},
			ExcludeGroups:     []string{DemoGroupRequired},
			SelectAllUngrouped: false,
		})
		if err := h.sync(ctx, skillsSelection{
			TeamGroupDefaults:  true,
			IncludeGroups:      []string{DemoGroupOptional},
			ExcludeGroups:      []string{DemoGroupRequired},
			SelectAllUngrouped: false,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		for _, id := range []string{DemoSkillTroubleshoot, DemoSkillWorkflows} {
			if !skillPresent(root, id) {
				t.Fatalf("expected optional group skill on disk: %s", id)
			}
		}
		for _, id := range []string{DemoSkillOnboarding, DemoSkillAPIGuide} {
			if skillPresent(root, id) {
				t.Fatalf("excluded required group skill should be absent: %s", id)
			}
		}
		if skillPresent(root, DemoSkillStandalone) {
			t.Fatalf("ungrouped skill should not sync when select_all_ungrouped is false")
		}
	})

	t.Run("CLISyncWithoutInit", func(t *testing.T) {
		homeDir := filepath.Join(h.env.ConfigDir, "cli-sync-no-init-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := h.writeConfigOrgOnly(); err != nil {
			t.Fatalf("config: %v", err)
		}
		cfgPath := filepath.Join(h.env.ConfigDir, "config.yaml")
		cmd := exec.Command(h.env.PortBin, "--config", cfgPath, "skills", "sync")
		cmd.Dir = h.env.WorkDir
		cmd.Env = append(os.Environ(), "HOME="+homeDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("port skills sync: %v\n%s", err, out)
		}
		claudeRoot := portSkillsRootForBase(filepath.Join(homeDir, ".claude"))
		if !skillPresent(claudeRoot, DemoSkillOnboarding) {
			t.Fatalf("CLI sync without init did not write %s to %s", DemoSkillOnboarding, claudeRoot)
		}
	})

	t.Run("SyncActiveVersions", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     append([]string(nil), demoAllGroups...),
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     demoAllGroups,
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		want := []string{
			DemoSkillOnboarding, DemoSkillAPIGuide, DemoSkillStandalone,
			DemoSkillTroubleshoot, DemoSkillWorkflows, DemoSkillSecurity,
		}
		for _, id := range want {
			if !skillPresent(root, id) {
				t.Fatalf("missing on disk: %s", id)
			}
		}
		assertOnlyDemoSkills(t, root, want)
		assertDiskReflectsCatalog(t, root, catalog, want...)
		assertLegacyAbsent(t, root, want...)
	})

	t.Run("SelectionRequiredAndUngrouped", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     []string{DemoGroupRequired},
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{DemoGroupRequired},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		present := []string{DemoSkillOnboarding, DemoSkillAPIGuide, DemoSkillStandalone}
		absent := []string{DemoSkillTroubleshoot, DemoSkillWorkflows, DemoSkillSecurity}
		for _, id := range present {
			if !skillPresent(root, id) {
				t.Fatalf("expected on disk: %s", id)
			}
		}
		for _, id := range absent {
			if skillPresent(root, id) {
				t.Fatalf("should be absent: %s", id)
			}
		}
		assertOnlyDemoSkills(t, root, present)
		assertDiskReflectsCatalog(t, root, catalog, present...)
		assertLegacyAbsent(t, root, DemoSkillOnboarding, DemoSkillAPIGuide)
	})

	t.Run("SelectionWidenThenNarrow", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     []string{DemoGroupRequired, DemoGroupOptional},
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{DemoGroupRequired, DemoGroupOptional},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync widen: %v", err)
		}
		root := h.env.PortSkillsRoot
		withOptional := []string{DemoSkillOnboarding, DemoSkillTroubleshoot, DemoSkillWorkflows}
		for _, id := range withOptional {
			if !skillPresent(root, id) {
				t.Fatalf("expected after widen: %s", id)
			}
		}
		if skillPresent(root, DemoSkillSecurity) {
			t.Fatalf("security skill should stay excluded")
		}
		assertDiskReflectsCatalog(t, root, catalog, DemoSkillTroubleshoot)

		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{DemoGroupRequired},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync narrow: %v", err)
		}
		kept := []string{DemoSkillOnboarding, DemoSkillAPIGuide, DemoSkillStandalone}
		pruned := []string{DemoSkillTroubleshoot, DemoSkillWorkflows}
		for _, id := range kept {
			if !skillPresent(root, id) {
				t.Fatalf("expected kept: %s", id)
			}
		}
		for _, id := range pruned {
			if skillPresent(root, id) {
				t.Fatalf("expected pruned: %s", id)
			}
		}
		assertOnlyDemoSkills(t, root, kept)
	})

	t.Run("TeamOwnership", func(t *testing.T) {
		email := strings.TrimSpace(h.token.Claims.Email)
		orgID := strings.TrimSpace(h.token.Claims.OrgId)
		if email == "" || orgID == "" {
			t.Skip("token missing email or org id — cannot run team ownership E2E")
		}

		groupsResp, err := h.ai.GetSkillGroups(ctx, h.token)
		if err != nil {
			t.Fatalf("get skill groups: %v", err)
		}
		teamMap, err := listTeamIDToName(ctx, h.orgCfg.APIURL, h.token)
		if err != nil {
			t.Fatalf("list _team entities: %v", err)
		}

		required, _ := findGroups(groupsResp.Groups)
		if required == nil {
			t.Fatalf("missing group %s in catalog", DemoGroupRequired)
		}
		if len(required.OwningTeamIDs) == 0 {
			t.Fatalf("group %s has no owning teams (re-seed demo skills?)", DemoGroupRequired)
		}

		requiredTeam := firstTeamName(required.OwningTeamIDs, teamMap)
		if override := strings.TrimSpace(os.Getenv("E2E_TEAM_NAME")); override != "" {
			requiredTeam = override
		}
		if requiredTeam == "" {
			t.Fatalf("could not resolve team name for %s owningTeamIds %v", DemoGroupRequired, required.OwningTeamIDs)
		}

		originalTeams, err := h.admin.GetUserTeamNames(ctx, h.token, orgID, email)
		if err != nil {
			t.Fatalf("get user teams: %v", err)
		}
		t.Cleanup(func() {
			cleanupCtx := context.Background()
			_ = h.admin.setUserTeams(cleanupCtx, h.token, orgID, email, originalTeams)
		})

		teamSyncSel := skillsSelection{
			TeamGroupDefaults:  true,
			IncludeGroups:      []string{DemoGroupRequired},
			ExcludeGroups:      []string{DemoGroupOptional, DemoGroupSecurity},
			SelectAllUngrouped: true,
		}

		if err := h.admin.setUserTeams(ctx, h.token, orgID, email, []string{requiredTeam}); err != nil {
			t.Fatalf("assign owning team %q: %v", requiredTeam, err)
		}
		groupsAfter, err := h.ai.GetSkillGroups(ctx, h.token)
		if err != nil {
			t.Fatalf("re-fetch groups: %v", err)
		}
		for _, g := range groupsAfter.Groups {
			if g.Identifier == DemoGroupRequired && !g.MatchesUserTeams {
				t.Fatalf("group %s should match user teams after assigning %q", DemoGroupRequired, requiredTeam)
			}
		}

		h.beginScenario(&teamSyncSel)
		if err := h.sync(ctx, teamSyncSel); err != nil {
			t.Fatalf("sync with team defaults: %v", err)
		}

		fetched, err := h.mod.FetchSkillsWithQuery(ctx, skillmod.FetchSkillsQuery{
			TeamsDefault:  true,
			IncludeGroups: []string{DemoGroupRequired},
			ExcludeGroups: []string{DemoGroupOptional, DemoGroupSecurity},
		})
		if err != nil {
			t.Fatalf("fetch catalog for team sync: %v", err)
		}
		allowedIDs := make(map[string]bool)
		for _, s := range fetched.Skills {
			allowedIDs[s.Identifier] = true
		}

		root := h.env.PortSkillsRoot
		teamPresent := []string{DemoSkillOnboarding, DemoSkillAPIGuide, DemoSkillStandalone}
		for _, id := range teamPresent {
			if !skillPresent(root, id) {
				t.Fatalf("team sync missing %s", id)
			}
		}
		synced, err := listSyncedSkillIDs(root)
		if err != nil {
			t.Fatalf("list disk skills: %v", err)
		}
		for _, id := range synced {
			if strings.HasPrefix(id, "e2e-") {
				continue
			}
			if strings.HasPrefix(id, "demo-") && !allowedIDs[id] {
				t.Fatalf("disk has %q but team-filtered catalog did not include it", id)
			}
		}
		assertDiskReflectsCatalog(t, root, catalog, teamPresent...)
	})

	t.Run("CRUD", func(t *testing.T) {
		singleID := fmt.Sprintf("e2e-single-%s", h.env.RunID)
		singleDir := filepath.Join(h.env.ConfigDir, singleID)
		if err := writeSingleSkillFixture(singleDir, h.env.FixturesDir, singleID); err != nil {
			t.Fatalf("single fixture: %v", err)
		}
		first, err := h.mod.UploadSkillFromFolder(ctx, singleDir, skillmod.PackSkillFolderOptions{}, true)
		if err != nil {
			t.Fatalf("upload single: %v", err)
		}
		if first.Version != "1.0.0" {
			t.Fatalf("first version = %q", first.Version)
		}

		batchRoot := filepath.Join(h.env.ConfigDir, "batch-"+h.env.RunID)
		batchA := fmt.Sprintf("e2e-skill-a-%s", h.env.RunID)
		batchB := fmt.Sprintf("e2e-skill-b-%s", h.env.RunID)
		if err := writeBatchFixtures(batchRoot, h.env.FixturesDir, batchA, batchB); err != nil {
			t.Fatalf("batch fixtures: %v", err)
		}
		roots, err := skillmod.DiscoverSkillRoots(batchRoot)
		if err != nil {
			t.Fatalf("discover batch: %v", err)
		}
		packs := make([]skillmod.SkillPackWithFolder, 0, len(roots))
		for _, root := range roots {
			pack, err := skillmod.PackSkillFolder(root, skillmod.PackSkillFolderOptions{})
			if err != nil {
				t.Fatalf("pack %s: %v", root, err)
			}
			packs = append(packs, skillmod.SkillPackWithFolder{Pack: pack, FolderBase: filepath.Base(root)})
		}
		if _, err := h.mod.UploadSkillsBatch(ctx, packs, true); err != nil {
			t.Fatalf("batch upload: %v", err)
		}

		second, err := h.mod.UploadSkillFromFolder(ctx, singleDir, skillmod.PackSkillFolderOptions{}, true)
		if err != nil {
			t.Fatalf("re-upload: %v", err)
		}
		if second.Version != "1.0.1" {
			t.Fatalf("upsert version = %q, want 1.0.1", second.Version)
		}

		if err := h.mod.UnpublishSkill(ctx, singleID); err != nil {
			t.Fatalf("unpublish: %v", err)
		}
	})

	t.Run("ArchiveSubcommandRemoved", func(t *testing.T) {
		cmd := exec.Command(h.env.PortBin, "--config", filepath.Join(h.env.ConfigDir, "config.yaml"), "skills", "--tree")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("skills --tree: %v\n%s", err, out)
		}
		if strings.Contains(string(out), "archive") {
			t.Fatalf("skills command tree still lists archive")
		}
	})
}

func assertLegacyAbsent(t *testing.T, portRoot string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		legacy, ok := legacyMustNotAppear[id]
		if !ok {
			continue
		}
		disk, err := readSkillMD(portRoot, id)
		if err != nil {
			continue
		}
		for _, needle := range legacy {
			if strings.Contains(disk, needle) {
				t.Fatalf("skill %q disk contains superseded content %q", id, needle)
			}
		}
	}
}

func findGroups(groups []aiservice.SkillGroupCatalogEntry) (required, optional *aiservice.SkillGroupCatalogEntry) {
	for i := range groups {
		switch groups[i].Identifier {
		case DemoGroupRequired:
			required = &groups[i]
		case DemoGroupOptional:
			optional = &groups[i]
		}
	}
	return required, optional
}

func firstTeamName(teamIDs []string, idToName map[string]string) string {
	names := resolveTeamNames(teamIDs, idToName)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func writeSingleSkillFixture(destDir, fixturesDir, skillID string) error {
	src := filepath.Join(fixturesDir, "single-skill", "SKILL.md")
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	content := strings.Replace(string(data), "name: e2e-single-skill", "name: "+skillID, 1)
	return os.WriteFile(filepath.Join(destDir, "SKILL.md"), []byte(content), 0o644)
}

func writeBatchFixtures(batchRoot, fixturesDir, idA, idB string) error {
	for _, pair := range []struct{ sub, placeholder, id string }{
		{"skill-a", "e2e-skill-a", idA},
		{"skill-b", "e2e-skill-b", idB},
	} {
		src := filepath.Join(fixturesDir, "batch-two-skills", pair.sub, "SKILL.md")
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		dir := filepath.Join(batchRoot, pair.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		content := strings.Replace(string(data), "name: "+pair.placeholder, "name: "+pair.id, 1)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

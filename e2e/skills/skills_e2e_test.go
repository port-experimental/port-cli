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
	SeedSkillLocalDevSetup:      {"(v1.0.0) bootstrap-only checklist", "(v1.1.0) adds Docker Compose stack"},
	SeedSkillPortAPIClient:      {"(v1.0.0) legacy REST v1 examples"},
	SeedSkillMCPTroubleshooting: {"legacy MCP auth: PAT-only flows", "release_state: publish"},
	SeedSkillWorkflowAutomation: {"(v1.0.0) YAML-only automations"},
}

func TestSkillsE2E(t *testing.T) {
	if os.Getenv("PORT_E2E_SKILLS") != "1" {
		t.Skip("set PORT_E2E_SKILLS=1 to run local skills E2E (make e2e-skills-local)")
	}

	h := newHarness(t)
	h.logArtifactPaths(t)
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
		entry, ok := findCatalogEntry(entries, SeedSkillLocalDevSetup)
		if !ok {
			t.Fatalf("catalog missing %s — run yarn seed:demo-skills in Port repo", SeedSkillLocalDevSetup)
		}
		exp := catalog[SeedSkillLocalDevSetup]
		if exp.Version == "" {
			t.Fatalf("catalog has no active version for %s", SeedSkillLocalDevSetup)
		}
		got := catalogPropString(entry.Version.Properties, "version")
		if got != exp.Version {
			t.Fatalf("list version for %s: got %q want %q (from grouped catalog)", SeedSkillLocalDevSetup, got, exp.Version)
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
			if !skillPresent(root, SeedSkillLocalDevSetup) {
				t.Fatalf("expected %s under %s", SeedSkillLocalDevSetup, root)
			}
		}
		assertDiskReflectsCatalog(t, agentsRoot, catalog, SeedSkillLocalDevSetup)
	})

	t.Run("InitSavedIncludeExclude", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			TeamGroupDefaults: true,
			IncludeGroups:     []string{SeedGroupOperations},
			ExcludeGroups:     []string{SeedGroupPlatform},
			SelectAllUngrouped: false,
		})
		if err := h.sync(ctx, skillsSelection{
			TeamGroupDefaults:  true,
			IncludeGroups:      []string{SeedGroupOperations},
			ExcludeGroups:      []string{SeedGroupPlatform},
			SelectAllUngrouped: false,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		for _, id := range []string{SeedSkillMCPTroubleshooting, SeedSkillWorkflowAutomation} {
			if !skillPresent(root, id) {
				t.Fatalf("expected optional group skill on disk: %s", id)
			}
		}
		for _, id := range []string{SeedSkillLocalDevSetup, SeedSkillPortAPIClient} {
			if skillPresent(root, id) {
				t.Fatalf("excluded required group skill should be absent: %s", id)
			}
		}
		if skillPresent(root, SeedSkillIntegrationsOverview) {
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
		agentsRoot := portSkillsRootForBase(filepath.Join(homeDir, ".agents"))
		claudeRoot := portSkillsRootForBase(filepath.Join(homeDir, ".claude"))
		for _, root := range []string{agentsRoot, claudeRoot} {
			if !skillPresent(root, SeedSkillLocalDevSetup) {
				t.Fatalf("CLI sync without init did not write %s to %s", SeedSkillLocalDevSetup, root)
			}
		}
	})

	t.Run("SyncActiveVersions", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     append([]string(nil), seedAllGroups...),
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     seedAllGroups,
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		want := []string{
			SeedSkillLocalDevSetup, SeedSkillPortAPIClient, SeedSkillIntegrationsOverview,
			SeedSkillMCPTroubleshooting, SeedSkillWorkflowAutomation, SeedSkillSecurityPRReview,
		}
		for _, id := range want {
			if !skillPresent(root, id) {
				t.Fatalf("missing on disk: %s", id)
			}
		}
		assertOnlySeedCatalogSkills(t, root, want)
		assertDiskReflectsCatalog(t, root, catalog, want...)
		assertLegacyAbsent(t, root, want...)
	})

	t.Run("SelectionRequiredAndUngrouped", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     []string{SeedGroupPlatform},
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{SeedGroupPlatform},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync: %v", err)
		}
		root := h.env.PortSkillsRoot
		present := []string{SeedSkillLocalDevSetup, SeedSkillPortAPIClient, SeedSkillIntegrationsOverview}
		absent := []string{SeedSkillMCPTroubleshooting, SeedSkillWorkflowAutomation, SeedSkillSecurityPRReview}
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
		assertOnlySeedCatalogSkills(t, root, present)
		assertDiskReflectsCatalog(t, root, catalog, present...)
		assertLegacyAbsent(t, root, SeedSkillLocalDevSetup, SeedSkillPortAPIClient)
	})

	t.Run("SelectionWidenThenNarrow", func(t *testing.T) {
		h.beginScenario(&skillsSelection{
			SelectedGroups:     []string{SeedGroupPlatform, SeedGroupOperations},
			SelectAllUngrouped: true,
		})
		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{SeedGroupPlatform, SeedGroupOperations},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync widen: %v", err)
		}
		root := h.env.PortSkillsRoot
		withOptional := []string{SeedSkillLocalDevSetup, SeedSkillMCPTroubleshooting, SeedSkillWorkflowAutomation}
		for _, id := range withOptional {
			if !skillPresent(root, id) {
				t.Fatalf("expected after widen: %s", id)
			}
		}
		if skillPresent(root, SeedSkillSecurityPRReview) {
			t.Fatalf("security skill should stay excluded")
		}
		assertDiskReflectsCatalog(t, root, catalog, SeedSkillMCPTroubleshooting)

		if err := h.sync(ctx, skillsSelection{
			SelectedGroups:     []string{SeedGroupPlatform},
			SelectAllUngrouped: true,
		}); err != nil {
			t.Fatalf("sync narrow: %v", err)
		}
		kept := []string{SeedSkillLocalDevSetup, SeedSkillPortAPIClient, SeedSkillIntegrationsOverview}
		pruned := []string{SeedSkillMCPTroubleshooting, SeedSkillWorkflowAutomation}
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
		assertOnlySeedCatalogSkills(t, root, kept)
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
			t.Fatalf("missing group %s in catalog", SeedGroupPlatform)
		}
		if len(required.OwningTeamIDs) == 0 {
			t.Fatalf("group %s has no owning teams (re-seed demo skills?)", SeedGroupPlatform)
		}

		requiredTeam := firstTeamName(required.OwningTeamIDs, teamMap)
		if override := strings.TrimSpace(os.Getenv("E2E_TEAM_NAME")); override != "" {
			requiredTeam = override
		}
		if requiredTeam == "" {
			t.Fatalf("could not resolve team name for %s owningTeamIds %v", SeedGroupPlatform, required.OwningTeamIDs)
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
			IncludeGroups:      []string{SeedGroupPlatform},
			ExcludeGroups:      []string{SeedGroupOperations, SeedGroupSecurity},
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
			if g.Identifier == SeedGroupPlatform && !g.MatchesUserTeams {
				t.Fatalf("group %s should match user teams after assigning %q", SeedGroupPlatform, requiredTeam)
			}
		}

		h.beginScenario(&teamSyncSel)
		if err := h.sync(ctx, teamSyncSel); err != nil {
			t.Fatalf("sync with team defaults: %v", err)
		}

		fetched, err := h.mod.FetchSkillsWithQuery(ctx, skillmod.FetchSkillsQuery{
			TeamsDefault:  true,
			IncludeGroups: []string{SeedGroupPlatform},
			ExcludeGroups: []string{SeedGroupOperations, SeedGroupSecurity},
		})
		if err != nil {
			t.Fatalf("fetch catalog for team sync: %v", err)
		}
		allowedIDs := make(map[string]bool)
		for _, s := range fetched.Skills {
			allowedIDs[s.Identifier] = true
		}

		root := h.env.PortSkillsRoot
		teamPresent := []string{SeedSkillLocalDevSetup, SeedSkillPortAPIClient, SeedSkillIntegrationsOverview}
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
			if seedCatalogSkillIDs[id] && !allowedIDs[id] {
				t.Fatalf("disk has %q but team-filtered catalog did not include it", id)
			}
		}
		assertDiskReflectsCatalog(t, root, catalog, teamPresent...)
	})

	t.Run("UploadLifecycle", func(t *testing.T) {
		skillA := fmt.Sprintf("e2e-life-a-%s", h.env.RunID)
		skillB := fmt.Sprintf("e2e-life-b-%s", h.env.RunID)
		dirA := filepath.Join(h.env.ConfigDir, skillA)
		dirB := filepath.Join(h.env.ConfigDir, skillB)

		mismatchRoot := filepath.Join(h.env.FixturesDir, "name-mismatch")
		_, err := skillmod.PackSkillFolder(filepath.Join(mismatchRoot, "wrong-name"), skillmod.PackSkillFolderOptions{})
		if err == nil {
			t.Fatal("PackSkillFolder should reject folder/name mismatch")
		}
		if !strings.Contains(err.Error(), "does not match SKILL.md name") {
			t.Fatalf("mismatch error: %v", err)
		}

		cfgPath := filepath.Join(h.env.ConfigDir, "config.yaml")
		cmd := exec.Command(h.env.PortBin, "--config", cfgPath, "skills", "upload", mismatchRoot)
		cmd.Dir = h.env.WorkDir
		if out, runErr := cmd.CombinedOutput(); runErr == nil {
			t.Fatalf("port skills upload name-mismatch should fail, output: %s", out)
		}

		if err := writeSingleSkillFixture(dirA, h.env.FixturesDir, skillA); err != nil {
			t.Fatalf("fixture A: %v", err)
		}
		if err := writeSingleSkillFixture(dirB, h.env.FixturesDir, skillB); err != nil {
			t.Fatalf("fixture B: %v", err)
		}

		packA, err := skillmod.PackSkillFolder(dirA, skillmod.PackSkillFolderOptions{})
		if err != nil {
			t.Fatalf("pack A: %v", err)
		}
		packB, err := skillmod.PackSkillFolder(dirB, skillmod.PackSkillFolderOptions{})
		if err != nil {
			t.Fatalf("pack B: %v", err)
		}

		first, err := h.mod.UploadSkillFromPack(ctx, packA, filepath.Base(dirA), false)
		if err != nil {
			t.Fatalf("create A without publish: %v", err)
		}
		if first.Version != "1.0.0" || first.ActiveVersionSet {
			t.Fatalf("create A: version=%q active=%v", first.Version, first.ActiveVersionSet)
		}
		assertSummaryVersion(t, ctx, h.mod, skillA, "1.0.0", false)
		assertNotInPublishedGrouped(t, ctx, h.ai, h.token, skillA)

		batchResp, err := h.ai.UploadSkillsBatch(ctx, h.token, aiservice.BatchUploadSkillsRequest{
			Skills: []aiservice.UploadSkillRequest{
				uploadRequestFromPack(packA, filepath.Base(dirA), false),
				uploadRequestFromPack(packB, filepath.Base(dirB), false),
			},
		})
		if err != nil {
			t.Fatalf("batch upload A+B: %v", err)
		}
		if len(batchResp.Results) != 2 {
			t.Fatalf("batch results len = %d", len(batchResp.Results))
		}
		for _, item := range batchResp.Results {
			if !item.OK || item.Result == nil {
				msg := ""
				if item.Error != nil {
					msg = item.Error.Message
				}
				t.Fatalf("batch item %q failed: %s", item.Identifier, msg)
			}
		}
		wantBatchVersion := map[string]string{skillA: "1.0.1", skillB: "1.0.0"}
		for id, want := range wantBatchVersion {
			var got string
			for _, item := range batchResp.Results {
				if item.Identifier == id {
					got = item.Result.Version
					break
				}
			}
			if got != want {
				t.Fatalf("batch %q version = %q want %q", id, got, want)
			}
		}

		assertSummaryVersion(t, ctx, h.mod, skillA, "1.0.1", false)
		assertSummaryVersion(t, ctx, h.mod, skillB, "1.0.0", false)
		assertNotInPublishedGrouped(t, ctx, h.ai, h.token, skillA)
		assertNotInPublishedGrouped(t, ctx, h.ai, h.token, skillB)

		published, err := h.mod.UploadSkillFromPack(ctx, packA, filepath.Base(dirA), true)
		if err != nil {
			t.Fatalf("publish A via upload: %v", err)
		}
		if published.Version != "1.0.2" || !published.ActiveVersionSet {
			t.Fatalf("publish A: version=%q active=%v", published.Version, published.ActiveVersionSet)
		}
		assertPublishedGroupedVersion(t, ctx, h.ai, h.token, skillA, "1.0.2")
		assertSummaryVersion(t, ctx, h.mod, skillA, "1.0.2", true)

		if err := h.mod.UnpublishSkill(ctx, skillA); err != nil {
			t.Fatalf("unpublish A: %v", err)
		}
		assertNotInPublishedGrouped(t, ctx, h.ai, h.token, skillA)
		_, ok, err := summarySkillVersion(ctx, h.mod, skillA, true)
		if err != nil {
			t.Fatalf("summary after unpublish: %v", err)
		}
		if ok {
			t.Fatalf("skill %q should not appear in published-only summary after unpublish", skillA)
		}
		assertSummaryVersion(t, ctx, h.mod, skillA, "1.0.2", false)
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
		case SeedGroupPlatform:
			required = &groups[i]
		case SeedGroupOperations:
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

func uploadRequestFromPack(pack *skillmod.SkillFolderPack, folderBase string, publish bool) aiservice.UploadSkillRequest {
	return aiservice.UploadSkillRequest{
		Identifier:     pack.Identifier,
		Title:          pack.Title,
		Description:    pack.Description,
		Location:       pack.Location,
		Publish:        publish,
		FolderBaseName: folderBase,
		Files:          pack.Files,
	}
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

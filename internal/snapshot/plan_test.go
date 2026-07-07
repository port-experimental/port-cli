package snapshot

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestCompareCollectPlan_EntitiesOptIn(t *testing.T) {
	without := CompareCollectPlan([]string{"blueprints", "actions"})
	if without.IncludeEntities {
		t.Error("entities should not be included without explicit --include entities")
	}
	opts := without.ExportOptions()
	if !opts.SkipEntities {
		t.Error("expected SkipEntities when entities not included")
	}

	with := CompareCollectPlan([]string{"blueprints", "entities"})
	if !with.IncludeEntities {
		t.Error("expected IncludeEntities when entities in include list")
	}
	withOpts := with.ExportOptions()
	if withOpts.SkipEntities {
		t.Error("expected SkipEntities false when entities included")
	}
}

func TestCompareCollectPlan_IncludesRuleResults(t *testing.T) {
	plan := CompareCollectPlan(nil)
	if !plan.IncludeRuleResults {
		t.Error("compare plan should include rule results by default")
	}
	if !plan.ExportOptions().IncludeRuleResults {
		t.Error("export options should include rule results for compare")
	}
}

func TestCollectPlan_IncludesPermissions(t *testing.T) {
	all := CollectPlan{}
	if !all.IncludesPermissions() {
		t.Error("empty include list should include permissions")
	}

	schemaOnly := CollectPlan{IncludeResources: []string{"blueprints"}}
	if schemaOnly.IncludesPermissions() {
		t.Error("blueprints-only include should not include permissions")
	}

	withPerms := CollectPlan{IncludeResources: []string{"blueprints", string(resources.KindBlueprintPermissions)}}
	if !withPerms.IncludesPermissions() {
		t.Error("expected blueprint-permissions in scope")
	}
}

func TestCollectPlan_ExportOptionsMapsFilters(t *testing.T) {
	plan := CollectPlan{
		IncludeResources: []string{"entities", "scorecards"},
		IncludeEntities:  true,
		Filters: Filters{
			Blueprints: []string{"service"},
			Entities:   []string{"svc-a"},
			Scorecards: []string{"quality"},
		},
		ExcludeBlueprints: []string{"_user"},
	}

	opts := plan.ExportOptions()
	if opts.SkipEntities {
		t.Error("expected entities collected")
	}
	if len(opts.Blueprints) != 1 || opts.Blueprints[0] != "service" {
		t.Errorf("unexpected blueprint filter: %#v", opts.Blueprints)
	}
	if len(opts.Entities) != 1 || opts.Entities[0] != "svc-a" {
		t.Errorf("unexpected entity filter: %#v", opts.Entities)
	}
	if len(opts.Scorecards) != 1 || opts.Scorecards[0] != "quality" {
		t.Errorf("unexpected scorecard filter: %#v", opts.Scorecards)
	}
	if len(opts.ExcludeBlueprints) != 1 || opts.ExcludeBlueprints[0] != "_user" {
		t.Errorf("unexpected exclude blueprints: %#v", opts.ExcludeBlueprints)
	}
}

func TestExportMetadataCollectPlan_MatchesExportMetadataPass(t *testing.T) {
	opts := export.Options{
		IncludeResources:    []string{"blueprints", "entities", "teams"},
		IncludeRuleResults:  false,
		AutoScopeBlueprints: true,
		SkipEntities:        false,
		Blueprints:          []string{"service"},
		Teams:               []string{"platform"},
		ExcludeBlueprints:   []string{"_user"},
	}
	metadata := ExportMetadataCollectPlan(opts).ExportOptions()
	if !metadata.SkipEntities {
		t.Error("export metadata pass should always skip inline entities")
	}
	if metadata.AutoScopeBlueprints != opts.AutoScopeBlueprints {
		t.Error("expected AutoScopeBlueprints preserved")
	}
	if len(metadata.Blueprints) != 1 || metadata.Blueprints[0] != "service" {
		t.Errorf("unexpected blueprint filter: %#v", metadata.Blueprints)
	}
	if len(metadata.ExcludeBlueprints) != 1 || metadata.ExcludeBlueprints[0] != "_user" {
		t.Errorf("unexpected exclude blueprints: %#v", metadata.ExcludeBlueprints)
	}
}

func TestExportMetadataCollectPlan_NeverInlineCollectsEntities(t *testing.T) {
	plan := ExportMetadataCollectPlan(export.Options{
		IncludeResources:    []string{"blueprints", "entities", "teams"},
		IncludeRuleResults:  true,
		AutoScopeBlueprints: true,
		Blueprints:          []string{"service"},
		Teams:               []string{"platform"},
	})
	if plan.IncludeEntities {
		t.Error("export metadata plan should never inline-collect entities")
	}
	opts := plan.ExportOptions()
	if !opts.SkipEntities {
		t.Error("expected SkipEntities in export options for metadata pass")
	}
	if !opts.AutoScopeBlueprints {
		t.Error("expected AutoScopeBlueprints forwarded")
	}
	if len(opts.Blueprints) != 1 || opts.Blueprints[0] != "service" {
		t.Errorf("unexpected blueprint filter: %#v", opts.Blueprints)
	}
}

func TestMigrateCollectPlan_NeverInlineCollectsEntities(t *testing.T) {
	plan := MigrateCollectPlan(
		true,
		[]string{"blueprints", "entities"},
		nil, nil,
		false, false, true,
		Filters{Blueprints: []string{"service"}},
	)
	if plan.IncludeEntities {
		t.Error("migrate plan should never inline-collect entities")
	}
	opts := plan.ExportOptions()
	if !opts.SkipEntities {
		t.Error("expected SkipEntities in export options for migrate metadata pass")
	}
	if !opts.AutoScopeBlueprints {
		t.Error("expected AutoScopeBlueprints forwarded")
	}
	if len(opts.Blueprints) != 1 || opts.Blueprints[0] != "service" {
		t.Errorf("unexpected blueprint filter: %#v", opts.Blueprints)
	}
}

func TestImportDiffCollectPlan_RespectsSkipEntities(t *testing.T) {
	withEntities := ImportDiffCollectPlan(false, true, nil, nil, nil)
	if !withEntities.IncludeEntities {
		t.Error("expected entities when SkipEntities is false")
	}
	if !withEntities.ExportOptions().IncludeRuleResults {
		t.Error("expected rule results when requested")
	}

	withoutEntities := ImportDiffCollectPlan(true, false, []string{"blueprints"}, nil, nil)
	if withoutEntities.IncludeEntities {
		t.Error("expected entities skipped when SkipEntities is true")
	}
	if withoutEntities.ExportOptions().SkipEntities != true {
		t.Error("expected SkipEntities in export options")
	}
}

func TestFromData_PreservesOrgName(t *testing.T) {
	snap := FromData("production", "file", &export.Data{})
	if snap.OrgName != "production" {
		t.Errorf("expected org name production, got %q", snap.OrgName)
	}
	if snap.Metadata.Source != "file" {
		t.Errorf("expected source file, got %q", snap.Metadata.Source)
	}
}

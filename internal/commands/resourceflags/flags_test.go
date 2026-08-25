package resourceflags

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestParseAndValidateInclude_AcceptsKnownResources(t *testing.T) {
	list, err := ParseAndValidateInclude("blueprints,pages")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0] != "blueprints" || list[1] != "pages" {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestValidateInclude_RejectsUnknownResource(t *testing.T) {
	err := ValidateInclude([]string{"widgets"})
	if err == nil || !strings.Contains(err.Error(), "invalid resource: widgets") {
		t.Fatalf("expected invalid resource error, got %v", err)
	}
}

func TestValidateInclude_PagePermissionsRequiresPages(t *testing.T) {
	err := ValidateInclude([]string{"page-permissions"})
	if err == nil || !strings.Contains(err.Error(), "page-permissions requires pages") {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestReconcileSkipEntities_IgnoresSkipWhenEntitiesIncluded(t *testing.T) {
	skip, warnings := ReconcileSkipEntities([]string{"entities"}, true)
	if skip {
		t.Fatal("expected skipEntities cleared")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "entities") {
		t.Fatalf("expected entities warning, got %v", warnings)
	}
}

func TestBuildSelection_ActionsFlagImpliesPermissions(t *testing.T) {
	sel, err := BuildSelection(BuildInput{
		Changed: map[string]bool{"actions": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"actions", "action-permissions", "blueprints"} {
		if !slicesContains(sel.IncludeResources, want) {
			t.Fatalf("expected %q in include list, got %v", want, sel.IncludeResources)
		}
	}
	if !sel.AutoScopeBlueprints {
		t.Fatal("expected auto-scope when blueprints not explicit")
	}
}

func TestBuildSelection_BlueprintsFlagDisablesAutoScope(t *testing.T) {
	sel, err := BuildSelection(BuildInput{
		BlueprintsChanged: true,
		Changed:           map[string]bool{"entities": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.AutoScopeBlueprints {
		t.Fatal("expected auto-scope off when --blueprints explicit")
	}
}

func TestRegisterPerResourceFlags_RegistersExpectedNames(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var (
		include      string
		blueprints   string
		scorecards   string
		actions      string
		pages        string
		integrations string
		teams        string
		users        string
		entities     string
	)
	RegisterInclude(cmd, &include, "test")
	RegisterPerResourceFilters(cmd, PerResourceFlagRefs{
		Blueprints:   &blueprints,
		Scorecards:   &scorecards,
		Actions:      &actions,
		Pages:        &pages,
		Integrations: &integrations,
		Teams:        &teams,
		Users:        &users,
		Entities:     &entities,
	}, "export")

	for _, name := range []string{
		"include", "blueprints", "scorecards", "actions", "pages",
		"integrations", "teams", "users", "entities",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("flag %q not registered", name)
		}
	}
}

func slicesContains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func TestChangedFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("entities", "", "")
	if err := fs.Parse([]string{"--entities", "svc"}); err != nil {
		t.Fatal(err)
	}
	changed := Changed(fs, "entities", "actions")
	if !changed["entities"] {
		t.Fatal("expected entities changed")
	}
	if changed["actions"] {
		t.Fatal("expected actions unchanged")
	}
}

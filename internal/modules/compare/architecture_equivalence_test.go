package compare

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
)

func TestCompareIdenticalSnapshotContract(t *testing.T) {
	source := &export.Data{
		Blueprints:           []api.Blueprint{{"identifier": "service", "title": "Service"}},
		Actions:              []api.Action{{"identifier": "deploy", "title": "Deploy"}},
		Scorecards:           []api.Scorecard{{"identifier": "quality", "blueprintIdentifier": "service"}},
		Pages:                []api.Page{{"identifier": "home"}},
		Integrations:         []api.Integration{{"installationId": "github"}},
		Teams:                []api.Team{{"name": "platform"}},
		Users:                []api.User{{"email": "user@example.com"}},
		BlueprintPermissions: map[string]api.Permissions{"service": {"read": []interface{}{"Everyone"}}},
		ActionPermissions:    map[string]api.Permissions{"deploy": {"execute": []interface{}{"Everyone"}}},
		Entities:             []api.Entity{{"identifier": "svc", "blueprint": "service"}},
	}
	target := &export.Data{
		Blueprints:           append([]api.Blueprint{}, source.Blueprints...),
		Actions:              append([]api.Action{}, source.Actions...),
		Scorecards:           append([]api.Scorecard{}, source.Scorecards...),
		Pages:                append([]api.Page{}, source.Pages...),
		Integrations:         append([]api.Integration{}, source.Integrations...),
		Teams:                append([]api.Team{}, source.Teams...),
		Users:                append([]api.User{}, source.Users...),
		BlueprintPermissions: source.BlueprintPermissions,
		ActionPermissions:    source.ActionPermissions,
		Entities:             append([]api.Entity{}, source.Entities...),
	}

	result := NewDiffer().Diff(source, target, []string{"blueprints", "actions", "scorecards", "pages", "integrations", "teams", "users", "blueprint-permissions", "action-permissions", "entities"})
	if !result.Identical {
		t.Fatalf("expected identical snapshots, got %#v", result)
	}
}

package resources

import "testing"

func TestBlueprintIdentity(t *testing.T) {
	id, ok := BlueprintIdentity(map[string]interface{}{"identifier": "service"})
	if !ok || id != "service" {
		t.Errorf("got (%q, %v), want (service, true)", id, ok)
	}
}

func TestEntityIdentity(t *testing.T) {
	id, ok := EntityIdentity(map[string]interface{}{"blueprint": "service", "identifier": "svc-a"})
	if !ok || id != "service:svc-a" {
		t.Errorf("got (%q, %v), want (service:svc-a, true)", id, ok)
	}
}

func TestScorecardIdentity(t *testing.T) {
	id, ok := ScorecardIdentity(map[string]interface{}{"blueprintIdentifier": "service", "identifier": "quality"})
	if !ok || id != "service:quality" {
		t.Errorf("got (%q, %v), want (service:quality, true)", id, ok)
	}
}

func TestIntegrationIdentity_InstallationID(t *testing.T) {
	id, ok := IntegrationIdentity(map[string]interface{}{"installationId": "github"})
	if !ok || id != "github" {
		t.Errorf("got (%q, %v), want (github, true)", id, ok)
	}
}

func TestIntegrationIdentity_FallbackIdentifier(t *testing.T) {
	id, ok := IntegrationIdentity(map[string]interface{}{"identifier": "slack"})
	if !ok || id != "slack" {
		t.Errorf("got (%q, %v), want (slack, true)", id, ok)
	}
}

func TestIntegrationIdentity_Missing(t *testing.T) {
	_, ok := IntegrationIdentity(map[string]interface{}{"name": "GitHub"})
	if ok {
		t.Error("expected false for integration without identity fields")
	}
}

func TestGet_AllKindsRegistered(t *testing.T) {
	kinds := []ResourceKind{
		KindBlueprints, KindEntities, KindScorecards, KindActions, KindPages,
		KindIntegrations, KindTeams, KindUsers,
		KindBlueprintPermissions, KindActionPermissions, KindPagePermissions,
	}
	for _, kind := range kinds {
		desc, ok := Get(kind)
		if !ok {
			t.Errorf("missing descriptor for %q", kind)
			continue
		}
		if desc.Identity == nil {
			t.Errorf("descriptor %q has nil Identity", kind)
		}
	}
}

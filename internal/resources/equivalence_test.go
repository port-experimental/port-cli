package resources

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// fixturePairs holds identical and changed pairs for a resource kind.
type fixturePair struct {
	kind      ResourceKind
	identical [2]map[string]interface{}
	changed   [2]map[string]interface{}
}

func TestIdentityEquivalence_IdenticalFixtures(t *testing.T) {
	pairs := []fixturePair{
		{
			kind: KindBlueprints,
			identical: [2]map[string]interface{}{
				{"identifier": "service", "title": "Service"},
				{"identifier": "service", "title": "Service", "createdAt": "2026-01-01"},
			},
			changed: [2]map[string]interface{}{
				{"identifier": "service", "title": "Service"},
				{"identifier": "service", "title": "Changed"},
			},
		},
		{
			kind: KindEntities,
			identical: [2]map[string]interface{}{
				{"blueprint": "service", "identifier": "svc"},
				{"blueprint": "service", "identifier": "svc", "id": "abc"},
			},
			changed: [2]map[string]interface{}{
				{"blueprint": "service", "identifier": "svc", "title": "A"},
				{"blueprint": "service", "identifier": "svc", "title": "B"},
			},
		},
		{
			kind: KindScorecards,
			identical: [2]map[string]interface{}{
				{"blueprintIdentifier": "service", "identifier": "quality", "title": "Quality"},
				{"blueprintIdentifier": "service", "identifier": "quality", "title": "Quality", "updatedAt": "2026-01-01"},
			},
			changed: [2]map[string]interface{}{
				{"blueprintIdentifier": "service", "identifier": "quality", "title": "Quality"},
				{"blueprintIdentifier": "service", "identifier": "quality", "title": "Changed"},
			},
		},
		{
			kind: KindIntegrations,
			identical: [2]map[string]interface{}{
				{"installationId": "github", "config": map[string]interface{}{"org": "acme"}},
				{"installationId": "github", "config": map[string]interface{}{"org": "acme"}, "createdAt": "2026-01-01"},
			},
			changed: [2]map[string]interface{}{
				{"installationId": "github", "config": map[string]interface{}{"org": "acme"}},
				{"installationId": "github", "config": map[string]interface{}{"org": "other"}},
			},
		},
	}

	for _, pair := range pairs {
		pair := pair
		t.Run(string(pair.kind), func(t *testing.T) {
			desc, ok := Get(pair.kind)
			if !ok {
				t.Fatalf("missing descriptor for %q", pair.kind)
			}

			idA, okA := desc.Identity(pair.identical[0])
			idB, okB := desc.Identity(pair.identical[1])
			if !okA || !okB || idA != idB {
				t.Errorf("identical fixtures should share identity, got (%q,%v) and (%q,%v)", idA, okA, idB, okB)
			}
			if !ResourcesEqual(pair.identical[0], pair.identical[1], desc.ServerManagedFields) {
				t.Error("identical fixtures should be equal after normalization")
			}

			chgA, okChgA := desc.Identity(pair.changed[0])
			chgB, okChgB := desc.Identity(pair.changed[1])
			if !okChgA || !okChgB || chgA != chgB {
				t.Errorf("changed fixtures should share identity, got (%q,%v) and (%q,%v)", chgA, okChgA, chgB, okChgB)
			}
			if ResourcesEqual(pair.changed[0], pair.changed[1], desc.ServerManagedFields) {
				t.Error("changed fixtures should not be equal after normalization")
			}
		})
	}
}

func TestPermissionsIdentity_FromMapKey(t *testing.T) {
	perms := map[string]api.Permissions{
		"service": {"read": []interface{}{"Everyone"}},
	}
	for id, p := range perms {
		entry := map[string]interface{}{"identifier": id}
		for k, v := range p {
			entry[k] = v
		}
		got, ok := MapKeyIdentity(entry)
		if !ok || got != "service" {
			t.Errorf("got (%q, %v), want (service, true)", got, ok)
		}
	}
}

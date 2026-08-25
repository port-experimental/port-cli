package snapshot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/resources"
)

func TestCollector_CollectLiveSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":         true,
				"blueprints": []map[string]interface{}{{"identifier": "service", "title": "Service"}},
			})
		case "/blueprints/service/permissions":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "permissions": map[string]interface{}{}})
		case "/blueprints/service/scorecards":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "scorecards": []interface{}{}})
		case "/blueprints/service/actions":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "actions": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	collector := NewCollector(client)

	plan := CompareCollectPlan([]string{string(resources.KindBlueprints)})
	snap, err := collector.Collect(context.Background(), "production", plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.OrgName != "production" {
		t.Errorf("expected org name production, got %q", snap.OrgName)
	}
	if snap.Metadata.Source != "live" {
		t.Errorf("expected live source, got %q", snap.Metadata.Source)
	}
	if snap.Metadata.CollectedAt.IsZero() {
		t.Error("expected collected timestamp")
	}
	if len(snap.Data.Blueprints) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(snap.Data.Blueprints))
	}
}

func TestCollector_SkipsEntitiesUnlessIncluded(t *testing.T) {
	var teamRequested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/teams":
			teamRequested = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "teams": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	collector := NewCollector(client)

	_, err := collector.Collect(context.Background(), "production", CompareCollectPlan([]string{"blueprints"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamRequested {
		t.Error("teams should not be requested when only blueprints included")
	}
}

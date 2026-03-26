package export

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestApplyBlueprintExclusions_Deep(t *testing.T) {
	all := []api.Blueprint{
		{"identifier": "service"},
		{"identifier": "_rule_result"},
		{"identifier": "domain"},
	}
	iterList, dataList := ApplyBlueprintExclusions(all, []string{"_rule_result"}, nil)
	// Deep exclusion: removed from both iteration list and data list
	if len(iterList) != 2 {
		t.Errorf("iterList: expected 2, got %d", len(iterList))
	}
	if len(dataList) != 2 {
		t.Errorf("dataList: expected 2, got %d", len(dataList))
	}
	for _, bp := range iterList {
		if bp["identifier"] == "_rule_result" {
			t.Error("deep-excluded blueprint still in iterList")
		}
	}
}

func TestApplyBlueprintExclusions_SchemaOnly(t *testing.T) {
	all := []api.Blueprint{
		{"identifier": "service"},
		{"identifier": "_rule_result"},
	}
	iterList, dataList := ApplyBlueprintExclusions(all, nil, []string{"_rule_result"})
	// Schema-only: removed from data list, but KEPT in iteration list (so entities/scorecards/actions still fetched)
	if len(iterList) != 2 {
		t.Errorf("iterList: expected 2 (schema-only keeps blueprint for fetching), got %d", len(iterList))
	}
	if len(dataList) != 1 {
		t.Errorf("dataList: expected 1 (schema excluded from output), got %d", len(dataList))
	}
	for _, bp := range dataList {
		if bp["identifier"] == "_rule_result" {
			t.Error("schema-excluded blueprint still in dataList")
		}
	}
}

func TestApplyBlueprintExclusions_OverlapDeepWins(t *testing.T) {
	all := []api.Blueprint{
		{"identifier": "service"},
		{"identifier": "overlap"},
	}
	iterList, dataList := ApplyBlueprintExclusions(all, []string{"overlap"}, []string{"overlap"})
	if len(iterList) != 1 || len(dataList) != 1 {
		t.Errorf("deep should win when id appears in both sets: iterList=%d dataList=%d", len(iterList), len(dataList))
	}
}

func TestApplyBlueprintExclusions_Empty(t *testing.T) {
	all := []api.Blueprint{{"identifier": "service"}}
	iterList, dataList := ApplyBlueprintExclusions(all, nil, nil)
	if len(iterList) != 1 || len(dataList) != 1 {
		t.Error("empty exclusion lists should return unchanged slices")
	}
}

func TestCollector_CollectsBlueprintPermissions(t *testing.T) {
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":          true,
				"permissions": map[string]interface{}{"entities": map[string]interface{}{"view": []string{"$team"}}},
			})
		case "/blueprints/service/scorecards":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "scorecards": []interface{}{}})
		case "/blueprints/service/actions":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "actions": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	data, err := collector.Collect(context.Background(), Options{SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.BlueprintPermissions["service"] == nil {
		t.Error("expected blueprint permissions for 'service'")
	}
}

func TestCollector_CollectsActionPermissions(t *testing.T) {
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      true,
				"actions": []map[string]interface{}{{"identifier": "deploy", "title": "Deploy"}},
			})
		case "/actions/deploy/permissions":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":          true,
				"permissions": map[string]interface{}{"execute": map[string]interface{}{"users": []string{}}},
			})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	data, err := collector.Collect(context.Background(), Options{SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.ActionPermissions["deploy"] == nil {
		t.Error("expected action permissions for 'deploy'")
	}
}

func TestCollector_SkipSystemBlueprints_ExcludesSchemaAndEntities(t *testing.T) {
	entitiesHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user", "title": "User"},
					{"identifier": "service", "title": "Service"},
				},
			})
		case "/blueprints/_user/entities":
			entitiesHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	data, err := collector.Collect(context.Background(), Options{SkipSystemBlueprints: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _user schema should NOT be in output blueprints
	for _, bp := range data.Blueprints {
		id, _ := bp["identifier"].(string)
		if id == "_user" {
			t.Error("_user blueprint schema should be excluded from output when SkipSystemBlueprints=true")
		}
	}

	// service blueprint should still be present
	found := false
	for _, bp := range data.Blueprints {
		if bp["identifier"] == "service" {
			found = true
		}
	}
	if !found {
		t.Error("non-system blueprint 'service' should remain in output")
	}

	// entities endpoint for _user should NOT be called
	if entitiesHit {
		t.Error("entities endpoint for _user should not be called when SkipSystemBlueprints=true")
	}
}

func TestCollector_SkipSystemBlueprints_StillCollectsScorecards(t *testing.T) {
	scorecardsHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_user", "title": "User"},
				},
			})
		case "/blueprints/_user/scorecards":
			scorecardsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "scorecards": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	_, err := collector.Collect(context.Background(), Options{SkipSystemBlueprints: true, SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scorecardsHit {
		t.Error("scorecards endpoint for _user should still be called when SkipSystemBlueprints=true (shallow skip)")
	}
}

func TestCollector_SkipEntities_SkipsTeamsAndUsers(t *testing.T) {
	teamsHit := false
	usersHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": []interface{}{}})
		case "/teams":
			teamsHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "teams": []interface{}{}})
		case "/users":
			usersHit = true
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		}
	}))
	defer server.Close()

	client := api.NewClient("id", "secret", server.URL, 0)
	collector := NewCollector(client)
	_, err := collector.Collect(context.Background(), Options{SkipEntities: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsHit {
		t.Error("teams endpoint should not be called when SkipEntities=true")
	}
	if usersHit {
		t.Error("users endpoint should not be called when SkipEntities=true")
	}
}

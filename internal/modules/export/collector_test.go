package export

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

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

package skills

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestDoctor_UnprefixedVersioned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/blueprints" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "skill_group"},
					{"identifier": "skill"},
					{"identifier": "skill_version"},
					{"identifier": "skill_file"},
				},
			})
			return
		}
		switch r.URL.Path {
		case "/blueprints/skill_group/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []map[string]interface{}{{"identifier": "g1"}}})
		case "/blueprints/skill/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []map[string]interface{}{{"identifier": "s1"}, {"identifier": "s2"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	mod := NewModule(nil, &config.OrganizationConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		APIURL:       server.URL,
	}, config.NewConfigManager(t.TempDir()+"/config.yaml"))

	result, err := mod.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if result.Family != "unprefixed" {
		t.Fatalf("family: got %q", result.Family)
	}
	if result.ContentModel != "versioned" {
		t.Fatalf("content model: got %q", result.ContentModel)
	}
	if result.Blueprints.SkillGroup != "skill_group" || result.Blueprints.Skill != "skill" {
		t.Fatalf("blueprints: %+v", result.Blueprints)
	}
	if result.GroupCount != 1 || result.SkillCount != 2 {
		t.Fatalf("counts: groups=%d skills=%d", result.GroupCount, result.SkillCount)
	}
}

func TestDoctor_PrefixedLegacy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/blueprints" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "_skill_group"},
					{"identifier": "_skill"},
				},
			})
			return
		}
		switch r.URL.Path {
		case "/blueprints/_skill_group/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []map[string]interface{}{}})
		case "/blueprints/_skill/entities/search":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "entities": []map[string]interface{}{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	mod := NewModule(nil, &config.OrganizationConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		APIURL:       server.URL,
	}, config.NewConfigManager(t.TempDir()+"/config.yaml"))

	result, err := mod.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if result.Family != "prefixed" {
		t.Fatalf("family: got %q", result.Family)
	}
	if result.ContentModel != "legacy" {
		t.Fatalf("content model: got %q", result.ContentModel)
	}
	if result.Blueprints.SkillVersion != "" || result.Blueprints.SkillFile != "" {
		t.Fatalf("expected no version/file blueprints, got %+v", result.Blueprints)
	}
}

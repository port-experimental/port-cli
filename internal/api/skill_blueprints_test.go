package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillBlueprintSet_FamilyAndContentModel(t *testing.T) {
	prefixed := SkillBlueprintSet{SkillGroup: "_skill_group", Skill: "_skill", SkillVersion: "_skill_version", SkillFile: "_skill_file"}
	if prefixed.Family() != "prefixed" || prefixed.ContentModel() != "versioned" {
		t.Fatalf("prefixed versioned: family=%q model=%q", prefixed.Family(), prefixed.ContentModel())
	}

	legacy := SkillBlueprintSet{SkillGroup: "skill_group", Skill: "skill"}
	if legacy.Family() != "unprefixed" || legacy.ContentModel() != "legacy" {
		t.Fatalf("unprefixed legacy: family=%q model=%q", legacy.Family(), legacy.ContentModel())
	}
}

func blueprintListHandler(ids ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path != "/blueprints" {
			http.NotFound(w, r)
			return
		}
		bps := make([]map[string]interface{}, len(ids))
		for i, id := range ids {
			bps[i] = map[string]interface{}{"identifier": id}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "blueprints": bps})
	}
}

func TestResolveSkillBlueprints_PrefixedWins(t *testing.T) {
	server := httptest.NewServer(blueprintListHandler(
		"_skill_group", "_skill", "_skill_version", "_skill_file",
		"skill_group", "skill", "skill_version", "skill_file",
	))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	set, err := client.ResolveSkillBlueprints(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.SkillGroup != "_skill_group" || set.Skill != "_skill" {
		t.Fatalf("expected prefixed group/skill, got %+v", set)
	}
	if set.SkillVersion != "_skill_version" || set.SkillFile != "_skill_file" {
		t.Fatalf("expected prefixed version/file, got %+v", set)
	}
}

func TestResolveSkillBlueprints_UnprefixedFallback(t *testing.T) {
	server := httptest.NewServer(blueprintListHandler(
		"skill_group", "skill", "skill_version", "skill_file",
	))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	set, err := client.ResolveSkillBlueprints(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.SkillGroup != "skill_group" || set.Skill != "skill" {
		t.Fatalf("expected unprefixed group/skill, got %+v", set)
	}
	if set.SkillVersion != "skill_version" || set.SkillFile != "skill_file" {
		t.Fatalf("expected unprefixed version/file, got %+v", set)
	}
}

func TestResolveSkillBlueprints_PartialVersionBlueprints(t *testing.T) {
	server := httptest.NewServer(blueprintListHandler("skill_group", "skill", "skill_version"))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	set, err := client.ResolveSkillBlueprints(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.SkillVersion != "skill_version" {
		t.Fatalf("expected skill_version, got %q", set.SkillVersion)
	}
	if set.SkillFile != "" {
		t.Fatalf("expected empty skill file blueprint, got %q", set.SkillFile)
	}
	if set.HasVersionedBlueprints() {
		t.Fatal("expected HasVersionedBlueprints false without skill_file")
	}
}

func TestResolveSkillBlueprints_NeitherPairErrors(t *testing.T) {
	server := httptest.NewServer(blueprintListHandler("service", "_user"))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	_, err := client.ResolveSkillBlueprints(context.Background())
	if err == nil {
		t.Fatal("expected error when no skills blueprints exist")
	}
}

func TestResolveSkillBlueprints_Cached(t *testing.T) {
	var blueprintCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/blueprints" {
			blueprintCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"blueprints": []map[string]interface{}{
					{"identifier": "skill_group"},
					{"identifier": "skill"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	if _, err := client.ResolveSkillBlueprints(context.Background()); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if _, err := client.ResolveSkillBlueprints(context.Background()); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if blueprintCalls != 1 {
		t.Fatalf("expected single GetBlueprints call, got %d", blueprintCalls)
	}
}

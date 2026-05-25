package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/auth"
)

func TestGetSkillVersionsForSkills_UsesTopSearchSort(t *testing.T) {
	var requestPath string
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"entities": []map[string]interface{}{
				{"identifier": "version-2"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	entities, err := client.GetSkillVersionsForSkills(context.Background(), []string{"skill-a", "skill-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0]["identifier"] != "version-2" {
		t.Fatalf("unexpected entities: %+v", entities)
	}
	if requestPath != "/blueprints/skill_version/entities/top-search" {
		t.Fatalf("expected top-search endpoint, got %s", requestPath)
	}
	if requestBody["limit"] != float64(1000) {
		t.Errorf("expected limit 1000, got %#v", requestBody["limit"])
	}
	sort, ok := requestBody["sort"].([]interface{})
	if !ok || len(sort) != 1 {
		t.Fatalf("expected one sort rule, got %#v", requestBody["sort"])
	}
	sortRule, ok := sort[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected sort rule: %#v", sort[0])
	}
	if sortRule["property"] != "version" || sortRule["order"] != "desc" {
		t.Errorf("unexpected sort rule: %#v", sortRule)
	}
	query := requestBody["query"].(map[string]interface{})
	rules := query["rules"].([]interface{})
	rule := rules[0].(map[string]interface{})
	if rule["operator"] != "matchAny" {
		t.Errorf("expected matchAny rule, got %#v", rule)
	}
	value := rule["value"].([]interface{})
	if len(value) != 2 || value[0] != "skill-a" || value[1] != "skill-b" {
		t.Errorf("unexpected skill filter value: %#v", value)
	}
}

func TestGetSkillFilesForVersions_UsesRelationPathSearch(t *testing.T) {
	var requestPath string
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"entities": []map[string]interface{}{
				{"identifier": "file-1"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	entities, err := client.GetSkillFilesForVersions(context.Background(), []string{"version-a", "version-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0]["identifier"] != "file-1" {
		t.Fatalf("unexpected entities: %+v", entities)
	}
	if requestPath != "/blueprints/skill_file/entities/search" {
		t.Fatalf("expected search endpoint, got %s", requestPath)
	}
	query := requestBody["query"].(map[string]interface{})
	rules := query["rules"].([]interface{})
	rule := rules[0].(map[string]interface{})
	if rule["operator"] != "matchAny" {
		t.Errorf("expected matchAny rule, got %#v", rule)
	}
	value := rule["value"].([]interface{})
	if len(value) != 2 || value[0] != "version-a" || value[1] != "version-b" {
		t.Errorf("unexpected version filter value: %#v", value)
	}
}

func TestGetSkills_IncludesSkillGroupRelation(t *testing.T) {
	var requestPath string
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"entities": []map[string]interface{}{
				{"identifier": "skill-a"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	entities, err := client.GetSkills(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0]["identifier"] != "skill-a" {
		t.Fatalf("unexpected entities: %+v", entities)
	}
	if requestPath != "/blueprints/skill/entities/search" {
		t.Fatalf("expected skill search endpoint, got %s", requestPath)
	}
	include := requestBody["include"].([]interface{})
	found := false
	for _, item := range include {
		if item == "skill_to_skill_group" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected include to contain skill_to_skill_group, got %#v", include)
	}
}

func TestGetSkills_FallsBackToLegacyEntitiesWhenRelationMissing(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/blueprints/skill/entities/search" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      false,
				"error":   "invalid_request",
				"message": "Some of the properties you are trying to include are not valid: skill_to_skill_group",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"entities": []map[string]interface{}{
				{"identifier": "legacy-skill"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	entities, err := client.GetSkills(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0]["identifier"] != "legacy-skill" {
		t.Fatalf("unexpected entities: %+v", entities)
	}
	expected := []string{"/blueprints/skill/entities/search", "/blueprints/skill/entities"}
	if len(paths) != len(expected) {
		t.Fatalf("expected paths %v, got %v", expected, paths)
	}
	for i := range expected {
		if paths[i] != expected[i] {
			t.Fatalf("expected paths %v, got %v", expected, paths)
		}
	}
}

func TestGetBlueprintPermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/blueprints/service/permissions" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"permissions": map[string]interface{}{
					"entities": map[string]interface{}{"view": []string{"$team"}, "create": []string{"$admin"}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	perms, err := client.GetBlueprintPermissions(context.Background(), "service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms["entities"] == nil {
		t.Error("expected entities permissions")
	}
}

func TestGetActionPermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/actions/deploy/permissions" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"permissions": map[string]interface{}{
					"execute": map[string]interface{}{"users": []string{"alice@example.com"}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	perms, err := client.GetActionPermissions(context.Background(), "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms["execute"] == nil {
		t.Error("expected execute permissions")
	}
}

func TestUpdateBlueprintPermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/blueprints/service/permissions" {
			if r.Method != http.MethodPatch {
				http.Error(w, "expected PATCH", http.StatusMethodNotAllowed)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"permissions": map[string]interface{}{
					"entities": map[string]interface{}{"view": []string{"$admin"}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	perms, err := client.UpdateBlueprintPermissions(context.Background(), "service", Permissions{
		"entities": map[string]interface{}{"view": []string{"$admin"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms["entities"] == nil {
		t.Error("expected entities in updated permissions")
	}
}

func TestUpdateActionPermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if r.URL.Path == "/actions/deploy/permissions" {
			if r.Method != http.MethodPatch {
				http.Error(w, "expected PATCH", http.StatusMethodNotAllowed)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"permissions": map[string]interface{}{
					"execute": map[string]interface{}{"users": []string{"alice@example.com"}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	perms, err := client.UpdateActionPermissions(context.Background(), "deploy", Permissions{
		"execute": map[string]interface{}{"users": []string{"alice@example.com"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms["execute"] == nil {
		t.Error("expected execute in updated permissions")
	}
}

func TestGetBlueprintPermissionsWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
			t.Fatal("unexpected call to /auth/access_token")
			return
		}
		if r.URL.Path == "/blueprints/service/permissions" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"permissions": map[string]interface{}{
					"entities": map[string]interface{}{"view": []string{"$team"}, "create": []string{"$admin"}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	exp := time.Now().Add(time.Hour * 24).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                             "https://api.example.com",
		"exp":                             float64(exp),
		"https://api.example.com/email":   "user@test.com",
		"https://api.example.com/orgId":   "someOrgId",
		"https://api.example.com/orgName": "Org Name",
	})
	signed, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := auth.ParseToken(signed)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(ClientOpts{Token: parsed, APIURL: server.URL, Timeout: 0})
	perms, err := client.GetBlueprintPermissions(context.Background(), "service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms["entities"] == nil {
		t.Error("expected entities permissions")
	}
}

func TestCallGenericGETAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}

		if r.Method != "GET" {
			t.Fatalf("unexpected %s call", r.Method)
			return
		}
		if r.URL.Path == "/actions/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	res, err := client.Request(context.Background(), RequestParams{Method: "GET", Endpoint: "/actions/runs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res, ok := res.(map[string]any); ok && res["ok"] != true {
		t.Error("expected entities permissions")
	}
}

func TestCallGenericPOSTAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}

		if r.Method != "POST" {
			t.Fatalf("unexpected %s call", r.Method)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("error reading body %v", err)
			return
		}
		if string(body) != `{"properties":{}}` {
			t.Fatalf("unexpected body '%s'", string(body))
			return
		}
		if r.URL.Path == "/actions/my-action/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	res, err := client.Request(context.Background(), RequestParams{
		Method:   "POST",
		Data:     map[string]any{"properties": map[string]any{}},
		Endpoint: "/actions/my-action/runs",
	},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res, ok := res.(map[string]any); ok && res["ok"] != true {
		t.Error("expected entities permissions")
	}
}

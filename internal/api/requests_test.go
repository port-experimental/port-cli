package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/auth"
)

func TestForEachEntity_UsesGetWhenCountAtThreshold(t *testing.T) {
	var countCalls, getCalls, searchCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints/service/entities-count":
			countCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": 10000})
		case "/blueprints/service/entities":
			getCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"entities": []map[string]interface{}{
					{"identifier": "svc-1", "blueprint": "service"},
					{"identifier": "svc-2", "blueprint": "service"},
				},
			})
		case "/blueprints/service/entities/search":
			searchCalls++
			http.Error(w, "unexpected search call", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	var batches [][]Entity
	err := client.ForEachEntity(context.Background(), "service", func(entities []Entity) error {
		batches = append(batches, entities)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachEntity returned error: %v", err)
	}
	if countCalls != 1 {
		t.Fatalf("expected 1 count call, got %d", countCalls)
	}
	if getCalls != 1 {
		t.Fatalf("expected 1 GET entities call, got %d", getCalls)
	}
	if searchCalls != 0 {
		t.Fatalf("expected no search calls, got %d", searchCalls)
	}
	if len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("expected one batch with 2 entities, got %#v", batches)
	}
}

func TestForEachEntity_UsesPaginatedSearchWhenCountAboveThreshold(t *testing.T) {
	var countCalls, getCalls, searchCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/access_token":
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
		case "/blueprints/service/entities-count":
			countCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": 10001})
		case "/blueprints/service/entities":
			getCalls++
			http.Error(w, "unexpected GET entities call", http.StatusInternalServerError)
		case "/blueprints/service/entities/search":
			searchCalls++
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode search body: %v", err)
			}
			query, ok := body["query"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected wrapped query body, got %#v", body)
			}
			if query["combinator"] != "and" {
				t.Fatalf("expected and combinator, got %#v", query["combinator"])
			}
			if _, ok := query["rules"].([]interface{}); !ok {
				t.Fatalf("expected query.rules array, got %#v", query["rules"])
			}
			if body["limit"] != float64(1000) {
				t.Fatalf("expected limit 1000, got %#v", body["limit"])
			}
			switch searchCalls {
			case 1:
				if _, ok := body["from"]; ok {
					t.Fatalf("first search request should not include from: %#v", body)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":       true,
					"next":     "cursor-1",
					"entities": []map[string]interface{}{{"identifier": "svc-1", "blueprint": "service"}},
				})
			case 2:
				if body["from"] != "cursor-1" {
					t.Fatalf("expected cursor-1, got %#v", body["from"])
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":       true,
					"entities": []map[string]interface{}{{"identifier": "svc-2", "blueprint": "service"}},
				})
			default:
				http.Error(w, "unexpected extra search call", http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})
	var identifiers []string
	err := client.ForEachEntity(context.Background(), "service", func(entities []Entity) error {
		for _, entity := range entities {
			id, _ := entity["identifier"].(string)
			identifiers = append(identifiers, id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachEntity returned error: %v", err)
	}
	if countCalls != 1 {
		t.Fatalf("expected 1 count call, got %d", countCalls)
	}
	if getCalls != 0 {
		t.Fatalf("expected no GET entities calls, got %d", getCalls)
	}
	if searchCalls != 2 {
		t.Fatalf("expected 2 search calls, got %d", searchCalls)
	}
	if got, want := strings.Join(identifiers, ","), "svc-1,svc-2"; got != want {
		t.Fatalf("expected identifiers %q, got %q", want, got)
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

func TestBulkDeleteEntities_Success(t *testing.T) {
	var requestPath string
	var requestBody map[string]interface{}
	var requestMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		requestPath = r.URL.Path
		requestMethod = r.Method

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})

	identifiers := []string{"id1", "id2"}

	res, err := client.BulkDeleteEntities(context.Background(), "my-blueprint", identifiers, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if requestMethod != "POST" {
		t.Errorf("expected POST requests, got %v", requestMethod)
	}
	if requestPath != "/blueprints/my-blueprint/bulk/entities/delete" {
		t.Errorf("expected path /blueprints/my-blueprint/bulk/entities/delete, got %v", requestPath)
	}
	entities, ok := requestBody["entities"].([]interface{})
	if !ok || len(entities) != 2 || entities[0] != "id1" || entities[1] != "id2" {
		t.Errorf("expected entities [id1, id2], got %v", requestBody["entities"])
	}
	if res["ok"] != true {
		t.Errorf("expected ok: true, got %v", res)
	}
}

func TestBulkDeleteEntities_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"error":   "internal_error",
			"message": "Something went wrong",
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL, Timeout: 0})

	_, err := client.BulkDeleteEntities(context.Background(), "my-blueprint", []string{"id1"}, true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

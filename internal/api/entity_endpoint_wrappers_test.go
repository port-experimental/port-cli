package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEntityEndpointWrappers(t *testing.T) {
	entity := Entity{"identifier": "svc", "blueprint": "service"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "get", call: func(ctx context.Context, c *Client) error { _, err := c.GetEntity(ctx, "service", "svc"); return err }, method: http.MethodGet, path: "/blueprints/service/entities/svc", resp: map[string]interface{}{"entity": entity}},
		{name: "create", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateEntity(ctx, "service", entity)
			return err
		}, method: http.MethodPost, path: "/blueprints/service/entities", body: true, resp: map[string]interface{}{"entity": entity}},
		{name: "update", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateEntity(ctx, "service", "svc", entity)
			return err
		}, method: http.MethodPut, path: "/blueprints/service/entities/svc", body: true, resp: map[string]interface{}{"entity": entity}},
		{name: "delete", call: func(ctx context.Context, c *Client) error { return c.DeleteEntity(ctx, "service", "svc") }, method: http.MethodDelete, path: "/blueprints/service/entities/svc", resp: map[string]interface{}{"ok": true}},
		{name: "search", call: func(ctx context.Context, c *Client) error {
			_, err := c.SearchEntities(ctx, "service", map[string]interface{}{"limit": 1000})
			return err
		}, method: http.MethodPost, path: "/blueprints/service/entities/search", body: true, resp: map[string]interface{}{"entities": []Entity{entity}}},
		{name: "top search", call: func(ctx context.Context, c *Client) error {
			_, err := c.TopSearchEntities(ctx, "service", map[string]interface{}{"limit": 10})
			return err
		}, method: http.MethodPost, path: "/blueprints/service/entities/top-search", body: true, resp: map[string]interface{}{"entities": []Entity{entity}}},
	})
}

func TestBulkUpsertEntities(t *testing.T) {
	var gotPath string
	var gotUpsert string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		gotPath = r.URL.Path
		gotUpsert = r.URL.Query().Get("upsert")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []map[string]interface{}{
				{"identifier": "svc-1", "statusCode": 409.0, "error": "conflict", "message": "already exists", "index": 0.0},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: srv.URL})
	entities := []Entity{
		{"identifier": "svc-1", "blueprint": "service"},
		{"identifier": "svc-2", "blueprint": "service"},
	}

	errs, err := client.BulkUpsertEntities(context.Background(), "service", entities, true)
	if err != nil {
		t.Fatalf("BulkUpsertEntities failed: %v", err)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Identifier != "svc-1" {
		t.Fatalf("unexpected identifier: %s", errs[0].Identifier)
	}
	if gotPath != "/blueprints/service/entities/bulk" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotUpsert != "true" {
		t.Fatalf("unexpected upsert query: %s", gotUpsert)
	}
}

func TestCreateUserEntitiesBulk_UsesGenericBulkEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": []interface{}{}})
	}))
	defer srv.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: srv.URL})
	_, err := client.CreateUserEntitiesBulk(context.Background(), []Entity{{"identifier": "u@example.com"}}, false)
	if err != nil {
		t.Fatalf("CreateUserEntitiesBulk failed: %v", err)
	}
	if gotPath != "/blueprints/_user/entities/bulk" {
		t.Fatalf("expected _user bulk path, got %s", gotPath)
	}
}

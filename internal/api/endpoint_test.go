package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDoJSON(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		params     map[string]string
		response   map[string]interface{}
		wantDecode bool
		wantField  string
	}{
		{
			name:       "get with query params",
			method:     http.MethodGet,
			path:       "/pages",
			params:     map[string]string{"limit": "10"},
			response:   map[string]interface{}{"pages": []map[string]interface{}{{"identifier": "home"}}},
			wantDecode: true,
			wantField:  "pages",
		},
		{
			name:     "delete no content",
			method:   http.MethodDelete,
			path:     "/blueprints/service",
			response: map[string]interface{}{"ok": true},
		},
		{
			name:       "post with body",
			method:     http.MethodPost,
			path:       "/blueprints",
			body:       Blueprint{"identifier": "service"},
			response:   map[string]interface{}{"blueprint": map[string]interface{}{"identifier": "service"}},
			wantDecode: true,
			wantField:  "blueprint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Fatalf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tt.path)
				}
				if tt.params != nil {
					for k, v := range tt.params {
						if got := r.URL.Query().Get(k); got != v {
							t.Fatalf("query %s = %q, want %q", k, got, v)
						}
					}
				}
				if tt.body != nil {
					var decoded map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
						t.Fatalf("decode request body: %v", err)
					}
					if len(decoded) == 0 {
						t.Fatal("expected non-empty request body")
					}
				}
				_ = json.NewEncoder(w).Encode(tt.response)
			})

			if tt.wantDecode {
				var out map[string]json.RawMessage
				if err := client.DoJSON(context.Background(), tt.method, tt.path, tt.body, tt.params, &out); err != nil {
					t.Fatalf("DoJSON failed: %v", err)
				}
				if _, ok := out[tt.wantField]; !ok {
					t.Fatalf("expected decoded field %q, got keys %#v", tt.wantField, out)
				}
				return
			}

			if err := client.DoJSON(context.Background(), tt.method, tt.path, tt.body, tt.params, nil); err != nil {
				t.Fatalf("DoJSON failed: %v", err)
			}
		})
	}
}

func TestDecodeEnvelope(t *testing.T) {
	raw := map[string]json.RawMessage{
		"blueprint": json.RawMessage(`{"identifier":"service"}`),
	}
	got, err := decodeEnvelope[Blueprint](raw, "blueprint", "failed to decode blueprint")
	if err != nil {
		t.Fatalf("decodeEnvelope failed: %v", err)
	}
	if got["identifier"] != "service" {
		t.Fatalf("unexpected blueprint: %#v", got)
	}

	missing, err := decodeEnvelope[[]Blueprint](map[string]json.RawMessage{}, "blueprints", "failed to decode blueprints")
	if err != nil {
		t.Fatalf("missing envelope key should return zero value: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil slice for missing key, got %#v", missing)
	}

	_, err = decodeEnvelope[Blueprint](map[string]json.RawMessage{"blueprint": json.RawMessage(`{`)}, "blueprint", "failed to decode blueprint")
	if err == nil {
		t.Fatal("expected error for invalid envelope payload")
	}
}

func TestDoEnvelopeBlueprintAndPageWrappers(t *testing.T) {
	blueprint := Blueprint{"identifier": "service"}
	page := Page{"identifier": "home"}

	tests := []struct {
		name       string
		call       func(context.Context, *Client) (any, error)
		method     string
		path       string
		body       bool
		envelope   string
		wantID     string
	}{
		{
			name: "get blueprints",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.GetBlueprints(ctx)
			},
			method: http.MethodGet, path: "/blueprints", envelope: "blueprints",
		},
		{
			name: "get blueprint",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.GetBlueprint(ctx, "service")
			},
			method: http.MethodGet, path: "/blueprints/service", envelope: "blueprint", wantID: "service",
		},
		{
			name: "create blueprint",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateBlueprint(ctx, blueprint)
			},
			method: http.MethodPost, path: "/blueprints", body: true, envelope: "blueprint", wantID: "service",
		},
		{
			name: "get pages",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.GetPages(ctx)
			},
			method: http.MethodGet, path: "/pages", envelope: "pages",
		},
		{
			name: "get page",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.GetPage(ctx, "home")
			},
			method: http.MethodGet, path: "/pages/home", envelope: "page", wantID: "home",
		},
		{
			name: "update page",
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.UpdatePage(ctx, "home", page)
			},
			method: http.MethodPatch, path: "/pages/home", body: true, envelope: "page", wantID: "home",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.Path != tt.path {
					t.Fatalf("got %s %s, want %s %s", r.Method, r.URL.Path, tt.method, tt.path)
				}
				if tt.body {
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatalf("decode body: %v", err)
					}
				}
				payload := map[string]interface{}{}
				switch tt.envelope {
				case "blueprints":
					payload["blueprints"] = []Blueprint{blueprint}
				case "blueprint":
					payload["blueprint"] = blueprint
				case "pages":
					payload["pages"] = []Page{page}
				case "page":
					payload["page"] = page
				}
				_ = json.NewEncoder(w).Encode(payload)
			})

			result, err := tt.call(context.Background(), client)
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if tt.wantID == "" {
				return
			}
			switch v := result.(type) {
			case Blueprint:
				if v["identifier"] != tt.wantID {
					t.Fatalf("identifier = %v, want %s", v["identifier"], tt.wantID)
				}
			case Page:
				if v["identifier"] != tt.wantID {
					t.Fatalf("identifier = %v, want %s", v["identifier"], tt.wantID)
				}
			default:
				t.Fatalf("unexpected result type %T", result)
			}
		})
	}
}

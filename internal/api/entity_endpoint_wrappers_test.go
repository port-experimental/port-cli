package api

import (
	"context"
	"net/http"
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

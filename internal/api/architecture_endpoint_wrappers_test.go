package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

type endpointWrapperCase struct {
	name   string
	call   func(context.Context, *Client) error
	method string
	path   string
	body   bool
	resp   map[string]interface{}
}

func runEndpointWrapperCases(t *testing.T, tests []endpointWrapperCase) {
	t.Helper()
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
					if len(body) == 0 {
						t.Fatal("expected non-empty JSON body")
					}
				}
				_ = json.NewEncoder(w).Encode(tt.resp)
			})
			if err := tt.call(context.Background(), client); err != nil {
				t.Fatalf("call failed: %v", err)
			}
		})
	}
}

func TestPageEndpointWrappers(t *testing.T) {
	page := Page{"identifier": "home"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list", call: func(ctx context.Context, c *Client) error { _, err := c.GetPages(ctx); return err }, method: http.MethodGet, path: "/pages", resp: map[string]interface{}{"pages": []Page{page}}},
		{name: "get", call: func(ctx context.Context, c *Client) error { _, err := c.GetPage(ctx, "home"); return err }, method: http.MethodGet, path: "/pages/home", resp: map[string]interface{}{"page": page}},
		{name: "create", call: func(ctx context.Context, c *Client) error { _, err := c.CreatePage(ctx, page); return err }, method: http.MethodPost, path: "/pages", body: true, resp: map[string]interface{}{"page": page}},
		{name: "update", call: func(ctx context.Context, c *Client) error { _, err := c.UpdatePage(ctx, "home", page); return err }, method: http.MethodPatch, path: "/pages/home", body: true, resp: map[string]interface{}{"page": page}},
		{name: "delete", call: func(ctx context.Context, c *Client) error { return c.DeletePage(ctx, "home") }, method: http.MethodDelete, path: "/pages/home", resp: map[string]interface{}{"ok": true}},
	})
}

func TestActionAndScorecardEndpointWrappers(t *testing.T) {
	action := Action{"identifier": "deploy"}
	scorecard := Scorecard{"identifier": "quality"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list actions", call: func(ctx context.Context, c *Client) error { _, err := c.GetActions(ctx, "service"); return err }, method: http.MethodGet, path: "/blueprints/service/actions", resp: map[string]interface{}{"actions": []Action{action}}},
		{name: "create action", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateAction(ctx, "service", action)
			return err
		}, method: http.MethodPost, path: "/blueprints/service/actions", body: true, resp: map[string]interface{}{"action": action}},
		{name: "update action", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateAction(ctx, "service", "deploy", action)
			return err
		}, method: http.MethodPatch, path: "/blueprints/service/actions/deploy", body: true, resp: map[string]interface{}{"action": action}},
		{name: "delete action", call: func(ctx context.Context, c *Client) error { return c.DeleteAction(ctx, "service", "deploy") }, method: http.MethodDelete, path: "/blueprints/service/actions/deploy", resp: map[string]interface{}{"ok": true}},
		{name: "list scorecards", call: func(ctx context.Context, c *Client) error { _, err := c.GetScorecards(ctx, "service"); return err }, method: http.MethodGet, path: "/blueprints/service/scorecards", resp: map[string]interface{}{"scorecards": []Scorecard{scorecard}}},
		{name: "create scorecard", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateScorecard(ctx, "service", scorecard)
			return err
		}, method: http.MethodPost, path: "/blueprints/service/scorecards", body: true, resp: map[string]interface{}{"scorecard": scorecard}},
		{name: "update scorecard", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateScorecard(ctx, "service", "quality", scorecard)
			return err
		}, method: http.MethodPatch, path: "/blueprints/service/scorecards/quality", body: true, resp: map[string]interface{}{"scorecard": scorecard}},
		{name: "delete scorecard", call: func(ctx context.Context, c *Client) error { return c.DeleteScorecard(ctx, "service", "quality") }, method: http.MethodDelete, path: "/blueprints/service/scorecards/quality", resp: map[string]interface{}{"ok": true}},
	})
}

func TestPermissionsEndpointWrappers(t *testing.T) {
	perms := Permissions{"read": []interface{}{"Everyone"}}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "get blueprint", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetBlueprintPermissions(ctx, "service")
			return err
		}, method: http.MethodGet, path: "/blueprints/service/permissions", resp: map[string]interface{}{"permissions": perms}},
		{name: "update blueprint", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateBlueprintPermissions(ctx, "service", perms)
			return err
		}, method: http.MethodPatch, path: "/blueprints/service/permissions", body: true, resp: map[string]interface{}{"permissions": perms}},
		{name: "get action", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetActionPermissions(ctx, "deploy")
			return err
		}, method: http.MethodGet, path: "/actions/deploy/permissions", resp: map[string]interface{}{"permissions": perms}},
		{name: "update action", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateActionPermissions(ctx, "deploy", perms)
			return err
		}, method: http.MethodPatch, path: "/actions/deploy/permissions", body: true, resp: map[string]interface{}{"permissions": perms}},
		{name: "get page", call: func(ctx context.Context, c *Client) error { _, err := c.GetPagePermissions(ctx, "home"); return err }, method: http.MethodGet, path: "/pages/home/permissions", resp: map[string]interface{}{"permissions": perms}},
		{name: "update page", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdatePagePermissions(ctx, "home", perms)
			return err
		}, method: http.MethodPatch, path: "/pages/home/permissions", body: true, resp: map[string]interface{}{"permissions": perms}},
	})
}

func TestWebhookEndpointWrappers(t *testing.T) {
	webhook := Webhook{"identifier": "hook"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list", call: func(ctx context.Context, c *Client) error { _, err := c.GetWebhooks(ctx); return err }, method: http.MethodGet, path: "/webhooks", resp: map[string]interface{}{"webhooks": []Webhook{webhook}}},
		{name: "get", call: func(ctx context.Context, c *Client) error { _, err := c.GetWebhook(ctx, "hook"); return err }, method: http.MethodGet, path: "/webhooks/hook", resp: map[string]interface{}{"webhook": webhook}},
		{name: "create", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateWebhook(ctx, map[string]interface{}{"identifier": "hook"})
			return err
		}, method: http.MethodPost, path: "/webhooks", body: true, resp: map[string]interface{}{"webhook": webhook}},
		{name: "update", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateWebhook(ctx, "hook", map[string]interface{}{"identifier": "hook"})
			return err
		}, method: http.MethodPatch, path: "/webhooks/hook", body: true, resp: map[string]interface{}{"webhook": webhook}},
		{name: "delete", call: func(ctx context.Context, c *Client) error { return c.DeleteWebhook(ctx, "hook") }, method: http.MethodDelete, path: "/webhooks/hook", resp: map[string]interface{}{"ok": true}},
	})
}

func TestTeamAndUserEndpointWrappers(t *testing.T) {
	team := Team{"name": "platform"}
	user := User{"email": "dev@example.com"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list teams", call: func(ctx context.Context, c *Client) error { _, err := c.GetTeams(ctx); return err }, method: http.MethodGet, path: "/teams", resp: map[string]interface{}{"teams": []Team{team}}},
		{name: "create team", call: func(ctx context.Context, c *Client) error { _, err := c.CreateTeam(ctx, team); return err }, method: http.MethodPost, path: "/teams", body: true, resp: map[string]interface{}{"team": team}},
		{name: "update team", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateTeam(ctx, "platform", team); return err }, method: http.MethodPatch, path: "/teams/platform", body: true, resp: map[string]interface{}{"team": team}},
		{name: "delete team", call: func(ctx context.Context, c *Client) error { return c.DeleteTeam(ctx, "platform") }, method: http.MethodDelete, path: "/teams/platform", resp: map[string]interface{}{"ok": true}},
		{name: "list users", call: func(ctx context.Context, c *Client) error { _, err := c.GetUsers(ctx); return err }, method: http.MethodGet, path: "/users", resp: map[string]interface{}{"users": []User{user}}},
		{name: "get user", call: func(ctx context.Context, c *Client) error { _, err := c.GetUser(ctx, "dev@example.com"); return err }, method: http.MethodGet, path: "/users/dev@example.com", resp: map[string]interface{}{"user": user}},
	})
}

func TestIntegrationAndActionRunEndpointWrappers(t *testing.T) {
	integration := Integration{"identifier": "github"}
	run := ActionRun{"id": "run-1"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list integrations", call: func(ctx context.Context, c *Client) error { _, err := c.GetIntegrations(ctx); return err }, method: http.MethodGet, path: "/integration", resp: map[string]interface{}{"integrations": []Integration{integration}}},
		{name: "update integration", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateIntegrationConfig(ctx, "github", map[string]interface{}{"enabled": true})
			return err
		}, method: http.MethodPatch, path: "/integration/github/config", body: true, resp: map[string]interface{}{"integration": integration}},
		{name: "delete integration", call: func(ctx context.Context, c *Client) error { return c.DeleteIntegration(ctx, "github") }, method: http.MethodDelete, path: "/integration/github", resp: map[string]interface{}{"ok": true}},
		{name: "list action runs", call: func(ctx context.Context, c *Client) error { _, err := c.GetActionRuns(ctx); return err }, method: http.MethodGet, path: "/actions/runs", resp: map[string]interface{}{"runs": []ActionRun{run}}},
		{name: "get action run", call: func(ctx context.Context, c *Client) error { _, err := c.GetActionRun(ctx, "run-1"); return err }, method: http.MethodGet, path: "/actions/runs/run-1", resp: map[string]interface{}{"run": run}},
	})
}

func TestAuditEndpointWrappers(t *testing.T) {
	entry := AuditLog{"action": "login"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list", call: func(ctx context.Context, c *Client) error { _, err := c.GetAuditLogs(ctx); return err }, method: http.MethodGet, path: "/audit-log", resp: map[string]interface{}{"audits": []AuditLog{entry}}},
	})
}

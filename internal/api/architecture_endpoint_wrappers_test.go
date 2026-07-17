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
		{name: "list actions", call: func(ctx context.Context, c *Client) error { _, err := c.GetActions(ctx, "service"); return err }, method: http.MethodGet, path: "/actions", resp: map[string]interface{}{"actions": []Action{action}}},
		{name: "create action", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateAction(ctx, "service", action)
			return err
		}, method: http.MethodPost, path: "/actions", body: true, resp: map[string]interface{}{"action": action}},
		{name: "update action", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateAction(ctx, "service", "deploy", action)
			return err
		}, method: http.MethodPut, path: "/actions/deploy", body: true, resp: map[string]interface{}{"action": action}},
		{name: "delete action", call: func(ctx context.Context, c *Client) error { return c.DeleteAction(ctx, "service", "deploy") }, method: http.MethodDelete, path: "/actions/deploy", resp: map[string]interface{}{"ok": true}},
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
		{name: "invite user", call: func(ctx context.Context, c *Client) error { _, err := c.InviteUser(ctx, user); return err }, method: http.MethodPost, path: "/users/invite", body: true, resp: map[string]interface{}{"user": user}},
		{name: "update user", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateUser(ctx, "dev@example.com", user); return err }, method: http.MethodPatch, path: "/users/dev@example.com", body: true, resp: map[string]interface{}{"user": user}},
		{name: "delete user", call: func(ctx context.Context, c *Client) error { return c.DeleteUser(ctx, "dev@example.com") }, method: http.MethodDelete, path: "/users/dev@example.com", resp: map[string]interface{}{"ok": true}},
		{name: "change account role", call: func(ctx context.Context, c *Client) error {
			_, err := c.ChangeUserAccountRole(ctx, "user-1", map[string]interface{}{"role": "Admin"})
			return err
		}, method: http.MethodPatch, path: "/users/user-1/account-role", body: true, resp: map[string]interface{}{"user": user}},
		{name: "change company role", call: func(ctx context.Context, c *Client) error {
			_, err := c.ChangeUserCompanyRole(ctx, "user-1", map[string]interface{}{"role": "Member"})
			return err
		}, method: http.MethodPatch, path: "/users/user-1/company-role", body: true, resp: map[string]interface{}{"user": user}},
	})
}

func TestIntegrationAndActionRunEndpointWrappers(t *testing.T) {
	integration := Integration{"identifier": "github"}
	run := ActionRun{"id": "run-1"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list integrations", call: func(ctx context.Context, c *Client) error { _, err := c.GetIntegrations(ctx); return err }, method: http.MethodGet, path: "/integration", resp: map[string]interface{}{"integrations": []Integration{integration}}},
		{name: "get integration", call: func(ctx context.Context, c *Client) error { _, err := c.GetIntegration(ctx, "github"); return err }, method: http.MethodGet, path: "/integration/github", resp: map[string]interface{}{"integration": integration}},
		{name: "update integration", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateIntegration(ctx, "github", integration)
			return err
		}, method: http.MethodPatch, path: "/integration/github", body: true, resp: map[string]interface{}{"integration": integration}},
		{name: "update integration config", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateIntegrationConfig(ctx, "github", map[string]interface{}{"enabled": true})
			return err
		}, method: http.MethodPatch, path: "/integration/github/config", body: true, resp: map[string]interface{}{"integration": integration}},
		{name: "delete integration", call: func(ctx context.Context, c *Client) error { return c.DeleteIntegration(ctx, "github") }, method: http.MethodDelete, path: "/integration/github", resp: map[string]interface{}{"ok": true}},
		{name: "list action runs", call: func(ctx context.Context, c *Client) error { _, err := c.GetActionRuns(ctx); return err }, method: http.MethodGet, path: "/actions/runs", resp: map[string]interface{}{"runs": []ActionRun{run}}},
		{name: "get action run", call: func(ctx context.Context, c *Client) error { _, err := c.GetActionRun(ctx, "run-1"); return err }, method: http.MethodGet, path: "/actions/runs/run-1", resp: map[string]interface{}{"run": run}},
	})
}

func TestMigrationEndpointWrappers(t *testing.T) {
	migration := Migration{"identifier": "mig-1", "status": "RUNNING"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list migrations", call: func(ctx context.Context, c *Client) error { _, err := c.GetMigrations(ctx, nil); return err }, method: http.MethodGet, path: "/migrations", resp: map[string]interface{}{"migrations": []Migration{migration}}},
		{name: "get migration", call: func(ctx context.Context, c *Client) error { _, err := c.GetMigration(ctx, "mig-1"); return err }, method: http.MethodGet, path: "/migrations/mig-1", resp: map[string]interface{}{"migration": migration}},
		{name: "create migration", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateMigration(ctx, MigrationRequest{
				SourceBlueprint: "service",
				Mapping:         map[string]interface{}{"blueprint": "service"},
			})
			return err
		}, method: http.MethodPost, path: "/migrations", body: true, resp: map[string]interface{}{"migration": migration}},
		{name: "cancel migration", call: func(ctx context.Context, c *Client) error { _, err := c.CancelMigration(ctx, "mig-1"); return err }, method: http.MethodPost, path: "/migrations/mig-1/cancel", resp: map[string]interface{}{"migration": migration}},
	})
}

func TestAuditEndpointWrappers(t *testing.T) {
	entry := AuditLog{"action": "login"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list", call: func(ctx context.Context, c *Client) error { _, err := c.GetAuditLogs(ctx); return err }, method: http.MethodGet, path: "/audit-log", resp: map[string]interface{}{"audits": []AuditLog{entry}}},
	})
}

func TestOrganizationAndSecretEndpointWrappers(t *testing.T) {
	org := Organization{"name": "acme"}
	secret := Secret{"secretName": "slack-token"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "get organization", call: func(ctx context.Context, c *Client) error { _, err := c.GetOrganization(ctx); return err }, method: http.MethodGet, path: "/organization", resp: map[string]interface{}{"organization": org}},
		{name: "update organization", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateOrganization(ctx, org); return err }, method: http.MethodPatch, path: "/organization", body: true, resp: map[string]interface{}{"organization": org}},
		{name: "replace organization", call: func(ctx context.Context, c *Client) error { _, err := c.ReplaceOrganization(ctx, org); return err }, method: http.MethodPut, path: "/organization", body: true, resp: map[string]interface{}{"organization": org}},
		{name: "list secrets", call: func(ctx context.Context, c *Client) error { _, err := c.GetSecrets(ctx); return err }, method: http.MethodGet, path: "/organization/secrets", resp: map[string]interface{}{"secrets": []Secret{secret}}},
		{name: "get secret", call: func(ctx context.Context, c *Client) error { _, err := c.GetSecret(ctx, "slack-token"); return err }, method: http.MethodGet, path: "/organization/secrets/slack-token", resp: map[string]interface{}{"secret": secret}},
		{name: "create secret", call: func(ctx context.Context, c *Client) error { _, err := c.CreateSecret(ctx, secret); return err }, method: http.MethodPost, path: "/organization/secrets", body: true, resp: map[string]interface{}{"secret": secret}},
		{name: "update secret", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateSecret(ctx, "slack-token", secret); return err }, method: http.MethodPatch, path: "/organization/secrets/slack-token", body: true, resp: map[string]interface{}{"secret": secret}},
		{name: "delete secret", call: func(ctx context.Context, c *Client) error { return c.DeleteSecret(ctx, "slack-token") }, method: http.MethodDelete, path: "/organization/secrets/slack-token", resp: map[string]interface{}{"ok": true}},
	})
}

func TestWorkflowEndpointWrappers(t *testing.T) {
	workflow := Workflow{"identifier": "deploy"}
	run := WorkflowRun{"identifier": "run-1"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list workflows", call: func(ctx context.Context, c *Client) error { _, err := c.GetWorkflows(ctx); return err }, method: http.MethodGet, path: "/workflows", resp: map[string]interface{}{"workflows": []Workflow{workflow}}},
		{name: "get workflow", call: func(ctx context.Context, c *Client) error { _, err := c.GetWorkflow(ctx, "deploy"); return err }, method: http.MethodGet, path: "/workflows/deploy", resp: map[string]interface{}{"workflow": workflow}},
		{name: "create workflow", call: func(ctx context.Context, c *Client) error { _, err := c.CreateWorkflow(ctx, workflow); return err }, method: http.MethodPost, path: "/workflows", body: true, resp: map[string]interface{}{"workflow": workflow}},
		{name: "update workflow", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateWorkflow(ctx, "deploy", workflow); return err }, method: http.MethodPut, path: "/workflows/deploy", body: true, resp: map[string]interface{}{"workflow": workflow}},
		{name: "delete workflow", call: func(ctx context.Context, c *Client) error { return c.DeleteWorkflow(ctx, "deploy") }, method: http.MethodDelete, path: "/workflows/deploy", resp: map[string]interface{}{"ok": true}},
		{name: "list workflow runs", call: func(ctx context.Context, c *Client) error { _, err := c.GetWorkflowRuns(ctx, nil); return err }, method: http.MethodGet, path: "/workflows/runs", resp: map[string]interface{}{"runs": []WorkflowRun{run}}},
		{name: "get workflow run", call: func(ctx context.Context, c *Client) error { _, err := c.GetWorkflowRun(ctx, "run-1"); return err }, method: http.MethodGet, path: "/workflows/runs/run-1", resp: map[string]interface{}{"run": run}},
		{name: "trigger workflow run", call: func(ctx context.Context, c *Client) error {
			_, err := c.TriggerWorkflowRun(ctx, "deploy", map[string]interface{}{"properties": map[string]interface{}{}})
			return err
		}, method: http.MethodPost, path: "/workflows/deploy/runs", body: true, resp: map[string]interface{}{"run": run}},
		{name: "cancel workflow run", call: func(ctx context.Context, c *Client) error { _, err := c.CancelWorkflowRun(ctx, "run-1"); return err }, method: http.MethodPost, path: "/workflows/runs/run-1/cancel", resp: map[string]interface{}{"run": run}},
		{name: "get workflow node", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetWorkflowNode(ctx, "deploy", "build")
			return err
		}, method: http.MethodGet, path: "/workflows/deploy/nodes/build", resp: map[string]interface{}{"node": WorkflowNode{"identifier": "build"}}},
		{name: "list self-service triggers", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetWorkflowSelfServiceTriggers(ctx)
			return err
		}, method: http.MethodGet, path: "/workflows/self-service-triggers", resp: map[string]interface{}{"triggers": []map[string]interface{}{{"identifier": "t1"}}}},
		{name: "get workflow run logs", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetWorkflowRunLogs(ctx, "run-1")
			return err
		}, method: http.MethodGet, path: "/workflows/runs/run-1/logs", resp: map[string]interface{}{"logs": []map[string]interface{}{{"message": "ok"}}}},
		{name: "get workflow node run logs", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetWorkflowNodeRunLogs(ctx, "node-run-1")
			return err
		}, method: http.MethodGet, path: "/workflows/nodes/runs/node-run-1/logs", resp: map[string]interface{}{"logs": []map[string]interface{}{{"message": "ok"}}}},
		{name: "update workflow node run", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateWorkflowNodeRun(ctx, "node-run-1", map[string]interface{}{"status": "SUCCESS"})
			return err
		}, method: http.MethodPatch, path: "/workflows/nodes/runs/node-run-1", body: true, resp: map[string]interface{}{"run": WorkflowNodeRun{"identifier": "node-run-1"}}},
		{name: "write workflow node run logs", call: func(ctx context.Context, c *Client) error {
			_, err := c.WriteWorkflowNodeRunLogs(ctx, "node-run-1", map[string]interface{}{"message": "done"})
			return err
		}, method: http.MethodPost, path: "/workflows/nodes/runs/node-run-1/logs", body: true, resp: map[string]interface{}{"logs": []map[string]interface{}{{"message": "done"}}}},
	})
}

func TestLLMMemoryAutoDiscoveryEndpointWrappers(t *testing.T) {
	provider := LLMProvider{"identifier": "openai"}
	record := MemoryRecord{"id": "m1"}
	invocation := AutoDiscoveryInvocation{"id": "inv-1"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list llm providers", call: func(ctx context.Context, c *Client) error { _, err := c.GetLLMProviders(ctx); return err }, method: http.MethodGet, path: "/llm-providers", resp: map[string]interface{}{"providers": []LLMProvider{provider}}},
		{name: "create llm provider", call: func(ctx context.Context, c *Client) error { _, err := c.CreateLLMProvider(ctx, provider); return err }, method: http.MethodPost, path: "/llm-providers", body: true, resp: map[string]interface{}{"provider": provider}},
		{name: "get llm defaults", call: func(ctx context.Context, c *Client) error { _, err := c.GetLLMProviderDefaults(ctx); return err }, method: http.MethodGet, path: "/llm-providers/defaults", resp: map[string]interface{}{"defaults": map[string]interface{}{"provider": "openai"}}},
		{name: "set llm defaults", call: func(ctx context.Context, c *Client) error {
			_, err := c.SetLLMProviderDefaults(ctx, map[string]interface{}{"provider": "openai"})
			return err
		}, method: http.MethodPut, path: "/llm-providers/defaults", body: true, resp: map[string]interface{}{"defaults": map[string]interface{}{"provider": "openai"}}},
		{name: "list memory", call: func(ctx context.Context, c *Client) error { _, err := c.GetMemoryRecords(ctx, nil); return err }, method: http.MethodGet, path: "/memory", resp: map[string]interface{}{"memory": []MemoryRecord{record}}},
		{name: "delete memory", call: func(ctx context.Context, c *Client) error {
			return c.DeleteMemoryRecords(ctx, map[string]interface{}{"ids": []string{"m1"}})
		}, method: http.MethodDelete, path: "/memory", body: true, resp: map[string]interface{}{"ok": true}},
		{name: "get memory settings", call: func(ctx context.Context, c *Client) error { _, err := c.GetMemorySettings(ctx); return err }, method: http.MethodGet, path: "/memory/settings", resp: map[string]interface{}{"settings": map[string]interface{}{"enabled": true}}},
		{name: "update memory settings", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateMemorySettings(ctx, map[string]interface{}{"enabled": true})
			return err
		}, method: http.MethodPut, path: "/memory/settings", body: true, resp: map[string]interface{}{"settings": map[string]interface{}{"enabled": true}}},
		{name: "create auto-discovery", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreateAutoDiscoveryInvocation(ctx, map[string]interface{}{"blueprint": "service"})
			return err
		}, method: http.MethodPost, path: "/ai/entities-auto-discovery", body: true, resp: map[string]interface{}{"invocation": invocation}},
		{name: "active auto-discovery", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetActiveAutoDiscoveryInvocations(ctx)
			return err
		}, method: http.MethodGet, path: "/ai/entities-auto-discovery/active", resp: map[string]interface{}{"invocations": []AutoDiscoveryInvocation{invocation}}},
		{name: "latest auto-discovery", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetLatestAutoDiscoveryInvocation(ctx, "service")
			return err
		}, method: http.MethodGet, path: "/ai/entities-auto-discovery/blueprint/service/latest", resp: map[string]interface{}{"invocation": invocation}},
		{name: "auto-discovery suggestions", call: func(ctx context.Context, c *Client) error {
			_, err := c.GetAutoDiscoverySuggestions(ctx, "inv-1")
			return err
		}, method: http.MethodGet, path: "/ai/entities-auto-discovery/inv-1/suggestions", resp: map[string]interface{}{"suggestions": []map[string]interface{}{{"id": "s1"}}}},
		{name: "review auto-discovery", call: func(ctx context.Context, c *Client) error {
			_, err := c.ReviewAutoDiscoverySuggestions(ctx, "inv-1", map[string]interface{}{"action": "approve"})
			return err
		}, method: http.MethodPost, path: "/ai/entities-auto-discovery/inv-1/review", body: true, resp: map[string]interface{}{"review": map[string]interface{}{"ok": true}}},
		{name: "update auto-discovery suggestion", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdateAutoDiscoverySuggestion(ctx, "inv-1", "svc-1", map[string]interface{}{"status": "accepted"})
			return err
		}, method: http.MethodPatch, path: "/ai/entities-auto-discovery/inv-1/suggestions/svc-1", body: true, resp: map[string]interface{}{"suggestion": map[string]interface{}{"id": "svc-1"}}},
	})
}

func TestMCPAppsPluginsEndpointWrappers(t *testing.T) {
	server := MCPServer{"id": "mcp-1"}
	app := AppCredentials{"id": "app-1"}
	plugin := Plugin{"identifier": "demo"}
	runEndpointWrapperCases(t, []endpointWrapperCase{
		{name: "list mcp servers", call: func(ctx context.Context, c *Client) error { _, err := c.GetMCPServers(ctx); return err }, method: http.MethodGet, path: "/mcp/user/servers", resp: map[string]interface{}{"servers": []MCPServer{server}}},
		{name: "get mcp server", call: func(ctx context.Context, c *Client) error { _, err := c.GetMCPServer(ctx, "mcp-1"); return err }, method: http.MethodGet, path: "/mcp/user/servers/mcp-1", resp: map[string]interface{}{"server": server}},
		{name: "disconnect mcp server", call: func(ctx context.Context, c *Client) error { return c.DisconnectMCPServer(ctx, "mcp-1") }, method: http.MethodDelete, path: "/mcp/user/servers/mcp-1", resp: map[string]interface{}{"ok": true}},
		{name: "list mcp templates", call: func(ctx context.Context, c *Client) error { _, err := c.GetMCPServerTemplates(ctx); return err }, method: http.MethodGet, path: "/mcp/templates", resp: map[string]interface{}{"templates": []map[string]interface{}{{"id": "t1"}}}},
		{name: "list port mcp tools", call: func(ctx context.Context, c *Client) error { _, err := c.GetPortMCPTools(ctx); return err }, method: http.MethodGet, path: "/mcp/port/tools", resp: map[string]interface{}{"tools": []map[string]interface{}{{"name": "search"}}}},
		{name: "list mcp server tools", call: func(ctx context.Context, c *Client) error { _, err := c.GetMCPServerTools(ctx, "mcp-1"); return err }, method: http.MethodGet, path: "/mcp/servers/mcp-1/tools", resp: map[string]interface{}{"tools": []map[string]interface{}{{"name": "query"}}}},
		{name: "call mcp tool", call: func(ctx context.Context, c *Client) error {
			_, err := c.CallMCPServerTool(ctx, "mcp-1", "query", map[string]interface{}{"q": "x"})
			return err
		}, method: http.MethodPost, path: "/mcp/servers/mcp-1/tools/query/call", body: true, resp: map[string]interface{}{"result": map[string]interface{}{"ok": true}}},
		{name: "mcp session token", call: func(ctx context.Context, c *Client) error { _, err := c.GetMCPOAuth2SessionToken(ctx, "mcp-1"); return err }, method: http.MethodGet, path: "/mcp/oauth2/servers/mcp-1/session-token", resp: map[string]interface{}{"token": "tok"}},
		{name: "list apps", call: func(ctx context.Context, c *Client) error { _, err := c.GetApps(ctx); return err }, method: http.MethodGet, path: "/apps", resp: map[string]interface{}{"apps": []AppCredentials{app}}},
		{name: "update app", call: func(ctx context.Context, c *Client) error { _, err := c.UpdateApp(ctx, "app-1", map[string]interface{}{"name": "ci"}); return err }, method: http.MethodPut, path: "/apps/app-1", body: true, resp: map[string]interface{}{"app": app}},
		{name: "delete app", call: func(ctx context.Context, c *Client) error { return c.DeleteApp(ctx, "app-1") }, method: http.MethodDelete, path: "/apps/app-1", resp: map[string]interface{}{"ok": true}},
		{name: "rotate app secret", call: func(ctx context.Context, c *Client) error { _, err := c.RotateAppSecret(ctx, "app-1"); return err }, method: http.MethodPost, path: "/apps/app-1/rotate-secret", resp: map[string]interface{}{"app": app}},
		{name: "rotate user credentials", call: func(ctx context.Context, c *Client) error { _, err := c.RotateUserCredentials(ctx, "dev@example.com"); return err }, method: http.MethodPost, path: "/rotate-credentials/dev@example.com", resp: map[string]interface{}{"credentials": map[string]interface{}{"ok": true}}},
		{name: "list plugins", call: func(ctx context.Context, c *Client) error { _, err := c.GetPlugins(ctx); return err }, method: http.MethodGet, path: "/plugins", resp: map[string]interface{}{"plugins": []Plugin{plugin}}},
		{name: "get plugin", call: func(ctx context.Context, c *Client) error { _, err := c.GetPlugin(ctx, "demo"); return err }, method: http.MethodGet, path: "/plugins/demo", resp: map[string]interface{}{"plugin": plugin}},
		{name: "update plugin", call: func(ctx context.Context, c *Client) error { _, err := c.UpdatePlugin(ctx, "demo", map[string]interface{}{"title": "Demo"}); return err }, method: http.MethodPatch, path: "/plugins/demo", body: true, resp: map[string]interface{}{"plugin": plugin}},
		{name: "delete plugin", call: func(ctx context.Context, c *Client) error { return c.DeletePlugin(ctx, "demo") }, method: http.MethodDelete, path: "/plugins/demo", resp: map[string]interface{}{"ok": true}},
		{name: "plugin upload url", call: func(ctx context.Context, c *Client) error {
			_, err := c.CreatePluginUploadURL(ctx, map[string]interface{}{"filename": "p.zip"})
			return err
		}, method: http.MethodPost, path: "/plugins/upload-url", body: true, resp: map[string]interface{}{"uploadUrl": "https://example.com"}},
		{name: "plugin update upload url", call: func(ctx context.Context, c *Client) error {
			_, err := c.UpdatePluginUploadURL(ctx, "demo", map[string]interface{}{"filename": "p.zip"})
			return err
		}, method: http.MethodPut, path: "/plugins/demo/upload-url", body: true, resp: map[string]interface{}{"uploadUrl": "https://example.com"}},
		{name: "finalize plugin upload", call: func(ctx context.Context, c *Client) error {
			_, err := c.FinalizePluginUpload(ctx, map[string]interface{}{"key": "k"})
			return err
		}, method: http.MethodPost, path: "/plugins/finalize-upload", body: true, resp: map[string]interface{}{"plugin": plugin}},
		{name: "install plugin", call: func(ctx context.Context, c *Client) error {
			_, err := c.InstallPlugin(ctx, map[string]interface{}{"identifier": "demo"})
			return err
		}, method: http.MethodPost, path: "/plugins/install", body: true, resp: map[string]interface{}{"plugin": plugin}},
	})
}

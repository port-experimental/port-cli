package commands

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func TestAPIFactoryTeamsCommandsPreserveContract(t *testing.T) {
	spec := teamsResourceSpec()
	if spec.Name != "teams" {
		t.Fatalf("expected resource name teams, got %q", spec.Name)
	}
	if len(spec.Operations) != 4 {
		t.Fatalf("expected 4 team operations, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	teamsCmd, _, err := rootCmd.Find([]string{"teams"})
	if err != nil || teamsCmd == nil {
		t.Fatal("teams command not found")
	}
	if teamsCmd.Short != "Team operations" {
		t.Fatalf("unexpected teams short help: %q", teamsCmd.Short)
	}

	wantOps := map[string]struct {
		use   string
		short string
	}{
		"list":   {use: "list", short: "List all teams"},
		"create": {use: "create", short: "Create a new team"},
		"update": {use: "update [team-name]", short: "Update an existing team"},
		"delete": {use: "delete [team-name]", short: "Delete a team"},
	}
	for name, want := range wantOps {
		subCmd, _, findErr := teamsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("teams %s command not found: %v", name, findErr)
		}
		if subCmd.Use != want.use {
			t.Errorf("teams %s use = %q, want %q", name, subCmd.Use, want.use)
		}
		if subCmd.Short != want.short {
			t.Errorf("teams %s short = %q, want %q", name, subCmd.Short, want.short)
		}
	}

	createCmd, _, _ := teamsCmd.Find([]string{"create"})
	if err := createCmd.ParseFlags([]string{"--data", "team.json"}); err != nil {
		t.Fatalf("parse create flags: %v", err)
	}
	dataFile, _ := createCmd.Flags().GetString("data")
	if dataFile != "team.json" {
		t.Errorf("expected data team.json, got %q", dataFile)
	}

	deleteCmd, _, _ := teamsCmd.Find([]string{"delete"})
	if err := deleteCmd.ParseFlags([]string{"--force"}); err != nil {
		t.Fatalf("parse delete flags: %v", err)
	}
	force, _ := deleteCmd.Flags().GetBool("force")
	if !force {
		t.Error("expected --force true on teams delete")
	}

	listCmd, _, _ := teamsCmd.Find([]string{"list"})
	if err := listCmd.ParseFlags([]string{"--format", "yaml"}); err != nil {
		t.Fatalf("parse list flags: %v", err)
	}
	format, _ := listCmd.Flags().GetString("format")
	if format != "yaml" {
		t.Errorf("expected yaml format, got %q", format)
	}
}

func TestAPIFactoryUsersCommandsPreserveContract(t *testing.T) {
	spec := usersResourceSpec()
	if len(spec.Operations) != 7 {
		t.Fatalf("expected 7 user operations, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	usersCmd, _, err := rootCmd.Find([]string{"users"})
	if err != nil || usersCmd == nil {
		t.Fatal("users command not found")
	}

	for _, name := range []string{"list", "get", "invite", "update", "delete", "change-account-role", "change-company-role"} {
		subCmd, _, findErr := usersCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("users %s command not found", name)
		}
	}

	getCmd, _, _ := usersCmd.Find([]string{"get"})
	if err := getCmd.Args(getCmd, []string{"user@example.com"}); err != nil {
		t.Fatalf("users get args: %v", err)
	}

	listCmd, _, _ := usersCmd.Find([]string{"list"})
	if err := listCmd.ParseFlags([]string{"--format", "yaml"}); err != nil {
		t.Fatalf("parse list flags: %v", err)
	}
	format, _ := listCmd.Flags().GetString("format")
	if format != "yaml" {
		t.Errorf("expected yaml, got %q", format)
	}

	deleteCmd, _, _ := usersCmd.Find([]string{"delete"})
	if err := deleteCmd.ParseFlags([]string{"--force"}); err != nil {
		t.Fatalf("parse delete flags: %v", err)
	}
	force, _ := deleteCmd.Flags().GetBool("force")
	if !force {
		t.Error("expected --force to be true")
	}
}

func TestRegisterAPIUsesFactoryForTeamsAndUsers(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	for _, resource := range []string{
		"teams", "users", "webhooks", "blueprints", "pages", "entities", "scorecards", "actions",
		"action-runs", "audit", "agents", "ai", "integrations", "migrations", "organization", "secrets",
		"workflows", "workflow-runs",
	} {
		resourceCmd, _, err := apiCmd.Find([]string{resource})
		if err != nil || resourceCmd == nil {
			t.Fatalf("api %s command not found", resource)
		}
	}
}

func TestAPIFactoryWebhooksCommandsPreserveContract(t *testing.T) {
	spec := webhooksResourceSpec()
	if len(spec.Operations) != 5 {
		t.Fatalf("expected 5 webhook operations, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	webhooksCmd, _, err := rootCmd.Find([]string{"webhooks"})
	if err != nil || webhooksCmd == nil {
		t.Fatal("webhooks command not found")
	}

	for _, name := range []string{"list", "get", "create", "update", "delete"} {
		subCmd, _, findErr := webhooksCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("webhooks %s command not found", name)
		}
	}

	createCmd, _, _ := webhooksCmd.Find([]string{"create"})
	if err := createCmd.ParseFlags([]string{"--data", "webhook.json"}); err != nil {
		t.Fatalf("parse create flags: %v", err)
	}
	dataFile, _ := createCmd.Flags().GetString("data")
	if dataFile != "webhook.json" {
		t.Errorf("expected webhook.json, got %q", dataFile)
	}
}

func TestAPIFactoryScorecardsBlueprintFlag(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(scorecardsResourceSpec()))

	scorecardsCmd, _, _ := rootCmd.Find([]string{"scorecards"})
	listCmd, _, _ := scorecardsCmd.Find([]string{"list"})
	if err := listCmd.ParseFlags([]string{"--blueprint", "service"}); err != nil {
		t.Fatalf("parse list flags: %v", err)
	}
	bp, _ := listCmd.Flags().GetString("blueprint")
	if bp != "service" {
		t.Errorf("expected service, got %q", bp)
	}
}

func TestAPIFactoryActionRunsCommandsPreserveContract(t *testing.T) {
	spec := actionRunsResourceSpec()
	if len(spec.Operations) != 5 {
		t.Fatalf("expected 5 action-runs operations, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	actionRunsCmd, _, err := rootCmd.Find([]string{"action-runs"})
	if err != nil || actionRunsCmd == nil {
		t.Fatal("action-runs command not found")
	}

	for _, name := range []string{"list", "get", "update", "approve", "execute"} {
		subCmd, _, findErr := actionRunsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("action-runs %s command not found", name)
		}
	}

	executeCmd, _, _ := actionRunsCmd.Find([]string{"execute"})
	if err := executeCmd.ParseFlags([]string{"--data", "run.json"}); err != nil {
		t.Fatalf("parse execute flags: %v", err)
	}
	dataFile, _ := executeCmd.Flags().GetString("data")
	if dataFile != "run.json" {
		t.Errorf("expected run.json, got %q", dataFile)
	}
}

func TestAPIFactoryAuditListPreservesContract(t *testing.T) {
	spec := auditResourceSpec()
	if len(spec.Operations) != 1 {
		t.Fatalf("expected 1 audit operation, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	auditCmd, _, err := rootCmd.Find([]string{"audit"})
	if err != nil || auditCmd == nil {
		t.Fatal("audit command not found")
	}
	listCmd, _, err := auditCmd.Find([]string{"list"})
	if err != nil || listCmd == nil {
		t.Fatal("audit list command not found")
	}
	if err := listCmd.ParseFlags([]string{"--format", "yaml"}); err != nil {
		t.Fatalf("parse list flags: %v", err)
	}
	format, _ := listCmd.Flags().GetString("format")
	if format != "yaml" {
		t.Errorf("expected yaml, got %q", format)
	}
}

func TestAPIFactoryPermissionsChildPreservesContract(t *testing.T) {
	spec := permissionsChildSpec("blueprints", "blueprint", (*api.Client).GetBlueprintPermissions, (*api.Client).UpdateBlueprintPermissions)
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	bpCmd, _, err := rootCmd.Find([]string{"blueprints"})
	if err != nil || bpCmd == nil {
		t.Fatal("permissions blueprints command not found")
	}
	for _, name := range []string{"get", "update"} {
		subCmd, _, findErr := bpCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("blueprints %s command not found", name)
		}
	}
}

func TestAPIFactoryAgentsInvokePreservesContract(t *testing.T) {
	spec := agentsResourceSpec()
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	agentsCmd, _, err := rootCmd.Find([]string{"agents"})
	if err != nil || agentsCmd == nil {
		t.Fatal("agents command not found")
	}
	invokeCmd, _, err := agentsCmd.Find([]string{"invoke"})
	if err != nil || invokeCmd == nil {
		t.Fatal("agents invoke command not found")
	}
	if err := invokeCmd.ParseFlags([]string{"--data", "agent.json"}); err != nil {
		t.Fatalf("parse invoke flags: %v", err)
	}
	dataFile, _ := invokeCmd.Flags().GetString("data")
	if dataFile != "agent.json" {
		t.Errorf("expected agent.json, got %q", dataFile)
	}
}

func TestAPIFactoryAICommandsPreserveContract(t *testing.T) {
	spec := aiResourceSpec()
	if len(spec.Operations) != 2 {
		t.Fatalf("expected 2 ai operations, got %d", len(spec.Operations))
	}

	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(spec))

	aiCmd, _, err := rootCmd.Find([]string{"ai"})
	if err != nil || aiCmd == nil {
		t.Fatal("ai command not found")
	}
	for _, name := range []string{"invoke", "get"} {
		subCmd, _, findErr := aiCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("ai %s command not found", name)
		}
	}

	invokeCmd, _, _ := aiCmd.Find([]string{"invoke"})
	if err := invokeCmd.ParseFlags([]string{"--data", "ai.json"}); err != nil {
		t.Fatalf("parse invoke flags: %v", err)
	}
	getCmd, _, _ := aiCmd.Find([]string{"get"})
	if err := getCmd.ParseFlags([]string{"--format", "yaml"}); err != nil {
		t.Fatalf("parse get flags: %v", err)
	}
	format, _ := getCmd.Flags().GetString("format")
	if format != "yaml" {
		t.Errorf("expected yaml, got %q", format)
	}
}

func TestAPIFactoryIntegrationsAndMigrationsCommands(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(integrationsResourceSpec()))
	rootCmd.AddCommand(registerAPIResource(migrationsResourceSpec()))

	integrationsCmd, _, err := rootCmd.Find([]string{"integrations"})
	if err != nil || integrationsCmd == nil {
		t.Fatal("integrations command not found")
	}
	for _, name := range []string{"list", "get", "update", "update-config", "delete"} {
		subCmd, _, findErr := integrationsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("integrations %s command not found", name)
		}
	}

	migrationsCmd, _, err := rootCmd.Find([]string{"migrations"})
	if err != nil || migrationsCmd == nil {
		t.Fatal("migrations command not found")
	}
	for _, name := range []string{"list", "get", "create", "cancel"} {
		subCmd, _, findErr := migrationsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("migrations %s command not found", name)
		}
	}

	createCmd, _, _ := migrationsCmd.Find([]string{"create"})
	if err := createCmd.ParseFlags([]string{"--data", "migration.json"}); err != nil {
		t.Fatalf("parse create flags: %v", err)
	}
}

func TestMigrationRequestFromData(t *testing.T) {
	req, err := migrationRequestFromData(map[string]interface{}{
		"sourceBlueprint": "service",
		"mapping":         map[string]interface{}{"blueprint": "service"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.SourceBlueprint != "service" {
		t.Fatalf("sourceBlueprint = %q", req.SourceBlueprint)
	}

	if _, err := migrationRequestFromData(map[string]interface{}{"mapping": map[string]interface{}{}}); err == nil {
		t.Fatal("expected error for missing sourceBlueprint")
	}
	if _, err := migrationRequestFromData(map[string]interface{}{"sourceBlueprint": "service"}); err == nil {
		t.Fatal("expected error for missing mapping")
	}
}

func TestAPIFactoryOrganizationAndSecretsCommands(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(organizationResourceSpec()))
	rootCmd.AddCommand(registerAPIResource(secretsResourceSpec()))

	orgCmd, _, err := rootCmd.Find([]string{"organization"})
	if err != nil || orgCmd == nil {
		t.Fatal("organization command not found")
	}
	for _, name := range []string{"get", "update", "replace"} {
		subCmd, _, findErr := orgCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("organization %s command not found", name)
		}
	}

	secretsCmd, _, err := rootCmd.Find([]string{"secrets"})
	if err != nil || secretsCmd == nil {
		t.Fatal("secrets command not found")
	}
	for _, name := range []string{"list", "get", "create", "update", "delete"} {
		subCmd, _, findErr := secretsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("secrets %s command not found", name)
		}
	}

	deleteCmd, _, _ := secretsCmd.Find([]string{"delete"})
	if err := deleteCmd.ParseFlags([]string{"--force"}); err != nil {
		t.Fatalf("parse delete flags: %v", err)
	}
}

func TestAPIFactoryWorkflowCommands(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	rootCmd.AddCommand(registerAPIResource(workflowsResourceSpec()))
	rootCmd.AddCommand(registerAPIResource(workflowRunsResourceSpec()))

	workflowsCmd, _, err := rootCmd.Find([]string{"workflows"})
	if err != nil || workflowsCmd == nil {
		t.Fatal("workflows command not found")
	}
	for _, name := range []string{"list", "get", "create", "update", "delete", "get-node", "list-triggers"} {
		subCmd, _, findErr := workflowsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("workflows %s command not found", name)
		}
	}

	runsCmd, _, err := rootCmd.Find([]string{"workflow-runs"})
	if err != nil || runsCmd == nil {
		t.Fatal("workflow-runs command not found")
	}
	for _, name := range []string{"list", "get", "trigger", "cancel", "logs", "node-logs", "update-node-run", "write-node-logs"} {
		subCmd, _, findErr := runsCmd.Find([]string{name})
		if findErr != nil || subCmd == nil {
			t.Fatalf("workflow-runs %s command not found", name)
		}
	}

	triggerCmd, _, _ := runsCmd.Find([]string{"trigger"})
	if err := triggerCmd.ParseFlags([]string{"--data", "run.json"}); err != nil {
		t.Fatalf("parse trigger flags: %v", err)
	}

	getNodeCmd, _, _ := workflowsCmd.Find([]string{"get-node"})
	if err := getNodeCmd.Args(getNodeCmd, []string{"deploy", "build"}); err != nil {
		t.Fatalf("workflows get-node args: %v", err)
	}
}

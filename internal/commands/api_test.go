package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAPICallFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	if apiCmd == nil {
		t.Fatal("api command not found")
	}

	callCmd, _, _ := apiCmd.Find([]string{"call"})
	if callCmd == nil {
		t.Fatal("call command not found")
	}

	// Parse args without executing RunE
	callCmd.DisableFlagParsing = false
	err := callCmd.ParseFlags([]string{
		"--org", "local",
		"--method", "POST",
		"--data", "{}",
		"--format", "yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	org, err := callCmd.Flags().GetString("org")
	if err != nil {
		t.Fatalf("could not get --org %v", err)
	}
	if org != "local" {
		t.Errorf("expected 'local', got %q", org)
	}

	method, err := callCmd.Flags().GetString("method")
	if err != nil {
		t.Fatalf("could not get --method %v", err)
	}
	if method != "POST" {
		t.Errorf("expected 'POST', got %q", method)
	}

	data, err := callCmd.Flags().GetString("data")
	if err != nil {
		t.Fatalf("could not get --data %v", err)
	}
	if data != "{}" {
		t.Errorf("expected '{}', got %q", data)
	}

	format, err := callCmd.Flags().GetString("format")
	if err != nil {
		t.Fatalf("could not get --format %v", err)
	}
	if format != "yaml" {
		t.Errorf("expected 'yaml', got %q", format)
	}
}

func TestPageSubcommandsFlagsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	pagesCmd, _, _ := apiCmd.Find([]string{"pages"})
	if pagesCmd == nil {
		t.Fatal("pages command not found")
	}

	listCmd, _, _ := pagesCmd.Find([]string{"list"})
	if listCmd == nil {
		t.Fatal("pages list command not found")
	}

	createCmd, _, _ := pagesCmd.Find([]string{"create"})
	if createCmd == nil {
		t.Fatal("pages create command not found")
	}
	err := createCmd.ParseFlags([]string{"--data", "page.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dataFile, _ := createCmd.Flags().GetString("data")
	if dataFile != "page.json" {
		t.Errorf("expected 'page.json', got %q", dataFile)
	}

	updateCmd, _, _ := pagesCmd.Find([]string{"update"})
	if updateCmd == nil {
		t.Fatal("pages update command not found")
	}
}

func TestTeamSubcommandsFlagsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	teamsCmd, _, _ := apiCmd.Find([]string{"teams"})
	if teamsCmd == nil {
		t.Fatal("teams command not found")
	}

	for _, sub := range []string{"list", "create", "update", "delete"} {
		subCmd, _, _ := teamsCmd.Find([]string{sub})
		if subCmd == nil {
			t.Fatalf("teams %s command not found", sub)
		}
	}

	createCmd, _, _ := teamsCmd.Find([]string{"create"})
	err := createCmd.ParseFlags([]string{"--data", "team.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dataFile, _ := createCmd.Flags().GetString("data")
	if dataFile != "team.json" {
		t.Errorf("expected 'team.json', got %q", dataFile)
	}

	deleteCmd, _, _ := teamsCmd.Find([]string{"delete"})
	err = deleteCmd.ParseFlags([]string{"--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	force, _ := deleteCmd.Flags().GetBool("force")
	if !force {
		t.Error("expected --force to be true")
	}
}

func TestUserSubcommandsFlagsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	usersCmd, _, _ := apiCmd.Find([]string{"users"})
	if usersCmd == nil {
		t.Fatal("users command not found")
	}

	listCmd, _, _ := usersCmd.Find([]string{"list"})
	if listCmd == nil {
		t.Fatal("users list command not found")
	}

	getCmd, _, _ := usersCmd.Find([]string{"get"})
	if getCmd == nil {
		t.Fatal("users get command not found")
	}

	err := listCmd.ParseFlags([]string{"--format", "yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	format, _ := listCmd.Flags().GetString("format")
	if format != "yaml" {
		t.Errorf("expected 'yaml', got %q", format)
	}
}

func TestScorecardSubcommandsFlagsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	scorecardsCmd, _, _ := apiCmd.Find([]string{"scorecards"})
	if scorecardsCmd == nil {
		t.Fatal("scorecards command not found")
	}

	for _, sub := range []string{"list", "create", "update", "delete"} {
		subCmd, _, _ := scorecardsCmd.Find([]string{sub})
		if subCmd == nil {
			t.Fatalf("scorecards %s command not found", sub)
		}
	}

	listCmd, _, _ := scorecardsCmd.Find([]string{"list"})
	err := listCmd.ParseFlags([]string{"--blueprint", "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bp, _ := listCmd.Flags().GetString("blueprint")
	if bp != "service" {
		t.Errorf("expected 'service', got %q", bp)
	}
}

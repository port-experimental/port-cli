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

func TestAPIEntitiesBulkDeleteFlagsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAPI(rootCmd)

	apiCmd, _, _ := rootCmd.Find([]string{"api"})
	if apiCmd == nil {
		t.Fatal("api command not found")
	}

	entitiesCmd, _, _ := apiCmd.Find([]string{"entities"})
	if entitiesCmd == nil {
		t.Fatal("entities command not found")
	}

	bulkCmd, _, _ := entitiesCmd.Find([]string{"bulk-delete"})
	if bulkCmd == nil {
		t.Fatal("bulk-delete command not found")
	}

	bulkCmd.DisableFlagParsing = false
	err := bulkCmd.ParseFlags([]string{
		"--org", "test-org",
		"--jq", ".identifier",
		"--delete-dependents",
		"--force",
		"--batch-size", "50",
	})
	if err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	org, err := bulkCmd.Flags().GetString("org")
	if err != nil {
		t.Fatalf("could not get --org %v", err)
	}
	if org != "test-org" {
		t.Errorf("expected 'test-org', got %q", org)
	}

	jq, err := bulkCmd.Flags().GetString("jq")
	if err != nil {
		t.Fatalf("could not get --jq %v", err)
	}
	if jq != ".identifier" {
		t.Errorf("expected '.identifier', got %q", jq)
	}

	deleteDeps, err := bulkCmd.Flags().GetBool("delete-dependents")
	if err != nil {
		t.Fatalf("could not get --delete-dependents %v", err)
	}
	if !deleteDeps {
		t.Errorf("expected delete-dependents to be true")
	}

	force, err := bulkCmd.Flags().GetBool("force")
	if err != nil {
		t.Fatalf("could not get --force %v", err)
	}
	if !force {
		t.Errorf("expected force to be true")
	}

	batchSize, err := bulkCmd.Flags().GetInt("batch-size")
	if err != nil {
		t.Fatalf("could not get --batch-size %v", err)
	}
	if batchSize != 50 {
		t.Errorf("expected 50, got %d", batchSize)
	}
}

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

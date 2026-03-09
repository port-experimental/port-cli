package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMigrateExcludeFlags(t *testing.T) {
	// Build a root command and register migrate on it
	rootCmd := &cobra.Command{Use: "port"}
	RegisterMigrate(rootCmd)

	// Verify that the flags exist on the migrate subcommand
	migrateCmd, _, err := rootCmd.Find([]string{"migrate"})
	if err != nil || migrateCmd == nil {
		t.Fatal("migrate command not found")
	}

	tests := []struct {
		name string
		flag string
	}{
		{"exclude-blueprints flag exists", "exclude-blueprints"},
		{"exclude-blueprint-schema flag exists", "exclude-blueprint-schema"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := migrateCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Errorf("flag --%s not registered on migrate command", tt.flag)
			}
		})
	}
}

func TestMigrateExcludeFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterMigrate(rootCmd)

	migrateCmd, _, _ := rootCmd.Find([]string{"migrate"})
	if migrateCmd == nil {
		t.Fatal("migrate command not found")
	}

	// Parse args without executing RunE
	migrateCmd.DisableFlagParsing = false
	err := migrateCmd.ParseFlags([]string{
		"--exclude-blueprints", "service,microservice",
		"--exclude-blueprint-schema", "region",
		"--target-org", "my-target",
	})
	if err != nil {
		t.Errorf("unexpected error parsing flags: %v", err)
	}

	eb, err := migrateCmd.Flags().GetString("exclude-blueprints")
	if err != nil {
		t.Fatalf("could not get --exclude-blueprints: %v", err)
	}
	if eb != "service,microservice" {
		t.Errorf("expected 'service,microservice', got %q", eb)
	}

	ebs, err := migrateCmd.Flags().GetString("exclude-blueprint-schema")
	if err != nil {
		t.Fatalf("could not get --exclude-blueprint-schema: %v", err)
	}
	if ebs != "region" {
		t.Errorf("expected 'region', got %q", ebs)
	}
}

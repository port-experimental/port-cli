package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestImportExcludeFlags(t *testing.T) {
	// Build a root command and register import on it
	rootCmd := &cobra.Command{Use: "port"}
	RegisterImport(rootCmd)

	// Verify that the flags exist on the import subcommand
	importCmd, _, err := rootCmd.Find([]string{"import"})
	if err != nil || importCmd == nil {
		t.Fatal("import command not found")
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
			f := importCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Errorf("flag --%s not registered on import command", tt.flag)
			}
		})
	}
}

func TestImportExcludeFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterImport(rootCmd)

	importCmd, _, _ := rootCmd.Find([]string{"import"})
	if importCmd == nil {
		t.Fatal("import command not found")
	}

	// Parse args without executing RunE
	importCmd.DisableFlagParsing = false
	err := importCmd.ParseFlags([]string{
		"--exclude-blueprints", "service,microservice",
		"--exclude-blueprint-schema", "region",
		"--input", "dummy.tar.gz",
	})
	if err != nil {
		t.Errorf("unexpected error parsing flags: %v", err)
	}

	eb, err := importCmd.Flags().GetString("exclude-blueprints")
	if err != nil {
		t.Fatalf("could not get --exclude-blueprints: %v", err)
	}
	if eb != "service,microservice" {
		t.Errorf("expected 'service,microservice', got %q", eb)
	}

	ebs, err := importCmd.Flags().GetString("exclude-blueprint-schema")
	if err != nil {
		t.Fatalf("could not get --exclude-blueprint-schema: %v", err)
	}
	if ebs != "region" {
		t.Errorf("expected 'region', got %q", ebs)
	}
}

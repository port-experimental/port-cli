package commands

import (
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/modules/migrate"
	"github.com/spf13/cobra"
)

func TestMigrateExcludeFlags(t *testing.T) {
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
		{"integrations flag exists", "integrations"},
		{"entities flag exists", "entities"},
		{"actions flag exists", "actions"},
		{"scorecards flag exists", "scorecards"},
		{"pages flag exists", "pages"},
		{"teams flag exists", "teams"},
		{"users flag exists", "users"},
		{"skip-system-blueprint-properties flag exists", "skip-system-blueprint-properties"},
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

	skipSystemBlueprintProperties, err := migrateCmd.Flags().GetBool("skip-system-blueprint-properties")
	if err != nil {
		t.Fatalf("could not get --skip-system-blueprint-properties: %v", err)
	}
	if skipSystemBlueprintProperties {
		t.Error("expected --skip-system-blueprint-properties default to be false")
	}
}

func TestMigrateIntegrationsFlagParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterMigrate(rootCmd)

	migrateCmd, _, _ := rootCmd.Find([]string{"migrate"})
	if migrateCmd == nil {
		t.Fatal("migrate command not found")
	}

	err := migrateCmd.ParseFlags([]string{
		"--integrations", "int1,int2",
		"--target-org", "my-target",
	})
	if err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	integrations, err := migrateCmd.Flags().GetString("integrations")
	if err != nil {
		t.Fatalf("could not get --integrations: %v", err)
	}
	if integrations != "int1,int2" {
		t.Errorf("expected 'int1,int2', got %q", integrations)
	}
	if !migrateCmd.Flags().Changed("integrations") {
		t.Error("expected integrations flag to be marked as changed")
	}
}

func TestMigrationFailureMessageIncludesResultErrors(t *testing.T) {
	msg := migrationFailureMessage(&migrate.Result{
		Message: "Migration completed with 2 error(s)",
		Errors: []string{
			"Entities service: failed to get current entities",
			"Blueprint component: relation target missing",
		},
	})

	for _, want := range []string{
		"Migration completed with 2 error(s)",
		"Entities service: failed to get current entities",
		"Blueprint component: relation target missing",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected failure message to contain %q, got:\n%s", want, msg)
		}
	}
}

func TestMigrationFailureMessageWithoutErrorsIsGeneric(t *testing.T) {
	msg := migrationFailureMessage(&migrate.Result{
		Message: "Migration completed with 0 error(s)",
	})
	if msg != "migration failed" {
		t.Fatalf("expected generic failure without result errors, got %q", msg)
	}
}

func TestMigrationFailureMessageLimitsErrors(t *testing.T) {
	msg := migrationFailureMessage(&migrate.Result{
		Message: "Migration completed with 6 error(s)",
		Errors: []string{
			"err-1",
			"err-2",
			"err-3",
			"err-4",
			"err-5",
			"err-6",
		},
	})

	if strings.Contains(msg, "err-6") {
		t.Fatalf("expected message to omit sixth error, got:\n%s", msg)
	}
	if !strings.Contains(msg, "... and 1 more") {
		t.Fatalf("expected truncation message, got:\n%s", msg)
	}
}

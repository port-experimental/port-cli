package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestExportSystemBlueprintPropertiesFlag(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterExport(rootCmd)

	exportCmd, _, err := rootCmd.Find([]string{"export"})
	if err != nil || exportCmd == nil {
		t.Fatal("export command not found")
	}

	flag := exportCmd.Flags().Lookup("skip-system-blueprint-properties")
	if flag == nil {
		t.Fatal("flag --skip-system-blueprint-properties not registered on export command")
	}

	value, err := exportCmd.Flags().GetBool("skip-system-blueprint-properties")
	if err != nil {
		t.Fatalf("could not get --skip-system-blueprint-properties: %v", err)
	}
	if value {
		t.Fatal("expected --skip-system-blueprint-properties default to be false")
	}
}

func TestExportMaxErrorsFlag(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterExport(rootCmd)

	exportCmd, _, err := rootCmd.Find([]string{"export"})
	if err != nil || exportCmd == nil {
		t.Fatal("export command not found")
	}

	flag := exportCmd.Flags().Lookup("max-errors")
	if flag == nil {
		t.Fatal("flag --max-errors not registered on export command")
	}

	value, err := exportCmd.Flags().GetInt("max-errors")
	if err != nil {
		t.Fatalf("could not get --max-errors: %v", err)
	}
	if value != defaultMaxErrors {
		t.Fatalf("expected --max-errors default to be %d, got %d", defaultMaxErrors, value)
	}
}

func TestExportMaxErrorsFlagParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterExport(rootCmd)

	exportCmd, _, _ := rootCmd.Find([]string{"export"})
	if exportCmd == nil {
		t.Fatal("export command not found")
	}

	err := exportCmd.ParseFlags([]string{
		"--max-errors", "-1",
		"--output", "dummy.json",
	})
	if err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	value, err := exportCmd.Flags().GetInt("max-errors")
	if err != nil {
		t.Fatalf("could not get --max-errors: %v", err)
	}
	if value != hideAllErrors {
		t.Fatalf("expected --max-errors to parse as -1, got %d", value)
	}
}

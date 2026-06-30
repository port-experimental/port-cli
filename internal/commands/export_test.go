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

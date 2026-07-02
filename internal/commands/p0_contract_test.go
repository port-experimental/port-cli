package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestValidateStringEnumRejectsInvalidCompareOutput(t *testing.T) {
	err := validateStringEnum("--output", "xml", []string{"text", "json", "html"})
	if err == nil {
		t.Fatal("expected invalid enum error")
	}
	if !strings.Contains(err.Error(), "Valid values: text, json, html") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVersionQuietPrintsOnlyVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	output.SetWriters(&out, &errOut)
	output.SetVerbosity(output.QuietLevel)
	defer output.SetWriters(os.Stdout, os.Stderr)
	defer output.SetVerbosity(output.NormalLevel)

	root := &cobra.Command{Use: "port"}
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(WithGlobalFlags(cmd.Context(), GlobalFlags{Quiet: true}))
	}
	RegisterVersion(root)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	got := out.String()
	if got != buildInfo.Version+"\n" {
		t.Fatalf("expected only version %q, got %q", buildInfo.Version+"\n", got)
	}
}

func TestDefaultConfigPathUsesPortConfigFileEnv(t *testing.T) {
	t.Setenv("PORT_CONFIG_FILE", "/tmp/custom-port-config.yaml")
	if got := config.DefaultConfigPath(); got != "/tmp/custom-port-config.yaml" {
		t.Fatalf("expected PORT_CONFIG_FILE path, got %q", got)
	}
}

func TestConfigWritesUsePrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows file mode semantics differ")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	manager := config.NewConfigManager(path)
	if err := manager.Write(&config.Config{Organizations: map[string]config.OrganizationConfig{}}); err != nil {
		t.Fatalf("write config: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", got)
	}
}

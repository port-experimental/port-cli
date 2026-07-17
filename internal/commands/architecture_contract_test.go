package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func architectureContractRoot() *cobra.Command {
	root := &cobra.Command{Use: "port"}
	root.PersistentFlags().String("config", "", "Path to configuration file")
	root.PersistentFlags().String("client-id", "", "Base org Port API client ID (overrides config/env)")
	root.PersistentFlags().String("client-secret", "", "Base org Port API client secret (overrides config/env)")
	root.PersistentFlags().String("api-url", "", "Base org Port API URL (overrides config/env)")
	root.PersistentFlags().String("target-client-id", "", "Target org Port API client ID (overrides config/env)")
	root.PersistentFlags().String("target-client-secret", "", "Target org Port API client secret (overrides config/env)")
	root.PersistentFlags().String("target-api-url", "", "Target org Port API URL (overrides config/env)")
	root.PersistentFlags().Bool("no-color", false, "Disable color output")
	root.PersistentFlags().Bool("json-errors", false, "Print command errors as structured JSON")
	root.PersistentFlags().Bool("no-env-file", false, "Do not load .env files from the current directory or ~/.port")
	root.PersistentFlags().Bool("quiet", false, "Suppress non-error output")
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	root.PersistentFlags().Bool("yes", false, "Skip confirmation prompts")
	root.PersistentFlags().Bool(TreeFlagName, false, "Print the full command tree for this command and exit")

	RegisterAuth(root)
	RegisterExport(root)
	RegisterImport(root)
	RegisterClear(root)
	RegisterMigrate(root)
	RegisterCompare(root)
	RegisterAPI(root)
	RegisterVersion(root)
	RegisterConfig(root)
	RegisterCompletion(root)
	RegisterDocs(root)
	RegisterSkills(root)
	RegisterCache(root)
	return root
}

func TestCommandTreeContractContainsCoreCommands(t *testing.T) {
	root := architectureContractRoot()
	var buf bytes.Buffer
	PrintCommandTree(&buf, root)
	got := buf.String()

	for _, want := range []string{
		"api — Direct Port API operations",
		"auth — Authenticate the CLI with Port",
		"cache — Manage locally cached Port data",
		"compare — Compare two Port organizations",
		"config — Manage Port CLI configuration",
		"docs — Generate CLI reference documentation",
		"export — Export data from Port",
		"import — Import data to Port",
		"migrate — Migrate data between Port organizations",
		"skills — Manage Port AI skills",
		"├── call — Generic API operations",
		"webhooks — Webhook operations",
		"llm-providers — LLM provider operations",
		"memory — Memory record and settings operations",
		"auto-discovery — Catalog auto-discovery operations",
		"mcp — MCP connector operations",
		"apps — App credentials operations",
		"plugins — Plugin operations",
		"└── workflows — Workflow operations",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("command tree missing %q\nTree:\n%s", want, got)
		}
	}
}

func TestRepresentativeHelpContracts(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"api"}, want: []string{"Direct Port API operations", "blueprints", "webhooks", "workflows", "llm-providers", "memory", "auto-discovery", "mcp", "apps", "plugins", "call"}},
		{args: []string{"api", "call"}, want: []string{"raw Port API response envelope", "--unwrap"}},
		{args: []string{"export"}, want: []string{"--output", "--output-format", "--skip-entities"}},
		{args: []string{"import"}, want: []string{"--input", "--dry-run", "--output-format"}},
		{args: []string{"migrate"}, want: []string{"--source-org", "--target-org", "--dry-run"}},
		{args: []string{"compare"}, want: []string{"--source", "--target", "--output"}},
		{args: []string{"skills"}, want: []string{"Manage Port AI skills", "init", "sync", "upload"}},
		{args: []string{"config"}, want: []string{"sources", "--show", "--init"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			root := architectureContractRoot()
			cmd, _, err := root.Find(tt.args)
			if err != nil {
				t.Fatalf("find command: %v", err)
			}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			if err := cmd.Help(); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			got := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("help for %v missing %q\nHelp:\n%s", tt.args, want, got)
				}
			}
		})
	}
}

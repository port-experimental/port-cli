/**
 * @spec-handoff
 * @interface registerAgentCreate() *cobra.Command  (registered via RegisterAgents → agentsCmd.AddCommand)
 * @behavior
 *   - "create" subcommand is registered under "agents"
 *   - --file/-f (string, required): path to the agent .md file
 *   - --force (bool, default false): create if new; replace if exists
 *   - --patch (bool, default false): partial update; fails if agent not found
 *   - --output/-o (string, default "table"): one of table/json/yaml
 *   - --yes/-y (bool, default false): skip confirmation prompt
 *   - --org (string): organization name (optional)
 *   - Missing --file causes a cobra flag-required error before RunE executes
 *   - --force and --patch are mutually exclusive (validated in RunE)
 *   - --mode flag MUST NOT be registered (removed in redesign)
 * @edge-cases
 *   - "agents create" without --file → cobra returns flag-required error mentioning "file"
 *   - --force and --patch set simultaneously → RunE returns error containing "mutually exclusive"
 *   - --force flag is boolean (Type() == "bool"), default false
 *   - --patch flag is boolean (Type() == "bool"), default false
 *   - --yes/-y is a boolean flag (Type() == "bool")
 *   - --output/-o defaults to "table"
 *   - Lookup("mode") returns nil — flag does not exist
 * @see ./agents.go (RegisterAgents, registerAgentCreate)
 */

package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// findCreateCmd returns the "create" subcommand under "agents".
func findCreateCmd(t *testing.T, rootCmd *cobra.Command) *cobra.Command {
	t.Helper()
	agentsCmd := findAgentsCmd(t, rootCmd)
	createCmd := findSubcmd(t, agentsCmd, "create")
	return createCmd
}

// ---------------------------------------------------------------------------
// A1: "create" is registered under "agents"
// ---------------------------------------------------------------------------

func TestAgentCreate_SubcommandPresence(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)

	found := false
	for _, cmd := range agentsCmd.Commands() {
		if cmd.Name() == "create" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected subcommand \"create\" under \"agents\", not found")
	}
}

// ---------------------------------------------------------------------------
// A2: --file is required — error when missing
// ---------------------------------------------------------------------------

func TestAgentCreate_FileRequired_ErrorWhenMissing(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs([]string{"agents", "create"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --file is missing, got nil")
	}
	if !strings.Contains(err.Error(), "file") {
		t.Errorf("expected error to mention 'file', got: %q", err.Error())
	}
}

func TestAgentCreate_FileFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("file")
	if f == nil {
		t.Fatal("expected flag --file on \"create\" command")
	}
	if f.Value.Type() != "string" {
		t.Errorf("expected --file to be string, got %q", f.Value.Type())
	}
	if f.Shorthand != "f" {
		t.Errorf("expected --file shorthand to be 'f', got %q", f.Shorthand)
	}
}

func TestAgentCreate_FileFlagRequired(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("file")
	if f == nil {
		t.Fatal("expected flag --file on \"create\" command")
	}
	annotations := f.Annotations
	requiredAnnotation, ok := annotations[cobra.BashCompOneRequiredFlag]
	if !ok {
		t.Fatal("expected --file to be marked as required via cobra annotation")
	}
	if len(requiredAnnotation) == 0 || requiredAnnotation[0] != "true" {
		t.Errorf("expected BashCompOneRequiredFlag to be 'true', got %v", requiredAnnotation)
	}
}

// ---------------------------------------------------------------------------
// A3: --force flag is registered and boolean
// ---------------------------------------------------------------------------

func TestAgentCreate_ForceFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("expected flag --force on \"create\" command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --force to be bool, got %q", f.Value.Type())
	}
	if f.DefValue != "false" {
		t.Errorf("expected --force default to be \"false\", got %q", f.DefValue)
	}
}

// ---------------------------------------------------------------------------
// A4: --patch flag is registered and boolean
// ---------------------------------------------------------------------------

func TestAgentCreate_PatchFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("patch")
	if f == nil {
		t.Fatal("expected flag --patch on \"create\" command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --patch to be bool, got %q", f.Value.Type())
	}
	if f.DefValue != "false" {
		t.Errorf("expected --patch default to be \"false\", got %q", f.DefValue)
	}
}

// ---------------------------------------------------------------------------
// A5: --yes/-y flag is registered and boolean
// ---------------------------------------------------------------------------

func TestAgentCreate_YesFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("expected flag --yes on \"create\" command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --yes to be bool, got %q", f.Value.Type())
	}
	if f.DefValue != "false" {
		t.Errorf("expected --yes default to be \"false\", got %q", f.DefValue)
	}
	if f.Shorthand != "y" {
		t.Errorf("expected --yes shorthand to be 'y', got %q", f.Shorthand)
	}
}

// ---------------------------------------------------------------------------
// A6: --output/-o accepts table/json/yaml, defaults to "table"
// ---------------------------------------------------------------------------

func TestAgentCreate_OutputFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("expected flag --output on \"create\" command")
	}
	if f.Value.Type() != "string" {
		t.Errorf("expected --output to be string, got %q", f.Value.Type())
	}
	if f.DefValue != "table" {
		t.Errorf("expected --output default to be \"table\", got %q", f.DefValue)
	}
	if f.Shorthand != "o" {
		t.Errorf("expected --output shorthand to be 'o', got %q", f.Shorthand)
	}
}

func TestAgentCreate_OutputFlag_AcceptsValidValues(t *testing.T) {
	validOutputs := []string{"table", "json", "yaml"}

	for _, output := range validOutputs {
		t.Run("output="+output, func(t *testing.T) {
			rootCmd := buildAgentsRoot(t)
			createCmd := findCreateCmd(t, rootCmd)

			if err := createCmd.Flags().Set("output", output); err != nil {
				t.Errorf("expected output %q to be accepted by flag, got error: %v", output, err)
			}
			got := createCmd.Flags().Lookup("output").Value.String()
			if got != output {
				t.Errorf("after Set(output, %q), got %q", output, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// A7: --mode flag MUST NOT be registered (regression guard)
// ---------------------------------------------------------------------------

func TestAgentCreate_ModeFlag_MustNotExist(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("mode")
	if f != nil {
		t.Errorf("--mode flag must NOT be registered on \"create\" command (it was removed); found: %+v", f)
	}
}

// ---------------------------------------------------------------------------
// A8: --force and --patch can be set independently; error when both set (RunE validation)
// ---------------------------------------------------------------------------

func TestAgentCreate_ForceOnly_FlagSet(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	if err := createCmd.Flags().Set("force", "true"); err != nil {
		t.Errorf("expected --force to be settable independently, got error: %v", err)
	}
	got := createCmd.Flags().Lookup("force").Value.String()
	if got != "true" {
		t.Errorf("after Set(force, true), got %q", got)
	}
}

func TestAgentCreate_PatchOnly_FlagSet(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	if err := createCmd.Flags().Set("patch", "true"); err != nil {
		t.Errorf("expected --patch to be settable independently, got error: %v", err)
	}
	got := createCmd.Flags().Lookup("patch").Value.String()
	if got != "true" {
		t.Errorf("after Set(patch, true), got %q", got)
	}
}

func TestAgentCreate_ForcePatch_BothSet_RunEReturnsError(t *testing.T) {
	// Create a temp .md file with minimal valid frontmatter so the --file flag
	// is satisfied and the guard can reach the mutual-exclusivity check.
	tmp, err := os.CreateTemp(t.TempDir(), "agent-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	content := "---\nidentifier: test_agent\n---\n\nThis is the agent body.\n"
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmp.Close()

	rootCmd := buildAgentsRoot(t)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs([]string{
		"agents", "create",
		"--file", tmp.Name(),
		"--force",
		"--patch",
		"--yes",
	})

	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --force and --patch are set, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected error to contain \"mutually exclusive\", got: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// A9: All sibling subcommands still present (regression guard)
// ---------------------------------------------------------------------------

func TestAgentCreate_AllSiblingSubcommandsStillPresent(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)

	expected := []string{"invoke", "list", "get", "update", "create"}
	for _, name := range expected {
		found := false
		for _, cmd := range agentsCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q under \"agents\", not found", name)
		}
	}
}

// ---------------------------------------------------------------------------
// NoArgs enforcement — unchanged from prior design
// ---------------------------------------------------------------------------

func TestAgentCreate_NoArgs_AcceptsNoPositionalArgs(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	// cobra.NoArgs should accept 0 args.
	if err := createCmd.Args(createCmd, []string{}); err != nil {
		t.Errorf("expected NoArgs to accept 0 args, got error: %v", err)
	}
	// And reject 1+ positional args.
	if err := createCmd.Args(createCmd, []string{"unexpected-arg"}); err == nil {
		t.Error("expected NoArgs to reject positional args, got nil error")
	}
}

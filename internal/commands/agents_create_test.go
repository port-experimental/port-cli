/**
 * @spec-handoff
 * @interface registerAgentCreate() *cobra.Command  (registered via RegisterAgents → agentsCmd.AddCommand)
 * @behavior
 *   - "create" subcommand is registered under "agents"
 *   - --file/-f (string, required): path to the agent .md file
 *   - --mode (string, default "auto"): one of auto/create/upsert/patch; default "auto"
 *   - --output/-o (string, default "table"): one of table/json/yaml
 *   - --yes/-y (bool, default false): skip confirmation prompt
 *   - --org (string): organization name (optional)
 *   - Missing --file causes a cobra flag-required error before RunE executes
 *   - Unknown --mode value is not rejected at flag-parse time (validated in RunE)
 * @edge-cases
 *   - "agents create" without --file → cobra returns flag-required error mentioning "file"
 *   - --mode flag accepts all four valid values without error at flag level
 *   - --yes/-y is a boolean flag (Type() == "bool")
 *   - --output/-o defaults to "table"
 * @see ./agents.go (RegisterAgents, registerAgentCreate)
 */

package commands

import (
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
// Registration
// ---------------------------------------------------------------------------

// TestAgentCreate_SubcommandPresence verifies that "create" is registered under "agents".
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

// TestAgentCreate_AllSiblingSubcommandsStillPresent ensures adding "create" does not
// break the existing sibling commands.
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
// Flag registration
// ---------------------------------------------------------------------------

// TestAgentCreate_FileFlagRegistered verifies --file/-f is declared as a string flag.
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
	// Shorthand.
	if f.Shorthand != "f" {
		t.Errorf("expected --file shorthand to be 'f', got %q", f.Shorthand)
	}
}

// TestAgentCreate_FileFlagRequired verifies --file is marked required via cobra annotation.
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

// TestAgentCreate_ModeFlagRegistered verifies --mode is declared as a string flag
// with default "auto".
func TestAgentCreate_ModeFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("mode")
	if f == nil {
		t.Fatal("expected flag --mode on \"create\" command")
	}
	if f.Value.Type() != "string" {
		t.Errorf("expected --mode to be string, got %q", f.Value.Type())
	}
	if f.DefValue != "auto" {
		t.Errorf("expected --mode default to be \"auto\", got %q", f.DefValue)
	}
}

// TestAgentCreate_OutputFlagRegistered verifies --output/-o is declared as a string flag
// with default "table".
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

// TestAgentCreate_YesFlagRegistered verifies --yes/-y is declared as a boolean flag
// defaulting to false.
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

// TestAgentCreate_OrgFlagRegistered verifies --org is declared as a string flag.
func TestAgentCreate_OrgFlagRegistered(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	createCmd := findCreateCmd(t, rootCmd)

	f := createCmd.Flags().Lookup("org")
	if f == nil {
		t.Fatal("expected flag --org on \"create\" command")
	}
	if f.Value.Type() != "string" {
		t.Errorf("expected --org to be string, got %q", f.Value.Type())
	}
}

// ---------------------------------------------------------------------------
// Flag enforcement
// ---------------------------------------------------------------------------

// TestAgentCreate_FileRequired_ErrorWhenMissing verifies that executing "agents create"
// without --file causes cobra to return a flag-required error before RunE executes.
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

// TestAgentCreate_NoArgs_AcceptsNoPositionalArgs verifies that the command uses
// cobra.NoArgs (zero positional arguments required).
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

// ---------------------------------------------------------------------------
// --mode flag value acceptance (flag-level, not RunE validation)
// ---------------------------------------------------------------------------

// TestAgentCreate_ModeFlag_AcceptsValidValues verifies that the flag-set correctly
// stores each of the four valid mode strings when set via cobra flag parsing.
func TestAgentCreate_ModeFlag_AcceptsValidValues(t *testing.T) {
	validModes := []string{"auto", "create", "upsert", "patch"}

	for _, mode := range validModes {
		t.Run("mode="+mode, func(t *testing.T) {
			rootCmd := buildAgentsRoot(t)
			createCmd := findCreateCmd(t, rootCmd)

			if err := createCmd.Flags().Set("mode", mode); err != nil {
				t.Errorf("expected mode %q to be accepted by flag, got error: %v", mode, err)
			}
			got := createCmd.Flags().Lookup("mode").Value.String()
			if got != mode {
				t.Errorf("after Set(mode, %q), got %q", mode, got)
			}
		})
	}
}

// TestAgentCreate_OutputFlag_AcceptsValidValues verifies --output accepts
// table, json, yaml at the flag level.
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

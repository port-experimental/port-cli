/**
 * @spec-handoff
 * @interface RegisterAgents(rootCmd *cobra.Command)
 * @behavior
 *   - Registers an "agents" subcommand on rootCmd
 *   - "agents" exposes subcommands: invoke, list, get, update, create
 *   - "list" accepts flags: --org (string), --output/-o (string)
 *   - "get" accepts flags: --org (string), --output/-o (string); requires exactly 1 positional arg
 *   - "update" accepts flags: --org (string), --output/-o (string), --prompt-file (string, required)
 *   - "create" accepts flags: --org (string), --file/-f (string, required), --force (bool), --patch (bool), --yes/-y (bool), --output/-o (string)
 * @edge-cases
 *   - "update" invoked without --prompt-file must return a cobra flag-required error
 *   - "--force and --patch are mutually exclusive — both set must return error"
 * @see ./agents.go
 */

package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildAgentsRoot creates a minimal rootCmd with agents registered.
func buildAgentsRoot(t *testing.T) *cobra.Command {
	t.Helper()
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAgents(rootCmd)
	return rootCmd
}

// findAgentsCmd returns the "agents" command from rootCmd.
func findAgentsCmd(t *testing.T, rootCmd *cobra.Command) *cobra.Command {
	t.Helper()
	agentsCmd, _, err := rootCmd.Find([]string{"agents"})
	if err != nil || agentsCmd == nil || agentsCmd.Use == rootCmd.Use {
		t.Fatal("agents command not registered on rootCmd")
	}
	return agentsCmd
}

// findSubcmd returns a subcommand by name under parentCmd.
func findSubcmd(t *testing.T, parentCmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	sub, _, _ := parentCmd.Find([]string{name})
	if sub == nil || sub == parentCmd {
		t.Fatalf("subcommand %q not found under %q", name, parentCmd.Use)
	}
	return sub
}

// TestRegisterAgents_SubcommandPresence verifies that RegisterAgents adds all five
// expected subcommands: invoke, list, get, update, create.
func TestRegisterAgents_SubcommandPresence(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)

	expectedSubs := []string{"invoke", "list", "get", "update", "create"}
	for _, sub := range expectedSubs {
		found := false
		for _, cmd := range agentsCmd.Commands() {
			if cmd.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q under 'agents', not found", sub)
		}
	}
}

// TestAgentList_FlagRegistration verifies that the "list" subcommand declares
// the expected flags with correct types.
func TestAgentList_FlagRegistration(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)
	listCmd := findSubcmd(t, agentsCmd, "list")

	if f := listCmd.Flags().Lookup("org"); f == nil {
		t.Error("expected flag --org on 'list' command")
	} else if f.Value.Type() != "string" {
		t.Errorf("expected --org to be string, got %q", f.Value.Type())
	}

	if f := listCmd.Flags().Lookup("output"); f == nil {
		t.Error("expected flag --output on 'list' command")
	} else if f.Value.Type() != "string" {
		t.Errorf("expected --output to be string, got %q", f.Value.Type())
	}
}

// TestAgentGet_FlagRegistration verifies that the "get" subcommand declares
// the expected flags and enforces exactly 1 positional argument.
func TestAgentGet_FlagRegistration(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)
	getCmd := findSubcmd(t, agentsCmd, "get")

	if f := getCmd.Flags().Lookup("org"); f == nil {
		t.Error("expected flag --org on 'get' command")
	}

	if f := getCmd.Flags().Lookup("output"); f == nil {
		t.Error("expected flag --output on 'get' command")
	}

	// Validate Args policy: exactly 1 argument expected.
	// cobra.ExactArgs(1) rejects 0 and 2+ args.
	if err := getCmd.Args(getCmd, []string{}); err == nil {
		t.Error("expected Args to reject 0 arguments, got nil error")
	}
	if err := getCmd.Args(getCmd, []string{"agent-a", "extra"}); err == nil {
		t.Error("expected Args to reject 2 arguments, got nil error")
	}
	if err := getCmd.Args(getCmd, []string{"agent-a"}); err != nil {
		t.Errorf("expected Args to accept exactly 1 argument, got error: %v", err)
	}
}

// TestAgentUpdate_FlagRegistration verifies that the "update" subcommand declares
// the expected flags including the required --prompt-file flag.
func TestAgentUpdate_FlagRegistration(t *testing.T) {
	rootCmd := buildAgentsRoot(t)
	agentsCmd := findAgentsCmd(t, rootCmd)
	updateCmd := findSubcmd(t, agentsCmd, "update")

	if f := updateCmd.Flags().Lookup("org"); f == nil {
		t.Error("expected flag --org on 'update' command")
	}

	if f := updateCmd.Flags().Lookup("output"); f == nil {
		t.Error("expected flag --output on 'update' command")
	}

	promptFileFlag := updateCmd.Flags().Lookup("prompt-file")
	if promptFileFlag == nil {
		t.Fatal("expected flag --prompt-file on 'update' command")
	}
	if promptFileFlag.Value.Type() != "string" {
		t.Errorf("expected --prompt-file to be string, got %q", promptFileFlag.Value.Type())
	}

	// Verify the flag is marked required via its annotations.
	annotations := promptFileFlag.Annotations
	if requiredAnnotation, ok := annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Error("expected --prompt-file to be marked as required via cobra annotation")
	} else if len(requiredAnnotation) == 0 || requiredAnnotation[0] != "true" {
		t.Errorf("expected BashCompOneRequiredFlag annotation to be 'true', got %v", requiredAnnotation)
	}
}

// TestAgentUpdate_PromptFileRequired verifies that executing the "update" command
// without --prompt-file causes cobra to return a flag-required error before RunE runs.
func TestAgentUpdate_PromptFileRequired(t *testing.T) {
	rootCmd := buildAgentsRoot(t)

	// Redirect output to avoid noise in test results.
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	// Set args: agents update <agent-id> — missing --prompt-file
	rootCmd.SetArgs([]string{"agents", "update", "some-agent-id"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --prompt-file is missing, got nil")
	}

	// The error should mention the missing required flag.
	if !strings.Contains(err.Error(), "prompt-file") {
		t.Errorf("expected error to mention 'prompt-file', got: %q", err.Error())
	}
}

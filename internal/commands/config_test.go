package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigGetArgsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterConfig(rootCmd)

	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd == nil {
		t.Fatal("config command not found")
	}

	getCmd, _, _ := configCmd.Find([]string{"get"})
	if getCmd == nil {
		t.Fatal("get command not found")
	}

	err := getCmd.Args(getCmd, []string{"default_org"})
	if err != nil {
		t.Errorf("unexpected error parsing args: %v", err)
	}
}

func TestConfigSetArgsParsed(t *testing.T) {
	rootCmd := &cobra.Command{Use: "port"}
	RegisterConfig(rootCmd)

	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd == nil {
		t.Fatal("config command not found")
	}

	setCmd, _, _ := configCmd.Find([]string{"set"})
	if setCmd == nil {
		t.Fatal("set command not found")
	}

	err := setCmd.Args(setCmd, []string{"default_org", "demo"})
	if err != nil {
		t.Errorf("unexpected error parsing args: %v", err)
	}
}

package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthLoginFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAuth(rootCmd)

	authCmd, _, _ := rootCmd.Find([]string{"auth"})
	if authCmd == nil {
		t.Fatal("auth command not found")
	}

	loginCmd, _, _ := authCmd.Find([]string{"login"})
	if loginCmd == nil {
		t.Fatal("login command not found")
	}

	// Parse args without executing RunE
	loginCmd.DisableFlagParsing = false
	err := loginCmd.ParseFlags([]string{
		"--org", "local",
		"--with-token",
	})
	if err != nil {
		t.Errorf("unexpected error parsing flags: %v", err)
	}

	org, err := loginCmd.Flags().GetString("org")
	if err != nil {
		t.Fatalf("could not get --org %v", err)
	}
	if org != "local" {
		t.Errorf("expected 'local', got %q", org)
	}

	if _, err := loginCmd.Flags().GetBool("with-token"); err != nil {
		t.Fatalf("could not get --with-token %v", err)
	}
}

func TestAuthLogoutFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAuth(rootCmd)

	authCmd, _, _ := rootCmd.Find([]string{"auth"})
	if authCmd == nil {
		t.Fatal("auth command not found")
	}

	logoutCmd, _, _ := authCmd.Find([]string{"logout"})
	if logoutCmd == nil {
		t.Fatal("login command not found")
	}

	// Parse args without executing RunE
	logoutCmd.DisableFlagParsing = false
	err := logoutCmd.ParseFlags([]string{
		"--org", "local",
	})
	if err != nil {
		t.Errorf("unexpected error parsing flags: %v", err)
	}

	org, err := logoutCmd.Flags().GetString("org")
	if err != nil {
		t.Fatalf("could not get --org %v", err)
	}
	if org != "local" {
		t.Errorf("expected 'local', got %q", org)
	}
}

func TestAuthTokenFlagsParsed(t *testing.T) {
	// Verify that the flags are accepted without error at parse time (args parsing only)
	rootCmd := &cobra.Command{Use: "port"}
	RegisterAuth(rootCmd)

	authCmd, _, _ := rootCmd.Find([]string{"auth"})
	if authCmd == nil {
		t.Fatal("auth command not found")
	}

	tokenCmd, _, _ := authCmd.Find([]string{"token"})
	if tokenCmd == nil {
		t.Fatal("token command not found")
	}

	// Parse args without executing RunE
	tokenCmd.DisableFlagParsing = false
	err := tokenCmd.ParseFlags([]string{
		"--org", "local",
	})
	if err != nil {
		t.Errorf("unexpected error parsing flags: %v", err)
	}

	org, err := tokenCmd.Flags().GetString("org")
	if err != nil {
		t.Fatalf("could not get --org %v", err)
	}
	if org != "local" {
		t.Errorf("expected 'local', got %q", org)
	}
}

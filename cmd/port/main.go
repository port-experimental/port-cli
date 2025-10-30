package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/port-labs/port-cli/internal/commands"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildDate = "unknown"
	commit    = "unknown"
)

func init() {
	// Try to get build info from runtime
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "dev" {
			// Try to get version from build info
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && commit == "unknown" {
					commit = setting.Value
					if len(commit) > 7 {
						commit = commit[:7]
					}
				}
				if setting.Key == "vcs.time" && buildDate == "unknown" {
					buildDate = setting.Value
				}
			}
		}
	}

	// Set build info in commands package
	commands.SetBuildInfo(commands.BuildInfo{
		Version:   version,
		BuildDate: buildDate,
		Commit:    commit,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	})
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "port",
		Short: "Port CLI - Modular command-line interface for Port",
		Long: `Port CLI - Modular command-line interface for Port

Manage your Port organization with import/export, migration, and API operations.

Credentials can be provided via:
  1. CLI flags (--client-id, --client-secret) - highest priority
  2. Environment variables (PORT_CLIENT_ID, PORT_CLIENT_SECRET)
  3. Configuration file (~/.port/config.yaml)`,
		Version: version,
	}

	// Global flags
	var (
		configFile        string
		clientID          string
		clientSecret      string
		apiURL            string
		targetClientID    string
		targetClientSecret string
		targetAPIURL       string
		debug             bool
	)

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&clientID, "client-id", "", "Base org Port API client ID (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&clientSecret, "client-secret", "", "Base org Port API client secret (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Base org Port API URL (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&targetClientID, "target-client-id", "", "Target org Port API client ID (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&targetClientSecret, "target-client-secret", "", "Target org Port API client secret (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&targetAPIURL, "target-api-url", "", "Target org Port API URL (overrides config/env)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
	rootCmd.PersistentFlags().MarkHidden("debug")

	// Store global flags in context
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(commands.WithGlobalFlags(cmd.Context(), commands.GlobalFlags{
			ConfigFile:        configFile,
			ClientID:          clientID,
			ClientSecret:      clientSecret,
			APIURL:             apiURL,
			TargetClientID:     targetClientID,
			TargetClientSecret: targetClientSecret,
			TargetAPIURL:       targetAPIURL,
			Debug:              debug,
		}))
	}

	// Add subcommands
	commands.RegisterExport(rootCmd)
	commands.RegisterImport(rootCmd)
	commands.RegisterMigrate(rootCmd)
	commands.RegisterAPI(rootCmd)
	commands.RegisterVersion(rootCmd)
	commands.RegisterConfig(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}


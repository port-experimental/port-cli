package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/port-experimental/port-cli/internal/commands"
	"github.com/port-experimental/port-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.3"
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
		configFile         string
		clientID           string
		clientSecret       string
		apiURL             string
		targetClientID     string
		targetClientSecret string
		targetAPIURL       string
		debug              bool
		noColor            bool
		quiet              bool
		verbose            bool
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
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Store global flags in context and initialize color output
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Initialize color output early
		output.Init(noColor)

		// Initialize verbosity
		if quiet {
			output.SetVerbosity(output.QuietLevel)
		} else if verbose {
			output.SetVerbosity(output.VerboseLevel)
		} else {
			output.SetVerbosity(output.NormalLevel)
		}

		cmd.SetContext(commands.WithGlobalFlags(cmd.Context(), commands.GlobalFlags{
			ConfigFile:         configFile,
			ClientID:           clientID,
			ClientSecret:       clientSecret,
			APIURL:             apiURL,
			TargetClientID:     targetClientID,
			TargetClientSecret: targetClientSecret,
			TargetAPIURL:       targetAPIURL,
			Debug:              debug,
			NoColor:            noColor,
			Quiet:              quiet,
			Verbose:            verbose,
		}))
	}

	// Add subcommands
	commands.RegisterExport(rootCmd)
	commands.RegisterImport(rootCmd)
	commands.RegisterMigrate(rootCmd)
	commands.RegisterCompare(rootCmd)
	commands.RegisterAPI(rootCmd)
	commands.RegisterVersion(rootCmd)
	commands.RegisterConfig(rootCmd)
	commands.RegisterCompletion(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		// Initialize output in case PreRun didn't execute
		output.Init(false)
		output.SetVerbosity(output.NormalLevel)
		formattedErr := output.FormatError(err)
		if formattedErr != "" {
			output.ErrorPrintf("%s\n", formattedErr)
		} else {
			output.ErrorPrintf("%s: %v\n", output.Error("Error"), err)
		}
		os.Exit(1)
	}
}

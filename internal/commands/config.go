package commands

import (
	"fmt"

	"github.com/port-labs/port-cli/internal/config"
	"github.com/spf13/cobra"
)

// RegisterConfig registers the config command.
func RegisterConfig(rootCmd *cobra.Command) {
	var show, init bool

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Port CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			if init {
				if err := configManager.CreateDefaultConfig(); err != nil {
					return fmt.Errorf("failed to create configuration: %w", err)
				}
				fmt.Printf("✓ Configuration file created at %s\n", configManager.ConfigPath())
				fmt.Println("\nPlease edit the file and add your Port credentials.")
				return nil
			}

			if show {
				cfg, err := configManager.Load()
				if err != nil {
					return fmt.Errorf("failed to load configuration: %w", err)
				}

				fmt.Println("\nCurrent Configuration:")
				fmt.Printf("Config file: %s\n", configManager.ConfigPath())
				fmt.Printf("Default org: %s\n", cfg.DefaultOrg)
				fmt.Printf("Backend URL: %s\n", cfg.Backend.URL)
				fmt.Printf("Organizations: %d\n", len(cfg.Organizations))
				for orgName := range cfg.Organizations {
					fmt.Printf("  - %s\n", orgName)
				}
				return nil
			}

			fmt.Println("Use --show to display configuration")
			fmt.Println("Use --init to create a new configuration file")
			return nil
		},
	}

	configCmd.Flags().BoolVar(&show, "show", false, "Show current configuration")
	configCmd.Flags().BoolVar(&init, "init", false, "Initialize configuration file")

	rootCmd.AddCommand(configCmd)
}


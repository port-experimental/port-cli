package commands

import (
	"github.com/spf13/cobra"
)

// RegisterCompletion registers the completion command.
func RegisterCompletion(rootCmd *cobra.Command) {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for port command.

To load completions in your current shell session:
  bash: source <(port completion bash)
  zsh:  source <(port completion zsh)
  fish: port completion fish | source

To load completions for all new shells:
  bash: port completion bash > /etc/bash_completion.d/port
  zsh:  port completion zsh > "${fpath[1]}/_port"
  fish: port completion fish > ~/.config/fish/completions/port.fish`,
	}

	bashCmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
		},
	}

	zshCmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		},
	}

	fishCmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}

	powershellCmd := &cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd.GenPowerShellCompletion(cmd.OutOrStdout())
		},
	}

	completionCmd.AddCommand(bashCmd)
	completionCmd.AddCommand(zshCmd)
	completionCmd.AddCommand(fishCmd)
	completionCmd.AddCommand(powershellCmd)

	rootCmd.AddCommand(completionCmd)
}












package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/agents"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// RegisterAgents registers the "port agents" command group.
func RegisterAgents(rootCmd *cobra.Command) {
	agentsCmd := &cobra.Command{
		Use:   "agents",
		Short: "Invoke and manage Port AI Agents",
		Long:  "Invoke Port AI Agents and manage their configuration.",
	}
	agentsCmd.AddCommand(registerAgentInvoke())
	rootCmd.AddCommand(agentsCmd)
}

func registerAgentInvoke() *cobra.Command {
	var (
		org    string
		raw    bool
		output string
	)

	cmd := &cobra.Command{
		Use:   "invoke <agent-id> <prompt>",
		Short: "Invoke a Port AI Agent",
		Long: `Invoke a Port AI Agent with a prompt and stream the response.

Progress events are written to stderr; the final output goes to stdout.
Use --raw to dump all SSE events as JSON (useful for debugging).
Use --output json to get a structured JSON result instead of plain text.

Examples:
  port agents invoke triage_agent "Storage account for the payments system"
  port agents invoke triage_agent "Virtual network for PCI workloads" --output json
  port agents invoke triage_agent "Key Vault for SOC logs" --raw`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]
			prompt := args[1]

			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				org,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			orgConfig, err := cfg.GetOrgConfig(useOrg)
			if err != nil {
				return err
			}

			token, err := getOrRefreshCommandToken(cmd, configManager, useOrg)
			if err != nil {
				return err
			}

			client := api.NewClient(api.ClientOpts{
				Token:        token,
				ClientID:     orgConfig.ClientID,
				ClientSecret: orgConfig.ClientSecret,
				APIURL:       orgConfig.APIURL,
			})
			defer client.Close()

			lipgloss.Fprintf(os.Stderr, "%s Invoking agent %s…\n",
				styles.CheckMark, styles.Bold.Render(agentID))

			executionStarted := false

			var onProgress func(string, string)
			if raw {
				onProgress = func(eventType string, data string) {
					enc := json.NewEncoder(os.Stdout)
					_ = enc.Encode(map[string]any{
						"type": eventType,
						"data": data,
					})
				}
			} else {
				onProgress = func(eventType string, data string) {
					switch eventType {
					case "invocationIdentifier":
						// data is plain UUID string
						lipgloss.Fprintf(os.Stderr, "  › ID: %s\n", data)
					case "waiting":
						lipgloss.Fprintf(os.Stderr, "  %s waiting…\n", styles.Circle)
					case "execution":
						if !executionStarted {
							lipgloss.Fprintf(os.Stderr, "  %s executing…\n", styles.Circle)
							executionStarted = true
						}
					case "toolPrep", "toolCall":
						var p struct {
							Name string `json:"name"`
						}
						if json.Unmarshal([]byte(data), &p) == nil && p.Name != "" {
							lipgloss.Fprintf(os.Stderr, "  %s tool: %s\n",
								styles.Circle, styles.Bold.Render(p.Name))
						}
					case "done":
						var p struct {
							MonthlyQuotaUsage struct {
								RemainingQuota int `json:"remainingQuota"`
								MonthlyLimit   int `json:"monthlyLimit"`
							} `json:"monthlyQuotaUsage"`
						}
						if err := json.Unmarshal([]byte(data), &p); err == nil && p.MonthlyQuotaUsage.MonthlyLimit > 0 {
							remaining := p.MonthlyQuotaUsage.RemainingQuota
							if remaining < 0 {
								remaining = 0
							}
							lipgloss.Fprintf(os.Stderr, "%s done  [quota: %d/%d remaining]\n",
								styles.CheckMark,
								remaining,
								p.MonthlyQuotaUsage.MonthlyLimit)
						} else {
							lipgloss.Fprintf(os.Stderr, "%s done\n", styles.CheckMark)
						}
					}
				}
			}

			result, err := agents.Invoke(cmd.Context(), client, agents.InvokeOptions{
				AgentID:    agentID,
				Prompt:     prompt,
				OnProgress: onProgress,
			})
			if err != nil {
				return fmt.Errorf("agent invocation failed: %w", err)
			}

			// Surface ask_user_questions if the agent needs more input
			if len(result.AskUserQuestions) > 0 {
				lipgloss.Fprintf(os.Stderr,
					"\n%s The agent needs more information:\n", styles.QuestionMark)
				for i, q := range result.AskUserQuestions {
					lipgloss.Fprintf(os.Stderr, "  %d. %s\n", i+1, q)
				}
				lipgloss.Fprintf(os.Stderr,
					"\nRe-invoke with a prompt that answers the questions above.\n")
				// Exit non-zero so callers and scripts can detect that more input is required.
				// os.Exit(1) is used directly (matching the pattern in compare.go) because
				// cobra.ErrSilent does not exist in cobra v1.9.1 and the user-friendly
				// message has already been printed to stderr above.
				os.Exit(1)
				return nil // unreachable; satisfies the error return type
			}

			switch strings.ToLower(output) {
			case "json":
				return formatOutput(map[string]any{
					"output":            result.Output,
					"invocationId":      result.InvocationID,
					"askUserQuestions":  result.AskUserQuestions,
					"monthlyQuotaUsage": result.MonthlyQuotaUsage,
					"rateLimitUsage":    result.RateLimitUsage,
					"contextUsage":      result.ContextUsage,
				}, "json")
			default:
				if !raw {
					if result.Output != "" {
						fmt.Println(result.Output)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Dump all SSE events as newline-delimited JSON to stdout (for scripting/debugging)")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text, json")

	return cmd
}

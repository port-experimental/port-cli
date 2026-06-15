package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

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
	agentsCmd.AddCommand(registerAgentList())
	agentsCmd.AddCommand(registerAgentGet())
	agentsCmd.AddCommand(registerAgentUpdate())
	agentsCmd.AddCommand(registerAgentCreate())
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

func registerAgentList() *cobra.Command {
	var (
		org    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all Port AI Agents in the organization",
		Long: `List all Port AI Agents registered in your organization.

Displays a table with each agent's identifier and title. Use --output json or
--output yaml to retrieve the full agent payload including blueprint, timestamps,
and properties.

Examples:
  port agents list
  port agents list --output json
  port agents list --output yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			result, err := agents.List(cmd.Context(), client, agents.ListOptions{})
			if err != nil {
				return fmt.Errorf("failed to list agents: %w", err)
			}

			agentsData := make([]map[string]interface{}, len(result.Entities))
			for i, a := range result.Entities {
				agentsData[i] = map[string]interface{}{
					"identifier": a.Identifier,
					"title":      a.Title,
					"blueprint":  a.Blueprint,
					"createdAt":  a.CreatedAt,
					"updatedAt":  a.UpdatedAt,
					"properties": a.Properties,
				}
			}

			switch strings.ToLower(output) {
			case "json", "yaml":
				return formatOutput(map[string]interface{}{"agents": agentsData}, strings.ToLower(output))
			default: // table
				if len(result.Entities) == 0 {
					fmt.Println("(no agents found)")
					return nil
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "IDENTIFIER\tTITLE")
				fmt.Fprintln(w, "──────────────────────────────\t──────────────────────────────────────")
				for _, a := range result.Entities {
					fmt.Fprintf(w, "%s\t%s\n", a.Identifier, a.Title)
				}
				w.Flush()
				fmt.Printf("\n%d agents\n", len(result.Entities))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")

	return cmd
}

func registerAgentGet() *cobra.Command {
	var (
		org    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "get <agent-id>",
		Short: "Get a Port AI Agent by identifier",
		Long: `Show full details for a single Port AI Agent.

Prints the agent's identifier, title, blueprint, timestamps, and property keys.
If the agent has a system prompt property (prompt, system_prompt, systemPrompt,
or instructions), a preview of up to 300 characters is shown in table mode.

Use --output json or --output yaml to retrieve the complete agent payload.

Examples:
  port agents get triage_agent
  port agents get triage_agent --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

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

			result, err := agents.Get(cmd.Context(), client, agents.GetOptions{AgentID: agentID})
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			entityData := map[string]interface{}{
				"identifier": result.Entity.Identifier,
				"title":      result.Entity.Title,
				"blueprint":  result.Entity.Blueprint,
				"createdAt":  result.Entity.CreatedAt,
				"updatedAt":  result.Entity.UpdatedAt,
				"properties": result.Entity.Properties,
			}

			switch strings.ToLower(output) {
			case "json", "yaml":
				return formatOutput(entityData, strings.ToLower(output))
			default: // table
				e := result.Entity

				// Collect property keys for display in deterministic order.
				propKeys := make([]string, 0, len(e.Properties))
				for k := range e.Properties {
					propKeys = append(propKeys, k)
				}
				sort.Strings(propKeys)

				fmt.Printf("Identifier:    %s\n", e.Identifier)
				fmt.Printf("Title:         %s\n", e.Title)
				fmt.Printf("Blueprint:     %s\n", e.Blueprint)
				fmt.Printf("Created:       %s\n", e.CreatedAt)
				fmt.Printf("Updated:       %s\n", e.UpdatedAt)
				fmt.Printf("Properties:    %s\n", strings.Join(propKeys, ", "))

				// Preview the prompt from the first matching candidate.
				for _, candidate := range []string{"prompt", "system_prompt", "systemPrompt", "instructions"} {
					val, ok := e.Properties[candidate]
					if !ok {
						continue
					}
					s, isStr := val.(string)
					if !isStr || s == "" {
						continue
					}
					preview := s
					if len(preview) > 300 {
						preview = preview[:300] + "…"
					}
					fmt.Printf("\n--- %s (preview) ---\n%s\n", candidate, preview)
					break
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")

	return cmd
}

func registerAgentUpdate() *cobra.Command {
	var (
		org        string
		output     string
		promptFile string
	)

	cmd := &cobra.Command{
		Use:   "update <agent-id>",
		Short: "Update a Port AI Agent's system prompt",
		Long: `Replace the system prompt of an existing Port AI Agent.

Reads the new prompt from a local file (--prompt-file) and PATCHes the agent
entity in Port. The file should contain plain text — the entire contents will
become the agent's new system prompt.

Use --output json or --output yaml to get the updated agent entity as structured
output.

Examples:
  port agents update triage_agent --prompt-file ./prompt.txt
  port agents update triage_agent --prompt-file ./prompt.txt --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			content, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("failed to read prompt file: %w", err)
			}

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

			result, err := agents.Update(cmd.Context(), client, agents.UpdateOptions{
				AgentID:   agentID,
				NewPrompt: string(content),
			})
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			entityData := map[string]interface{}{
				"identifier": result.Entity.Identifier,
				"title":      result.Entity.Title,
				"blueprint":  result.Entity.Blueprint,
				"createdAt":  result.Entity.CreatedAt,
				"updatedAt":  result.Entity.UpdatedAt,
				"properties": result.Entity.Properties,
			}

			switch strings.ToLower(output) {
			case "json", "yaml":
				return formatOutput(entityData, strings.ToLower(output))
			default: // table
				fmt.Printf("%s Agent %s updated successfully\n", styles.CheckMark, agentID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Path to file containing the new system prompt")
	cmd.MarkFlagRequired("prompt-file")

	return cmd
}

func registerAgentCreate() *cobra.Command {
	var (
		org    string
		file   string
		force  bool
		patch  bool
		yes    bool
		output string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Port AI Agent from a .md file",
		Long: `Create a Port AI Agent from a Markdown file with YAML frontmatter.

The file must contain a YAML frontmatter block delimited by "---" lines, followed
by the agent's system prompt as the body. The identifier field in the frontmatter
is required; all other fields are optional.

Default behavior:
  Checks if the agent exists first. If not, creates it. If it does exist, prints
  an error and exits 1 — use --force to overwrite.

  --force   Create if new; fully replace if exists. Never fails due to existence.
  --patch   Partially update an existing agent. Only sends non-empty fields from
            the .md file. Fails if the agent does not exist.

--force and --patch are mutually exclusive.

Examples:
  port agents create --file triage_agent.md
  port agents create --file triage_agent.md --force
  port agents create --file triage_agent.md --force --yes
  port agents create --file triage_agent.md --patch --output json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutually exclusive flags early — before any network I/O.
			if force && patch {
				return fmt.Errorf("--force and --patch are mutually exclusive")
			}

			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)

			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID, flags.ClientSecret, flags.APIURL, org,
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

			result, err := agents.Create(cmd.Context(), client, agents.CreateOptions{
				File:   file,
				Force:  force,
				Patch:  patch,
				Yes:    yes,
				Output: output,
			})
			if err != nil {
				if errors.Is(err, agents.ErrConfirmationDeclined) {
					lipgloss.Fprintf(os.Stderr, "%s Cancelled — no changes made.\n", styles.ExclamationMark)
					return nil
				}
				return fmt.Errorf("failed to create agent: %w", err)
			}

			entityData := map[string]interface{}{
				"identifier": result.Entity.Identifier,
				"title":      result.Entity.Title,
				"blueprint":  result.Entity.Blueprint,
				"createdAt":  result.Entity.CreatedAt,
				"updatedAt":  result.Entity.UpdatedAt,
				"properties": result.Entity.Properties,
			}

			switch strings.ToLower(output) {
			case "json", "yaml":
				return formatOutput(entityData, strings.ToLower(output))
			default: // table
				lipgloss.Fprintf(
					os.Stdout, "%s Agent %s %s\n",
					styles.CheckMark,
					styles.Bold.Render(result.Entity.Identifier),
					result.Action,
				)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "Identifier:\t%s\n", result.Entity.Identifier)
				fmt.Fprintf(w, "Title:\t%s\n", result.Entity.Title)
				fmt.Fprintf(w, "Action:\t%s\n", result.Action)
				fmt.Fprintf(w, "Updated:\t%s\n", result.Entity.UpdatedAt)
				fmt.Fprintf(w, "Prompt key:\t%s\n", result.PromptKey)
				w.Flush()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to the agent .md file (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Create if new; replace if exists")
	cmd.Flags().BoolVar(&patch, "patch", false, "Partially update an existing agent (fails if not found)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")
	cmd.MarkFlagRequired("file")

	return cmd
}

package agents

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/port-experimental/port-cli/internal/api"
)

// Create creates, upserts, or patches a _ai_agent entity in Port from a .md file.
func Create(ctx context.Context, apiClient *api.Client, opts CreateOptions) (*CreateResult, error) {
	if opts.File == "" {
		return nil, errors.New("file is required")
	}

	spec, err := ParseAgentFile(opts.File)
	if err != nil {
		return nil, err
	}

	// Determine effective mode and prompt key.
	effectiveMode := opts.Mode
	promptKey := "prompt"

	switch opts.Mode {
	case CreateModeAuto:
		existing, getErr := apiClient.GetEntity(ctx, agentBlueprint, spec.Identifier)
		if getErr != nil {
			// Check if it's a 404 — if so, use create path.
			if is404Error(getErr) {
				effectiveMode = CreateModeCreate
				promptKey = "prompt"
			} else {
				return nil, getErr
			}
		} else {
			effectiveMode = CreateModeUpsert
			// Try to detect the prompt property from existing entity.
			existingAgent := parseAgentEntity(existing)
			if key, detectErr := detectPromptProperty(existingAgent); detectErr == nil {
				promptKey = key
			}
			// On detection failure, fall back to "prompt" silently.
		}

	case CreateModePatch:
		existing, getErr := apiClient.GetEntity(ctx, agentBlueprint, spec.Identifier)
		if getErr != nil {
			return nil, getErr
		}
		existingAgent := parseAgentEntity(existing)
		if key, detectErr := detectPromptProperty(existingAgent); detectErr == nil {
			promptKey = key
		}
		// On detection failure, fall back to "prompt" silently.

	case CreateModeCreate, CreateModeUpsert:
		// No GET needed; always use "prompt" as default key.
		promptKey = "prompt"

	default:
		promptKey = "prompt"
	}

	// Run confirmation prompt if not skipped.
	if !opts.Yes {
		if err := runConfirmation(spec, effectiveMode); err != nil {
			return nil, err
		}
	}

	// Coerce nil tools to empty slice to avoid JSON null.
	tools := spec.Tools
	if tools == nil {
		tools = []string{}
	}

	var (
		raw    map[string]interface{}
		action string
		apiErr error
	)

	switch effectiveMode {
	case CreateModeCreate:
		body := buildCreateBody(spec, tools, promptKey)
		raw, apiErr = apiClient.CreateEntityWithParams(ctx, agentBlueprint, body, false, false)
		action = "created"

	case CreateModeUpsert:
		body := buildCreateBody(spec, tools, promptKey)
		raw, apiErr = apiClient.CreateEntityWithParams(ctx, agentBlueprint, body, true, false)
		action = "upserted"

	case CreateModePatch:
		patchBody := buildPatchBody(spec, tools, promptKey)
		var patchRaw api.Entity
		patchRaw, apiErr = apiClient.PatchEntity(ctx, agentBlueprint, spec.Identifier, api.Entity(patchBody))
		if patchRaw != nil {
			raw = map[string]interface{}(patchRaw)
		}
		action = "patched"
	}

	if apiErr != nil {
		return nil, apiErr
	}

	entity := parseAgentEntity(api.Entity(raw))

	return &CreateResult{
		Entity:    entity,
		Action:    action,
		ModeUsed:  effectiveMode,
		PromptKey: promptKey,
	}, nil
}

// buildCreateBody constructs the POST request body.
func buildCreateBody(spec *AgentFileSpec, tools []string, promptKey string) map[string]interface{} {
	properties := map[string]interface{}{
		"description":    spec.Description,
		"model":          spec.Model,
		"provider":       spec.Provider,
		"execution_mode": spec.ExecutionMode,
		"status":         spec.Status,
		"tools":          tools,
		promptKey:        spec.Prompt,
	}

	return map[string]interface{}{
		"identifier": spec.Identifier,
		"title":      spec.Title,
		"blueprint":  agentBlueprint,
		"properties": properties,
	}
}

// buildPatchBody constructs the PATCH request body — only non-empty fields.
func buildPatchBody(spec *AgentFileSpec, tools []string, promptKey string) map[string]interface{} {
	patchProps := map[string]interface{}{
		promptKey: spec.Prompt, // always included
	}

	if spec.Description != "" {
		patchProps["description"] = spec.Description
	}
	if spec.Model != "" {
		patchProps["model"] = spec.Model
	}
	if spec.Provider != "" {
		patchProps["provider"] = spec.Provider
	}
	if spec.ExecutionMode != "" {
		patchProps["execution_mode"] = spec.ExecutionMode
	}
	if spec.Status != "" {
		patchProps["status"] = spec.Status
	}
	if len(tools) > 0 {
		patchProps["tools"] = tools
	}

	body := map[string]interface{}{
		"properties": patchProps,
	}
	if spec.Title != "" {
		body["title"] = spec.Title
	}

	return body
}

// runConfirmation shows the confirmation summary and prompts the user.
// Returns ErrConfirmationDeclined if the user declines or it's a non-TTY.
func runConfirmation(spec *AgentFileSpec, mode CreateMode) error {
	// Print summary to stderr.
	promptPreview := spec.Prompt
	if promptPreview == "" {
		promptPreview = "(no prompt in file)"
	} else if len(promptPreview) > 100 {
		promptPreview = promptPreview[:100] + "…"
	}

	fmt.Fprintf(os.Stderr, "\nAgent to write\n")
	fmt.Fprintf(os.Stderr, "──────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "Identifier:     %s\n", spec.Identifier)
	fmt.Fprintf(os.Stderr, "Title:          %s\n", spec.Title)
	fmt.Fprintf(os.Stderr, "Mode:           %s\n", string(mode))
	fmt.Fprintf(os.Stderr, "Prompt preview: %s\n", promptPreview)
	fmt.Fprintf(os.Stderr, "──────────────────────────────────────────\n\n")

	// In non-TTY environments, treat as declined.
	if !term.IsTerminal(os.Stdin.Fd()) {
		return ErrConfirmationDeclined
	}

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Apply changes to %s/%s?", agentBlueprint, spec.Identifier)).
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return ErrConfirmationDeclined
	}

	if !confirmed {
		return ErrConfirmationDeclined
	}

	return nil
}

// is404Error checks whether an error from the API client represents a 404 Not Found.
func is404Error(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// The client formats errors as: "API request to <url> <method> failed: 404 Not Found..."
	return containsStatusCode(msg, "404")
}

// containsStatusCode checks if the error message contains the given HTTP status code.
func containsStatusCode(msg, code string) bool {
	for i := 0; i <= len(msg)-len(code); i++ {
		if msg[i:i+len(code)] == code {
			return true
		}
	}
	return false
}

package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
	"github.com/port-experimental/port-cli/internal/api"
)

// Create creates, replaces, or patches a _ai_agent entity in Port from a .md file.
//
// Paths:
//   - default (Force==false, Patch==false): confirm → GET; 404 → create; 200 → error
//   - --force (Force==true): confirm → GET; 404 → create; 200 → replace
//   - --patch (Patch==true): confirm → GET; 404 → error; 200 → PATCH
//
// Confirmation runs before the GET probe so that no network calls are made
// if the user declines.
func Create(ctx context.Context, apiClient *api.Client, opts CreateOptions) (*CreateResult, error) {
	if opts.File == "" {
		return nil, errors.New("file is required")
	}

	if opts.Force && opts.Patch {
		return nil, errors.New("--force and --patch are mutually exclusive")
	}

	spec, err := ParseAgentFile(opts.File)
	if err != nil {
		return nil, err
	}

	// Determine the preliminary action verb for the confirmation summary.
	// The final action is determined after the GET probe, but we need a label
	// before any network call so the user can decline without touching the API.
	preliminaryVerb := "create"
	if opts.Force {
		preliminaryVerb = "create or replace"
	} else if opts.Patch {
		preliminaryVerb = "patch"
	}

	// Run confirmation prompt BEFORE any network call.
	// This way, declining confirmation never triggers a GET or POST.
	if !opts.Yes {
		if err := runConfirmation(spec, preliminaryVerb, opts.StdinReader); err != nil {
			return nil, err
		}
	}

	// Probe GET — all paths require knowing whether the entity exists.
	existing, getErr := apiClient.GetEntity(ctx, agentBlueprint, spec.Identifier)

	// Determine the effective path and prompt key based on probe result and flags.
	type dispatchPath int
	const (
		pathCreateNew dispatchPath = iota
		pathReplace
		pathPatch
	)

	var (
		path      dispatchPath
		promptKey = "prompt"
	)

	switch {
	case opts.Patch:
		if getErr != nil {
			if is404Error(getErr) {
				return nil, fmt.Errorf("agent %q not found; cannot patch a non-existent agent", spec.Identifier)
			}
			return nil, getErr
		}
		existingAgent := parseAgentEntity(existing)
		if key, detectErr := detectPromptProperty(existingAgent); detectErr == nil {
			promptKey = key
		}
		path = pathPatch

	case opts.Force:
		if getErr != nil {
			if is404Error(getErr) {
				promptKey = "prompt"
				path = pathCreateNew
			} else {
				return nil, getErr
			}
		} else {
			existingAgent := parseAgentEntity(existing)
			if key, detectErr := detectPromptProperty(existingAgent); detectErr == nil {
				promptKey = key
			}
			path = pathReplace
		}

	default:
		if getErr != nil {
			if is404Error(getErr) {
				promptKey = "prompt"
				path = pathCreateNew
			} else {
				return nil, getErr
			}
		} else {
			return nil, fmt.Errorf("agent %q already exists; use --force to overwrite", spec.Identifier)
		}
	}

	// Map dispatch path to result action string.
	var action string
	switch path {
	case pathCreateNew:
		action = "created"
	case pathReplace:
		action = "replaced"
	case pathPatch:
		action = "patched"
	}

	// Coerce nil tools to empty slice to avoid JSON null.
	tools := spec.Tools
	if tools == nil {
		tools = []string{}
	}

	var (
		raw    map[string]interface{}
		apiErr error
	)

	switch path {
	case pathCreateNew:
		body := buildCreateBody(spec, tools, promptKey)
		raw, apiErr = apiClient.CreateEntityWithParams(ctx, agentBlueprint, body, false, false)

	case pathReplace:
		body := buildCreateBody(spec, tools, promptKey)
		raw, apiErr = apiClient.CreateEntityWithParams(ctx, agentBlueprint, body, true, false)

	case pathPatch:
		patchBody := buildPatchBody(spec, tools, promptKey)
		var patchRaw api.Entity
		patchRaw, apiErr = apiClient.PatchEntity(ctx, agentBlueprint, spec.Identifier, api.Entity(patchBody))
		if patchRaw != nil {
			raw = map[string]interface{}(patchRaw)
		}
	}

	if apiErr != nil {
		return nil, apiErr
	}

	entity := parseAgentEntity(api.Entity(raw))

	return &CreateResult{
		Entity:    entity,
		Action:    action,
		PromptKey: promptKey,
	}, nil
}

// buildCreateBody constructs the full POST request body for create and replace paths.
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
// The identifier is never sent (immutable); title is only sent when non-empty.
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

	patchBody := map[string]interface{}{
		"properties": patchProps,
	}
	if spec.Title != "" {
		patchBody["title"] = spec.Title
	}
	return patchBody
}

// runConfirmation shows a summary and prompts the user to confirm.
// action is a human-readable verb like "create", "create or replace", or "patch".
// stdinReader is the reader for interactive input; nil means use os.Stdin.
// Returns ErrConfirmationDeclined if the user declines.
func runConfirmation(spec *AgentFileSpec, action string, stdinReader io.Reader) error {
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
	fmt.Fprintf(os.Stderr, "Action:         %s\n", action)
	fmt.Fprintf(os.Stderr, "Prompt preview: %s\n", promptPreview)
	fmt.Fprintf(os.Stderr, "──────────────────────────────────────────\n\n")

	// Injected reader seam (tests): EOF → decline; any byte → confirm.
	if stdinReader != nil {
		buf := make([]byte, 1)
		n, readErr := stdinReader.Read(buf)
		if n == 0 || readErr == io.EOF {
			return ErrConfirmationDeclined
		}
		return nil
	}

	// Real interactive path: require a TTY.
	if !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("stdin is not a terminal; use --yes to confirm non-interactively")
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
// It matches the exact phrase "404 Not Found" that the client appends to error messages,
// avoiding false positives from URLs or body content that contain "404" as a substring.
func is404Error(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "404 Not Found")
}

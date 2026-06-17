package agents

import (
	"context"
	"errors"

	"github.com/port-experimental/port-cli/internal/api"
)

// Update changes the system prompt of an AI agent.
// 1. Validates AgentID != "" and NewPrompt != ""
// 2. Calls Get to fetch current entity
// 3. Calls detectPromptProperty
// 4. Calls client.PatchEntity with {"properties": {<key>: <NewPrompt>}}
// 5. Returns the updated entity
func Update(ctx context.Context, client *api.Client, opts UpdateOptions) (*UpdateResult, error) {
	if opts.AgentID == "" {
		return nil, errors.New("agent ID is required")
	}
	if opts.NewPrompt == "" {
		return nil, errors.New("new prompt is required")
	}

	getResult, err := Get(ctx, client, GetOptions{AgentID: opts.AgentID})
	if err != nil {
		return nil, err
	}

	promptKey, err := detectPromptProperty(getResult.Entity)
	if err != nil {
		return nil, err
	}

	patch := api.Entity{
		"properties": map[string]interface{}{
			promptKey: opts.NewPrompt,
		},
	}

	raw, err := client.PatchEntity(ctx, agentBlueprint, opts.AgentID, patch)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{Entity: parseAgentEntity(raw)}, nil
}

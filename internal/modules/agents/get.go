package agents

import (
	"context"
	"errors"

	"github.com/port-experimental/port-cli/internal/api"
)

// Get retrieves a single AI agent entity by identifier.
// Returns error "agent ID is required" if opts.AgentID is empty.
// Propagates 404 and other errors from client.GetEntity.
func Get(ctx context.Context, client *api.Client, opts GetOptions) (*GetResult, error) {
	if opts.AgentID == "" {
		return nil, errors.New("agent ID is required")
	}

	raw, err := client.GetEntity(ctx, agentBlueprint, opts.AgentID)
	if err != nil {
		return nil, err
	}

	return &GetResult{Entity: parseAgentEntity(raw)}, nil
}

package agents

import (
	"context"

	"github.com/port-experimental/port-cli/internal/api"
)

// List retrieves all AI agent entities from Port.
// Uses client.GetEntities(ctx, "_ai_agent", nil).
// Returns empty (non-nil) slice when no agents exist.
func List(ctx context.Context, client *api.Client, opts ListOptions) (*ListResult, error) {
	raw, err := client.GetEntities(ctx, agentBlueprint, nil)
	if err != nil {
		return nil, err
	}

	entities := make([]AgentEntity, 0, len(raw))
	for _, r := range raw {
		entities = append(entities, parseAgentEntity(r))
	}

	return &ListResult{Entities: entities}, nil
}

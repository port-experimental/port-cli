package agents

import (
	"errors"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
)

const agentBlueprint = "_ai_agent"

var promptPropertyCandidates = []string{
	"prompt",
	"system_prompt",
	"systemPrompt",
	"instructions",
}

// AgentEntity is the structured representation of a Port AI agent entity.
type AgentEntity struct {
	Identifier string
	Title      string
	Blueprint  string
	CreatedAt  string
	UpdatedAt  string
	Properties map[string]interface{}
}

// ListOptions controls the List operation.
type ListOptions struct{}

// ListResult holds the output of a List call.
type ListResult struct {
	Entities []AgentEntity
}

// GetOptions controls the Get operation.
type GetOptions struct {
	AgentID string
}

// GetResult holds the output of a Get call.
type GetResult struct {
	Entity AgentEntity
}

// UpdateOptions controls the Update operation.
type UpdateOptions struct {
	AgentID   string
	NewPrompt string
}

// UpdateResult holds the output of an Update call.
type UpdateResult struct {
	Entity AgentEntity
}

// AgentFileSpec holds all fields parsed from an agent .md file.
// Prompt is the full body of the file (after the closing frontmatter "---"),
// trimmed of leading and trailing whitespace.
type AgentFileSpec struct {
	Identifier    string   `yaml:"identifier"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Model         string   `yaml:"model"`
	Provider      string   `yaml:"provider"`
	ExecutionMode string   `yaml:"execution_mode"`
	Status        string   `yaml:"status"`
	Tools         []string `yaml:"tools"`
	Prompt        string   // populated from body, not frontmatter
}

// CreateMode controls how the entity is written to Port.
type CreateMode string

const (
	CreateModeAuto   CreateMode = "auto"
	CreateModeCreate CreateMode = "create"
	CreateModeUpsert CreateMode = "upsert"
	CreateModePatch  CreateMode = "patch"
)

// CreateOptions are the inputs to the Create module function.
type CreateOptions struct {
	File   string     // path to the .md agent file (required)
	Mode   CreateMode // "auto" | "create" | "upsert" | "patch" (default: "auto")
	Yes    bool       // skip interactive confirmation
	Output string     // "table" | "json" | "yaml" (default: "table")
}

// CreateResult is what Create returns on success.
type CreateResult struct {
	// Entity is the agent entity as returned by the Port API after the write.
	Entity AgentEntity

	// Action describes what was done. One of: "created", "upserted", "patched".
	Action string

	// ModeUsed is the effective mode (useful when Mode == "auto").
	ModeUsed CreateMode

	// PromptKey is the property name that received the prompt value.
	PromptKey string
}

// ErrConfirmationDeclined is returned by Create when the user declines
// the interactive confirmation prompt.
var ErrConfirmationDeclined = errors.New("confirmation declined")

// detectPromptProperty finds which Properties key holds the system prompt.
// Iterates promptPropertyCandidates in order.
// Returns ("", error) if nil Properties or none found.
// Returns ("", error) if key exists but value is not a non-empty string.
func detectPromptProperty(entity AgentEntity) (string, error) {
	if entity.Properties == nil {
		return "", errors.New("entity has no properties")
	}

	for _, candidate := range promptPropertyCandidates {
		val, ok := entity.Properties[candidate]
		if !ok {
			continue
		}
		s, isStr := val.(string)
		if !isStr || s == "" {
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf("no prompt property found among candidates %v", promptPropertyCandidates)
}

// parseAgentEntity converts an untyped api.Entity map into AgentEntity.
// Rules:
//   - "identifier", "title", "blueprint", "createdAt", "updatedAt" → string (zero if missing or wrong type)
//   - "properties" → map[string]interface{} (empty map if missing or wrong type — NEVER nil)
func parseAgentEntity(raw api.Entity) AgentEntity {
	str := func(key string) string {
		v, _ := raw[key].(string)
		return v
	}

	props, ok := raw["properties"].(map[string]interface{})
	if !ok {
		props = map[string]interface{}{}
	}

	return AgentEntity{
		Identifier: str("identifier"),
		Title:      str("title"),
		Blueprint:  str("blueprint"),
		CreatedAt:  str("createdAt"),
		UpdatedAt:  str("updatedAt"),
		Properties: props,
	}
}

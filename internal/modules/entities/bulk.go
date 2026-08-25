// Package entities provides shared entity bulk upsert logic for import and migrate.
package entities

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
)

// BatchSize is the Port API hard limit for entities per bulk call.
const BatchSize = 20

// BulkClient upserts entities via the Port bulk endpoint.
type BulkClient interface {
	BulkUpsertEntities(ctx context.Context, blueprintID string, entities []api.Entity, upsert bool) ([]api.BulkEntityError, error)
}

// ChunkError records a per-entity failure from a bulk chunk.
type ChunkError struct {
	BlueprintID string
	EntityID    string
	Message     string
}

// ChunkResult is the outcome of processing one bulk chunk.
type ChunkResult struct {
	Created        int
	Updated        int
	SuccessfulKeys []string // blueprintID:entityID
	Errors         []ChunkError
	Processed      int
}

// GroupByBlueprint groups entities by their blueprint field.
func GroupByBlueprint(entities []api.Entity) map[string][]api.Entity {
	byBlueprint := make(map[string][]api.Entity)
	for _, e := range entities {
		bp, _ := e["blueprint"].(string)
		if bp != "" {
			byBlueprint[bp] = append(byBlueprint[bp], e)
		}
	}
	return byBlueprint
}

// ChunkSlice splits items into batches of at most size.
func ChunkSlice[T any](items []T, size int) [][]T {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	var chunks [][]T
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

// ProcessChunk upserts one batch for a blueprint.
// When upsert is false, 409 conflicts are retried once with upsert=true.
func ProcessChunk(ctx context.Context, client BulkClient, blueprintID string, chunk []api.Entity, upsert bool) ChunkResult {
	result := ChunkResult{Processed: len(chunk)}

	bulkErrs, err := client.BulkUpsertEntities(ctx, blueprintID, chunk, upsert)
	if err != nil {
		for _, e := range chunk {
			id, _ := e["identifier"].(string)
			result.Errors = append(result.Errors, ChunkError{
				BlueprintID: blueprintID,
				EntityID:    id,
				Message:     err.Error(),
			})
		}
		return result
	}

	errByID := make(map[string]api.BulkEntityError, len(bulkErrs))
	for _, be := range bulkErrs {
		errByID[be.Identifier] = be
	}

	var conflicts []api.Entity
	for _, entity := range chunk {
		id, _ := entity["identifier"].(string)
		if bErr, failed := errByID[id]; failed {
			if int(bErr.StatusCode) == 409 && !upsert {
				conflicts = append(conflicts, entity)
			} else {
				result.Errors = append(result.Errors, ChunkError{
					BlueprintID: blueprintID,
					EntityID:    id,
					Message:     bErr.Message,
				})
			}
		} else {
			result.Created++
			result.SuccessfulKeys = append(result.SuccessfulKeys, fmt.Sprintf("%s:%s", blueprintID, id))
		}
	}

	if len(conflicts) == 0 {
		return result
	}

	retryErrs, retryErr := client.BulkUpsertEntities(ctx, blueprintID, conflicts, true)
	if retryErr != nil {
		for _, e := range conflicts {
			id, _ := e["identifier"].(string)
			result.Errors = append(result.Errors, ChunkError{
				BlueprintID: blueprintID,
				EntityID:    id,
				Message:     retryErr.Error(),
			})
		}
		return result
	}

	retryErrByID := make(map[string]api.BulkEntityError, len(retryErrs))
	for _, re := range retryErrs {
		retryErrByID[re.Identifier] = re
	}
	for _, entity := range conflicts {
		id, _ := entity["identifier"].(string)
		if rErr, failed := retryErrByID[id]; failed {
			result.Errors = append(result.Errors, ChunkError{
				BlueprintID: blueprintID,
				EntityID:    id,
				Message:     rErr.Message,
			})
		} else {
			result.Updated++
			result.SuccessfulKeys = append(result.SuccessfulKeys, fmt.Sprintf("%s:%s", blueprintID, id))
		}
	}
	return result
}

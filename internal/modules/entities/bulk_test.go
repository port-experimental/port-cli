package entities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

type recordingBulkClient struct {
	mu    sync.Mutex
	calls []struct {
		blueprintID string
		count       int
		upsert      bool
	}
	conflictFirst bool
}

func (r *recordingBulkClient) BulkUpsertEntities(_ context.Context, blueprintID string, entities []api.Entity, upsert bool) ([]api.BulkEntityError, error) {
	r.mu.Lock()
	r.calls = append(r.calls, struct {
		blueprintID string
		count       int
		upsert      bool
	}{blueprintID, len(entities), upsert})
	conflict := r.conflictFirst && !upsert
	r.conflictFirst = false
	r.mu.Unlock()

	if conflict && len(entities) > 0 {
		id, _ := entities[0]["identifier"].(string)
		return []api.BulkEntityError{{
			Identifier: id,
			StatusCode: 409,
			Message:    "already exists",
		}}, nil
	}
	return nil, nil
}

func TestProcessChunk_AllSucceed(t *testing.T) {
	client := &recordingBulkClient{}
	chunk := []api.Entity{
		{"identifier": "a", "blueprint": "service"},
		{"identifier": "b", "blueprint": "service"},
	}
	result := ProcessChunk(context.Background(), client, "service", chunk, false)

	if result.Created != 2 || result.Updated != 0 {
		t.Fatalf("expected 2 created, got created=%d updated=%d", result.Created, result.Updated)
	}
	if len(result.SuccessfulKeys) != 2 {
		t.Fatalf("expected 2 successful keys, got %#v", result.SuccessfulKeys)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}

func TestProcessChunk_ConflictRetried(t *testing.T) {
	client := &recordingBulkClient{conflictFirst: true}
	chunk := []api.Entity{
		{"identifier": "a", "blueprint": "service"},
		{"identifier": "b", "blueprint": "service"},
	}
	result := ProcessChunk(context.Background(), client, "service", chunk, false)

	if result.Created != 1 || result.Updated != 1 {
		t.Fatalf("expected 1 created and 1 updated, got created=%d updated=%d", result.Created, result.Updated)
	}
	if len(client.calls) != 2 {
		t.Fatalf("expected 2 bulk calls (original + retry), got %d", len(client.calls))
	}
	if !client.calls[1].upsert {
		t.Fatal("retry call should use upsert=true")
	}
}

func TestProcessChunk_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if strings.Contains(r.URL.Path, "/entities/bulk") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{
					{"identifier": "bad", "statusCode": 400.0, "message": "invalid"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: srv.URL})
	chunk := []api.Entity{
		{"identifier": "good", "blueprint": "service"},
		{"identifier": "bad", "blueprint": "service"},
	}
	result := ProcessChunk(context.Background(), client, "service", chunk, false)

	if result.Created != 1 {
		t.Fatalf("expected 1 created, got %d", result.Created)
	}
	if len(result.Errors) != 1 || result.Errors[0].EntityID != "bad" {
		t.Fatalf("expected error for bad entity, got %#v", result.Errors)
	}
}

func TestBulkUpsert_BatchesThousandsOfEntities(t *testing.T) {
	bulkCalls := 0
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "accessToken": "tok", "expiresIn": 3600})
			return
		}
		if strings.Contains(r.URL.Path, "/entities/bulk") {
			mu.Lock()
			bulkCalls++
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"errors": []interface{}{}})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := api.NewClient(api.ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: srv.URL})
	entities := make([]api.Entity, 1000)
	for i := range entities {
		entities[i] = api.Entity{"identifier": fmt.Sprintf("svc-%d", i), "blueprint": "service"}
	}

	for _, chunk := range ChunkSlice(entities, BatchSize) {
		ProcessChunk(context.Background(), client, "service", chunk, false)
	}

	wantCalls := (1000 + BatchSize - 1) / BatchSize
	if bulkCalls != wantCalls {
		t.Fatalf("expected %d bulk calls for 1000 entities, got %d", wantCalls, bulkCalls)
	}
}

//go:build e2e

package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/port-experimental/port-cli/internal/auth"
)

type teamEntity struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

type entitiesSearchResponse struct {
	Entities []teamEntity `json:"entities"`
	Next     string       `json:"next"`
}

// listTeamIDToName maps _team entity identifier → display name (title).
func listTeamIDToName(ctx context.Context, apiURL string, token *auth.Token) (map[string]string, error) {
	base := strings.TrimSuffix(apiURL, "/")
	client := &http.Client{}
	out := make(map[string]string)
	var from string
	for {
		body := map[string]any{
			"query": map[string]any{"combinator": "and", "rules": []any{}},
			"limit": 200,
		}
		if from != "" {
			body["from"] = from
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/blueprints/_team/entities/search", bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("team search: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		var page entitiesSearchResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, err
		}
		for _, e := range page.Entities {
			name := strings.TrimSpace(e.Title)
			if name == "" {
				name = e.Identifier
			}
			out[e.Identifier] = name
		}
		if page.Next == "" {
			break
		}
		from = page.Next
	}
	return out, nil
}

func resolveTeamNames(teamIDs []string, idToName map[string]string) []string {
	names := make([]string, 0, len(teamIDs))
	for _, id := range teamIDs {
		if name, ok := idToName[id]; ok && name != "" {
			names = append(names, name)
		}
	}
	return names
}

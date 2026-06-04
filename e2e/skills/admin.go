//go:build e2e

package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/port-experimental/port-cli/internal/auth"
)

// AdminClient calls the Port admin-service for member team assignment in E2E.
type AdminClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func newAdminClient(baseURL string) *AdminClient {
	return &AdminClient{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		HTTPClient: &http.Client{},
	}
}

type adminUserResponse struct {
	OK   bool `json:"ok"`
	User struct {
		Teams []struct {
			Name string `json:"name"`
		} `json:"teams"`
	} `json:"user"`
}

func (a *AdminClient) GetUserTeamNames(ctx context.Context, token *auth.Token, orgID, email string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/users/email/%s", a.BaseURL, url.PathEscape(email))
	q := url.Values{}
	q.Set("org_id", orgID)
	q.Set("fields", "teams.name")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get user teams: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed adminUserResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(parsed.User.Teams))
	for _, t := range parsed.User.Teams {
		if n := strings.TrimSpace(t.Name); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

func (a *AdminClient) assignTeams(ctx context.Context, token *auth.Token, method, orgID, email string, teamNames []string) error {
	if len(teamNames) == 0 {
		return nil
	}
	endpoint := fmt.Sprintf("%s/organizations/%s/members/email/%s/teams/name",
		a.BaseURL, url.PathEscape(orgID), url.PathEscape(email))
	body, err := json.Marshal(map[string]any{"teams": teamNames})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s teams: HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (a *AdminClient) AssignTeamsByName(ctx context.Context, token *auth.Token, orgID, email string, teamNames []string) error {
	return a.assignTeams(ctx, token, http.MethodPost, orgID, email, teamNames)
}

func (a *AdminClient) UnassignTeamsByName(ctx context.Context, token *auth.Token, orgID, email string, teamNames []string) error {
	return a.assignTeams(ctx, token, http.MethodDelete, orgID, email, teamNames)
}

// setUserTeams replaces the user's team membership with exactly teamNames.
func (a *AdminClient) setUserTeams(ctx context.Context, token *auth.Token, orgID, email string, teamNames []string) error {
	current, err := a.GetUserTeamNames(ctx, token, orgID, email)
	if err != nil {
		return err
	}
	if len(current) > 0 {
		if err := a.UnassignTeamsByName(ctx, token, orgID, email, current); err != nil {
			return err
		}
	}
	if len(teamNames) == 0 {
		return nil
	}
	return a.AssignTeamsByName(ctx, token, orgID, email, teamNames)
}

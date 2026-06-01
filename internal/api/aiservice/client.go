package aiservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/useragent"
)

// SkillFile matches ai-service skill file entities.
type SkillFile struct {
	Identifier string                 `json:"identifier,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Relations  map[string]interface{} `json:"relations,omitempty"`
}

// SkillAtLatestVersion is one skill at its latest published version.
type SkillAtLatestVersion struct {
	Identifier          string      `json:"identifier"`
	Title               string      `json:"title"`
	Location            string      `json:"location"`
	Description         string      `json:"description,omitempty"`
	Version             string      `json:"version"`
	VersionIdentifier   string      `json:"versionIdentifier"`
	ReleaseState        string      `json:"releaseState"`
	Files               []SkillFile `json:"files"`
}

// SkillGroupAtLatestVersion groups skills with catalog metadata.
type SkillGroupAtLatestVersion struct {
	Identifier string                 `json:"identifier"`
	Title      string                 `json:"title"`
	Required   bool                   `json:"required"`
	AutoSync   bool                   `json:"autoSync"`
	Skills     []SkillAtLatestVersion `json:"skills"`
}

// GroupedSkillsResponse is the GET /v1/skills response body.
type GroupedSkillsResponse struct {
	OK              bool                      `json:"ok"`
	Groups          []SkillGroupAtLatestVersion `json:"groups"`
	UngroupedSkills []SkillAtLatestVersion    `json:"ungroupedSkills"`
}

// Client calls the Port ai-service HTTP API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// ClientOpts configures the ai-service client.
type ClientOpts struct {
	APIURL        string
	AIServiceURL  string
	Timeout       time.Duration
}

// NewClient creates an ai-service client. AIServiceURL overrides DeriveAIServiceURL(APIURL).
func NewClient(opts ClientOpts) *Client {
	base := opts.AIServiceURL
	if base == "" {
		base = DeriveAIServiceURL(opts.APIURL)
	}
	base = strings.TrimSuffix(base, "/")
	return &Client{
		httpClient: &http.Client{Timeout: opts.Timeout},
		baseURL:    base,
	}
}

// DeriveAIServiceURL maps a Port API URL to the ai-service host.
func DeriveAIServiceURL(apiURL string) string {
	if apiURL == "" {
		apiURL = "https://api.getport.io/v1"
	}
	apiURL = strings.TrimSuffix(apiURL, "/")
	u, err := url.Parse(apiURL)
	if err != nil {
		return "https://ai-service.getport.io/v1"
	}
	host := u.Hostname()
	if strings.HasPrefix(host, "api.") {
		host = "ai-service." + strings.TrimPrefix(host, "api.")
	} else if host == "api.getport.io" || host == "" {
		host = "ai-service.getport.io"
	}
	return fmt.Sprintf("%s://%s/v1", u.Scheme, host)
}

// GetSkillsQuery optional filters for GET /v1/skills.
type GetSkillsQuery struct {
	SkillIdentifiers []string
	Limit            int
}

// GetSkillsGrouped fetches published skills grouped by skill group.
func (c *Client) GetSkillsGrouped(ctx context.Context, token *auth.Token, query GetSkillsQuery) (*GroupedSkillsResponse, error) {
	if token == nil {
		return nil, fmt.Errorf("authentication required for ai-service")
	}

	endpoint, err := url.Parse(c.baseURL + "/skills")
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("published_only", "true")
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	authHeader := token.Token
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = "Bearer " + authHeader
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("x-port-user-orgid", token.Claims.OrgId)
	req.Header.Set("x-port-user-userid", token.Claims.UserID)
	if token.Claims.IsMachine {
		req.Header.Set("x-port-user-ismachine", "true")
	}
	req.Header.Set("User-Agent", useragent.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai-service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai-service returned %d: %s", resp.StatusCode, string(body))
	}

	var result GroupedSkillsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ai-service response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("ai-service returned ok=false")
	}
	return &result, nil
}

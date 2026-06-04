package aiservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

// SkillAtLatestVersion is one skill at its active version (sync catalog).
type SkillAtLatestVersion struct {
	Identifier        string      `json:"identifier"`
	Title             string      `json:"title"`
	Location          string      `json:"location"`
	Description       string      `json:"description,omitempty"`
	Version           string      `json:"version"`
	VersionIdentifier string      `json:"versionIdentifier"`
	Files             []SkillFile `json:"files"`
}

// SkillGroupAtLatestVersion groups skills with catalog metadata.
type SkillGroupAtLatestVersion struct {
	Identifier string                 `json:"identifier"`
	Title      string                 `json:"title"`
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

// DeriveAIServiceURL maps a Port API URL to the ai-service base URL.
// For local port-api on :3000, ai-service defaults to :3016. Override with PORT_AI_SERVICE_URL.
func DeriveAIServiceURL(apiURL string) string {
	if apiURL == "" {
		apiURL = "https://api.getport.io/v1"
	}
	apiURL = strings.TrimSuffix(apiURL, "/")
	u, err := url.Parse(apiURL)
	if err != nil {
		return "https://ai-service.getport.io/v1"
	}

	hostname := u.Hostname()
	switch {
	case hostname == "localhost" || hostname == "127.0.0.1":
		return fmt.Sprintf("%s://%s/v1", u.Scheme, net.JoinHostPort(hostname, "3016"))
	case strings.HasPrefix(hostname, "api."):
		aiHostname := "ai-service." + strings.TrimPrefix(hostname, "api.")
		aiHost := aiHostname
		if port := u.Port(); port != "" {
			aiHost = net.JoinHostPort(aiHostname, port)
		}
		return fmt.Sprintf("%s://%s/v1", u.Scheme, aiHost)
	case hostname == "api.getport.io":
		return "https://ai-service.getport.io/v1"
	case hostname == "api.us.getport.io":
		return "https://ai-service.us.getport.io/v1"
	default:
		if u.Host != "" {
			return fmt.Sprintf("%s://%s/v1", u.Scheme, u.Host)
		}
		return "https://ai-service.getport.io/v1"
	}
}

// SkillFileInput is one file in a create/edit skill request.
type SkillFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Title   string `json:"title,omitempty"`
}

// CreateSkillRequest is the POST /v1/skills body.
type CreateSkillRequest struct {
	Identifier  string           `json:"identifier"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Location    string           `json:"location,omitempty"`
	Publish     bool             `json:"publish,omitempty"`
	Files       []SkillFileInput `json:"files"`
}

// EditSkillRequest is the PUT /v1/skills/:identifier body.
type EditSkillRequest struct {
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Location    string           `json:"location,omitempty"`
	Publish     bool             `json:"publish,omitempty"`
	Files       []SkillFileInput `json:"files"`
}

// SkillVersionWriteResponse is returned by create/edit skill endpoints.
type SkillVersionWriteResponse struct {
	OK                bool     `json:"ok"`
	SkillIdentifier   string   `json:"skillIdentifier"`
	Version           string   `json:"version"`
	VersionIdentifier string   `json:"versionIdentifier"`
	ActiveVersionSet  bool     `json:"activeVersionSet"`
	FileIdentifiers   []string `json:"fileIdentifiers"`
}

// BatchCreateSkillsRequest is the POST /v1/skills/batch body.
type BatchCreateSkillsRequest struct {
	Skills []CreateSkillRequest `json:"skills"`
}

// BatchSkillError is a per-item error in batch create.
type BatchSkillError struct {
	Name       string `json:"name"`
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// BatchCreateSkillResultItem is one result in POST /v1/skills/batch.
type BatchCreateSkillResultItem struct {
	Identifier string                      `json:"identifier"`
	OK         bool                        `json:"ok"`
	Result     *SkillVersionWriteResponse  `json:"result,omitempty"`
	Error      *BatchSkillError            `json:"error,omitempty"`
}

// BatchCreateSkillsResponse is returned by POST /v1/skills/batch.
type BatchCreateSkillsResponse struct {
	OK      bool                         `json:"ok"`
	Results []BatchCreateSkillResultItem `json:"results"`
}

// CatalogEntitySnapshot is a Port entity without file payloads.
type CatalogEntitySnapshot struct {
	Identifier string                 `json:"identifier"`
	Title      string                 `json:"title"`
	Blueprint  string                 `json:"blueprint"`
	Properties map[string]interface{} `json:"properties"`
	Relations  map[string]interface{} `json:"relations,omitempty"`
	CreatedAt  *string                `json:"createdAt"`
	UpdatedAt  *string                `json:"updatedAt"`
}

// SkillCatalogEntry pairs a skill entity with its resolved version (if any).
type SkillCatalogEntry struct {
	Skill   CatalogEntitySnapshot  `json:"skill"`
	Version *CatalogEntitySnapshot `json:"version"`
}

// SkillsSummaryResponse is the GET /v1/skills/summary response body.
type SkillsSummaryResponse struct {
	OK     bool                `json:"ok"`
	Skills []SkillCatalogEntry `json:"skills"`
}

// SkillGroupCatalogEntry is one row from GET /v1/skills/groups.
type SkillGroupCatalogEntry struct {
	Identifier       string   `json:"identifier"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	OwningTeamIDs    []string `json:"owningTeamIds"`
	MatchesUserTeams bool     `json:"matchesUserTeams"`
}

// SkillGroupsResponse is the GET /v1/skills/groups response body.
type SkillGroupsResponse struct {
	OK     bool                     `json:"ok"`
	Groups []SkillGroupCatalogEntry `json:"groups"`
}

// GetSkillsQuery optional filters for GET /v1/skills.
type GetSkillsQuery struct {
	SkillIdentifiers []string
	IncludeGroups    []string
	ExcludeGroups    []string
	TeamsDefault     bool
	Limit            int
}

// GetSkillsSummaryQuery optional filters for GET /v1/skills/summary.
type GetSkillsSummaryQuery struct {
	SkillIdentifiers []string
	Limit            int
	PublishedOnly    bool
}

// SearchSkillsQuery optional filters for GET /v1/skills/search.
type SearchSkillsQuery struct {
	Query         string
	Limit         int
	PublishedOnly bool
}

// GetSkillsGrouped fetches published skills grouped by skill group.
func (c *Client) GetSkillsGrouped(ctx context.Context, token *auth.Token, query GetSkillsQuery) (*GroupedSkillsResponse, error) {
	q := url.Values{}
	q.Set("published_only", "true")
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
	}
	for _, id := range query.IncludeGroups {
		q.Add("include_group", id)
	}
	for _, id := range query.ExcludeGroups {
		q.Add("exclude_group", id)
	}
	if query.TeamsDefault {
		q.Set("teams_default", "true")
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	var result GroupedSkillsResponse
	if err := c.getJSON(ctx, token, "/skills", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSkillGroups lists all skill groups for init selection (GET /v1/skills/groups).
func (c *Client) GetSkillGroups(ctx context.Context, token *auth.Token) (*SkillGroupsResponse, error) {
	var result SkillGroupsResponse
	if err := c.getJSON(ctx, token, "/skills/groups", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSkillsSummary lists skill entities (metadata only, no file content).
func (c *Client) GetSkillsSummary(ctx context.Context, token *auth.Token, query GetSkillsSummaryQuery) (*SkillsSummaryResponse, error) {
	q := url.Values{}
	if query.PublishedOnly {
		q.Set("published_only", "true")
	} else {
		q.Set("published_only", "false")
	}
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	var result SkillsSummaryResponse
	if err := c.getJSON(ctx, token, "/skills/summary", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchSkills finds skills whose identifier or title matches the query (GET /v1/skills/search).
func (c *Client) SearchSkills(ctx context.Context, token *auth.Token, query SearchSkillsQuery) (*SkillsSummaryResponse, error) {
	q := url.Values{}
	q.Set("q", query.Query)
	if query.PublishedOnly {
		q.Set("published_only", "true")
	} else {
		q.Set("published_only", "false")
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	var result SkillsSummaryResponse
	if err := c.getJSON(ctx, token, "/skills/search", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateSkill creates a skill with an initial version via POST /v1/skills.
func (c *Client) CreateSkill(ctx context.Context, token *auth.Token, body CreateSkillRequest) (*SkillVersionWriteResponse, error) {
	var result SkillVersionWriteResponse
	if err := c.doJSON(ctx, token, http.MethodPost, "/skills", nil, body, http.StatusCreated, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// EditSkill creates a new version via PUT /v1/skills/:identifier.
func (c *Client) EditSkill(ctx context.Context, token *auth.Token, identifier string, body EditSkillRequest) (*SkillVersionWriteResponse, error) {
	var result SkillVersionWriteResponse
	path := "/skills/" + url.PathEscape(identifier)
	if err := c.doJSON(ctx, token, http.MethodPut, path, nil, body, http.StatusOK, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateSkillsBatch creates multiple skills via POST /v1/skills/batch.
func (c *Client) CreateSkillsBatch(ctx context.Context, token *auth.Token, body BatchCreateSkillsRequest) (*BatchCreateSkillsResponse, error) {
	var result BatchCreateSkillsResponse
	if err := c.doJSON(ctx, token, http.MethodPost, "/skills/batch", nil, body, http.StatusOK, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) getJSON(ctx context.Context, token *auth.Token, path string, query url.Values, dest any) error {
	return c.doJSON(ctx, token, http.MethodGet, path, query, nil, http.StatusOK, dest)
}

func (c *Client) doJSON(
	ctx context.Context,
	token *auth.Token,
	method, path string,
	query url.Values,
	body any,
	expectStatus int,
	dest any,
) error {
	if token == nil {
		return fmt.Errorf("authentication required for ai-service")
	}

	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	authHeader := token.Token
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = "Bearer " + authHeader
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("x-port-user-orgid", token.Claims.OrgId)
	req.Header.Set("x-port-user-userid", token.Claims.UserID)
	if token.Claims.Email != "" {
		req.Header.Set("x-port-user-email", token.Claims.Email)
	}
	if token.Claims.IsMachine {
		req.Header.Set("x-port-user-ismachine", "true")
	}
	req.Header.Set("User-Agent", useragent.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ai-service request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != expectStatus {
		return fmt.Errorf("ai-service returned %d: %s", resp.StatusCode, string(respBody))
	}
	var envelope struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("failed to decode ai-service response: %w", err)
	}
	if !envelope.OK {
		return fmt.Errorf("ai-service returned ok=false")
	}
	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("failed to decode ai-service response: %w", err)
		}
	}
	return nil
}

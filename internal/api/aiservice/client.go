package aiservice

import (
	"bytes"
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
	Published   bool             `json:"published,omitempty"`
	Files       []SkillFileInput `json:"files"`
}

// EditSkillRequest is the PUT /v1/skills/:identifier body.
type EditSkillRequest struct {
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Location    string           `json:"location,omitempty"`
	Published   bool             `json:"published,omitempty"`
	Files       []SkillFileInput `json:"files"`
}

// SkillVersionWriteResponse is returned by create/edit skill endpoints.
type SkillVersionWriteResponse struct {
	OK                bool     `json:"ok"`
	SkillIdentifier   string   `json:"skillIdentifier"`
	Version           string   `json:"version"`
	VersionIdentifier string   `json:"versionIdentifier"`
	ReleaseState      string   `json:"releaseState"`
	FileIdentifiers   []string `json:"fileIdentifiers"`
}

// ArchiveSkillResponse is returned by POST /v1/skills/:identifier/archive.
type ArchiveSkillResponse struct {
	OK               bool   `json:"ok"`
	SkillIdentifier  string `json:"skillIdentifier"`
	VersionsArchived int    `json:"versionsArchived"`
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

// GetSkillsQuery optional filters for GET /v1/skills.
type GetSkillsQuery struct {
	SkillIdentifiers []string
	Limit            int
}

// GetSkillsSummaryQuery optional filters for GET /v1/skills/summary.
type GetSkillsSummaryQuery struct {
	SkillIdentifiers []string
	Limit            int
	PublishedOnly    bool
}

// GetSkillsGrouped fetches published skills grouped by skill group.
func (c *Client) GetSkillsGrouped(ctx context.Context, token *auth.Token, query GetSkillsQuery) (*GroupedSkillsResponse, error) {
	q := url.Values{}
	q.Set("published_only", "true")
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
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

// ArchiveSkill archives all versions via POST /v1/skills/:identifier/archive.
func (c *Client) ArchiveSkill(ctx context.Context, token *auth.Token, identifier string) (*ArchiveSkillResponse, error) {
	var result ArchiveSkillResponse
	path := "/skills/" + url.PathEscape(identifier) + "/archive"
	if err := c.doJSON(ctx, token, http.MethodPost, path, nil, nil, http.StatusOK, &result); err != nil {
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

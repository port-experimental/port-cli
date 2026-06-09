package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

// SkillFile is one file in a skill directory tree.
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
	GroupIdentifiers  []string    `json:"groupIdentifiers,omitempty"`
	Files             []SkillFile `json:"files"`
}

// SkillGroupAtLatestVersion groups skills with catalog metadata.
type SkillGroupAtLatestVersion struct {
	Identifier string                 `json:"identifier"`
	Title      string                 `json:"title"`
	Skills     []SkillAtLatestVersion `json:"skills"`
}

// GroupedSkillsResponse is the GET /skills response body.
type GroupedSkillsResponse struct {
	OK              bool                        `json:"ok"`
	Groups          []SkillGroupAtLatestVersion `json:"groups"`
	UngroupedSkills []SkillAtLatestVersion      `json:"ungroupedSkills"`
}

// SkillFileInput is one file in a create/edit skill request.
type SkillFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Title   string `json:"title,omitempty"`
}

// VersionBump is the semver increment for a new skill version.
type VersionBump string

const (
	VersionBumpPatch VersionBump = "patch"
	VersionBumpMinor VersionBump = "minor"
	VersionBumpMajor VersionBump = "major"
)

// UploadSkillRequest is the POST /skills/upload body.
type UploadSkillRequest struct {
	Identifier     string           `json:"identifier"`
	Title          string           `json:"title,omitempty"`
	Description    string           `json:"description,omitempty"`
	Location       string           `json:"location,omitempty"`
	Publish        bool             `json:"publish,omitempty"`
	VersionBump    VersionBump      `json:"versionBump,omitempty"`
	GroupIDs       []string         `json:"groupIdentifiers,omitempty"`
	FolderBaseName string           `json:"folderBaseName,omitempty"`
	Files          []SkillFileInput `json:"files"`
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

// BatchUploadSkillsRequest is the POST /skills/upload/batch body.
type BatchUploadSkillsRequest struct {
	Skills []UploadSkillRequest `json:"skills"`
}

// BatchSkillError is a per-item error in batch create.
type BatchSkillError struct {
	Name       string `json:"name"`
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// BatchUploadSkillResultItem is one result in POST /skills/upload/batch.
type BatchUploadSkillResultItem struct {
	Identifier string                     `json:"identifier"`
	OK         bool                       `json:"ok"`
	Result     *SkillVersionWriteResponse `json:"result,omitempty"`
	Error      *BatchSkillError           `json:"error,omitempty"`
}

// BatchUploadSkillsResponse is returned by POST /skills/upload/batch.
type BatchUploadSkillsResponse struct {
	OK      bool                         `json:"ok"`
	Results []BatchUploadSkillResultItem `json:"results"`
}

// GetSkillResponse is returned by GET /skills/:identifier.
type GetSkillResponse struct {
	OK    bool                 `json:"ok"`
	Skill SkillAtLatestVersion `json:"skill"`
}

// UnpublishSkillResponse is returned by POST /skills/:identifier/unpublish.
type UnpublishSkillResponse struct {
	OK              bool   `json:"ok"`
	SkillIdentifier string `json:"skillIdentifier"`
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

// SkillsSummaryResponse is the GET /skills/summary response body.
type SkillsSummaryResponse struct {
	OK     bool                `json:"ok"`
	Skills []SkillCatalogEntry `json:"skills"`
}

// SkillGroupCatalogEntry is one row from GET /skills/groups.
type SkillGroupCatalogEntry struct {
	Identifier       string   `json:"identifier"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	OwningTeamIDs    []string `json:"owningTeamIds"`
	MatchesUserTeams bool     `json:"matchesUserTeams"`
}

// SkillGroupsResponse is the GET /skills/groups response body.
type SkillGroupsResponse struct {
	OK     bool                     `json:"ok"`
	Groups []SkillGroupCatalogEntry `json:"groups"`
}

// GetSkillsQuery optional filters for GET /skills.
type GetSkillsQuery struct {
	SkillIdentifiers []string
	IncludeGroups    []string
	ExcludeGroups    []string
	TeamsDefault     *bool
	Limit            int
	Exclude          []string
}

// GetSkillsSummaryQuery optional filters for GET /skills/summary.
type GetSkillsSummaryQuery struct {
	SkillIdentifiers   []string
	Limit              int
	IncludeUnpublished bool
}

// SearchSkillsQuery optional filters for GET /skills/search.
type SearchSkillsQuery struct {
	Query string
	Limit int
}

func buildGetSkillsQuery(query GetSkillsQuery) url.Values {
	q := url.Values{}
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
	}
	for _, id := range query.IncludeGroups {
		q.Add("include_group", id)
	}
	for _, id := range query.ExcludeGroups {
		q.Add("exclude_group", id)
	}
	if query.TeamsDefault != nil {
		q.Set("teams_default", fmt.Sprintf("%t", *query.TeamsDefault))
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	for _, part := range query.Exclude {
		q.Add("exclude", part)
	}
	return q
}

// GetSkillsGrouped fetches published skills grouped by skill group.
func (c *Client) GetSkillsGrouped(ctx context.Context, query GetSkillsQuery) (*GroupedSkillsResponse, error) {
	var result GroupedSkillsResponse
	if err := c.skillsGET(ctx, "/skills", buildGetSkillsQuery(query), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSkillGroups lists all skill groups for init selection.
func (c *Client) GetSkillGroups(ctx context.Context) (*SkillGroupsResponse, error) {
	var result SkillGroupsResponse
	if err := c.skillsGET(ctx, "/skills/groups", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSkillsSummary lists skill entities (metadata only, no file content).
func (c *Client) GetSkillsSummary(ctx context.Context, query GetSkillsSummaryQuery) (*SkillsSummaryResponse, error) {
	q := url.Values{}
	for _, id := range query.SkillIdentifiers {
		q.Add("skill_identifier", id)
	}
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	if query.IncludeUnpublished {
		q.Set("include_unpublished", "true")
	}
	var result SkillsSummaryResponse
	if err := c.skillsGET(ctx, "/skills/summary", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchSkills finds skills whose identifier or title matches the query.
func (c *Client) SearchSkills(ctx context.Context, query SearchSkillsQuery) (*SkillsSummaryResponse, error) {
	q := url.Values{}
	q.Set("q", query.Query)
	if query.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	var result SkillsSummaryResponse
	if err := c.skillsGET(ctx, "/skills/search", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadSkill creates or updates a skill via POST /skills/upload.
func (c *Client) UploadSkill(ctx context.Context, body UploadSkillRequest) (*SkillVersionWriteResponse, error) {
	pr, _, contentType, err := buildSingleSkillMultipart(body)
	if err != nil {
		return nil, err
	}
	var result SkillVersionWriteResponse
	if err := c.skillsMultipart(ctx, "/skills/upload", contentType, pr, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadSkillsBatch uploads multiple skills via POST /skills/upload/batch.
func (c *Client) UploadSkillsBatch(ctx context.Context, body BatchUploadSkillsRequest) (*BatchUploadSkillsResponse, error) {
	pr, _, contentType, err := buildBatchSkillMultipart(body)
	if err != nil {
		return nil, err
	}
	var result BatchUploadSkillsResponse
	if err := c.skillsMultipart(ctx, "/skills/upload/batch", contentType, pr, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSkill fetches one published skill.
func (c *Client) GetSkill(ctx context.Context, identifier string) (*GetSkillResponse, error) {
	var result GetSkillResponse
	path := "/skills/" + url.PathEscape(identifier)
	if err := c.skillsGET(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PublishSkill sets the active version to the latest semver.
func (c *Client) PublishSkill(ctx context.Context, identifier string) (*SkillVersionWriteResponse, error) {
	var result SkillVersionWriteResponse
	path := "/skills/" + url.PathEscape(identifier) + "/publish"
	if err := c.skillsPOST(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UnpublishSkill clears the active version.
func (c *Client) UnpublishSkill(ctx context.Context, identifier string) (*UnpublishSkillResponse, error) {
	var result UnpublishSkillResponse
	path := "/skills/" + url.PathEscape(identifier) + "/unpublish"
	if err := c.skillsPOST(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) skillsGET(ctx context.Context, path string, query url.Values, dest any) error {
	return c.skillsRequest(ctx, http.MethodGet, path, query, "", nil, dest)
}

func (c *Client) skillsPOST(ctx context.Context, path string, dest any) error {
	return c.skillsRequest(ctx, http.MethodPost, path, nil, "", nil, dest)
}

func (c *Client) skillsMultipart(ctx context.Context, path, contentType string, body io.Reader, dest any) error {
	return c.skillsRequest(ctx, http.MethodPost, path, nil, contentType, body, dest)
}

func (c *Client) skillsRequest(
	ctx context.Context,
	method, path string,
	query url.Values,
	contentType string,
	body io.Reader,
	dest any,
) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	endpoint := c.apiURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to create skills request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("skills API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("skills API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var envelope struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("failed to decode skills API response: %w", err)
	}
	if !envelope.OK {
		return fmt.Errorf("skills API returned ok=false")
	}
	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("failed to decode skills API response: %w", err)
		}
	}
	return nil
}

func buildSingleSkillMultipart(body UploadSkillRequest) (*io.PipeReader, *io.PipeWriter, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var writeErr error
		defer func() {
			mw.Close()
			pw.CloseWithError(writeErr)
		}()

		meta := struct {
			Identifier     string      `json:"identifier"`
			Title          string      `json:"title,omitempty"`
			Description    string      `json:"description,omitempty"`
			Location       string      `json:"location,omitempty"`
			Publish        bool        `json:"publish,omitempty"`
			VersionBump    VersionBump `json:"versionBump,omitempty"`
			GroupIDs       []string    `json:"groupIdentifiers,omitempty"`
			FolderBaseName string      `json:"folderBaseName,omitempty"`
		}{
			Identifier:     body.Identifier,
			Title:          body.Title,
			Description:    body.Description,
			Location:       body.Location,
			Publish:        body.Publish,
			VersionBump:    body.VersionBump,
			GroupIDs:       body.GroupIDs,
			FolderBaseName: body.FolderBaseName,
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			writeErr = err
			return
		}
		fw, err := mw.CreateFormField("metadata")
		if err != nil {
			writeErr = err
			return
		}
		if _, err = fw.Write(metaJSON); err != nil {
			writeErr = err
			return
		}

		for _, f := range body.Files {
			fw, err := mw.CreateFormFile("file", f.Path)
			if err != nil {
				writeErr = err
				return
			}
			if _, err = fw.Write([]byte(f.Content)); err != nil {
				writeErr = err
				return
			}
		}
	}()
	return pr, pw, mw.FormDataContentType(), nil
}

func buildBatchSkillMultipart(body BatchUploadSkillsRequest) (*io.PipeReader, *io.PipeWriter, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var writeErr error
		defer func() {
			mw.Close()
			pw.CloseWithError(writeErr)
		}()

		for i, skill := range body.Skills {
			meta := struct {
				Identifier     string      `json:"identifier"`
				Title          string      `json:"title,omitempty"`
				Description    string      `json:"description,omitempty"`
				Location       string      `json:"location,omitempty"`
				Publish        bool        `json:"publish,omitempty"`
				VersionBump    VersionBump `json:"versionBump,omitempty"`
				GroupIDs       []string    `json:"groupIdentifiers,omitempty"`
				FolderBaseName string      `json:"folderBaseName,omitempty"`
			}{
				Identifier:     skill.Identifier,
				Title:          skill.Title,
				Description:    skill.Description,
				Location:       skill.Location,
				Publish:        skill.Publish,
				VersionBump:    skill.VersionBump,
				GroupIDs:       skill.GroupIDs,
				FolderBaseName: skill.FolderBaseName,
			}
			metaJSON, err := json.Marshal(meta)
			if err != nil {
				writeErr = err
				return
			}
			fw, err := mw.CreateFormField(fmt.Sprintf("skills[%d].metadata", i))
			if err != nil {
				writeErr = err
				return
			}
			if _, err = fw.Write(metaJSON); err != nil {
				writeErr = err
				return
			}

			for _, f := range skill.Files {
				fw, err := mw.CreateFormFile(fmt.Sprintf("skills[%d].file", i), f.Path)
				if err != nil {
					writeErr = err
					return
				}
				if _, err = fw.Write([]byte(f.Content)); err != nil {
					writeErr = err
					return
				}
			}
		}
	}()
	return pr, pw, mw.FormDataContentType(), nil
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxRetries      = 3
	baseRetryDelay  = 100 * time.Millisecond
	maxRetryDelay   = 5 * time.Second
	retryableStatus = 429 // Too Many Requests
)

// Client handles authenticated requests to Port's API.
type Client struct {
	httpClient *http.Client
	tokenMgr   *TokenManager
	apiURL     string
	timeout    time.Duration
}

// TokenResponse represents the Port API token response.
type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	TokenType   string `json:"tokenType"`
}

// NewClient creates a new Port API client.
func NewClient(clientID, clientSecret, apiURL string, timeout time.Duration) *Client {
	if apiURL == "" {
		apiURL = "https://api.getport.io/v1"
	}
	if timeout == 0 {
		timeout = 300 * time.Second
	}

	// Remove trailing slash
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		tokenMgr: NewTokenManager(clientID, clientSecret, apiURL),
		apiURL:   apiURL,
		timeout:  timeout,
	}
}

// getToken gets or refreshes the authentication token.
func (c *Client) getToken(ctx context.Context) (string, error) {
	token, err := c.tokenMgr.GetToken()
	if err == nil && token != "" {
		return token, nil
	}

	// Refresh token
	return c.refreshToken(ctx)
}

// refreshToken requests a new token from the API.
func (c *Client) refreshToken(ctx context.Context) (string, error) {
	authURL := fmt.Sprintf("%s/auth/access_token", c.apiURL)
	payload := map[string]string{
		"clientId":     c.tokenMgr.ClientID,
		"clientSecret": c.tokenMgr.ClientSecret,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// Cache the token
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	c.tokenMgr.SetToken(tokenResp.AccessToken, expiry)

	return tokenResp.AccessToken, nil
}

// request makes an authenticated request to the Port API.
func (c *Client) request(ctx context.Context, method, path string, data interface{}, params map[string]string) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", c.apiURL, path)

	var reqBody io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters
	if params != nil {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	var resp *http.Response

	// Retry logic with exponential backoff
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := baseRetryDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries+1, err)
			}
			// Retry on network errors
			continue
		}

		// Check if status code is retryable (429 Too Many Requests)
		if resp.StatusCode == retryableStatus && attempt < maxRetries {
			resp.Body.Close()
			// Retry on rate limit
			continue
		}

		// Non-retryable status codes
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed: %s - %s", resp.Status, string(body))
		}

		// Success
		return resp, nil
	}

	return resp, err
}

// Close closes the HTTP client (no-op for standard client, but implements closer pattern).
func (c *Client) Close() error {
	return nil
}

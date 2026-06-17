package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/port-experimental/port-cli/internal/useragent"
)

// SSEEvent represents a single parsed Server-Sent Event from the Port Agent API.
//
// Data holds the raw data field value (plain text or JSON string) — populated by
// ParseSSEBlock and InvokeAgent (multi-field SSE format used by the real Port API).
//
// Payload is the legacy structured payload field populated by ParseSSELine (single
// data-line JSON format). It exists so that existing ParseSSELine tests keep passing.
type SSEEvent struct {
	Type    string          `json:"type"`
	Data    string          // raw data field — plain text or JSON string
	Payload json.RawMessage `json:"payload,omitempty"` // legacy: used by ParseSSELine only
}

// ParseSSELine parses a single SSE line (e.g. "data: {...}") into an SSEEvent.
// Returns (nil, nil) for non-data lines (blank lines, comments, event/id fields).
func ParseSSELine(line string) (*SSEEvent, error) {
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}
	data := strings.TrimPrefix(line, "data: ")
	data = strings.TrimSpace(data)
	if data == "" || data == "[DONE]" {
		return nil, nil
	}
	var event SSEEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// Non-JSON payload — skip gracefully
		return nil, nil
	}
	return &event, nil
}

// ParseSSEBlock parses a slice of non-empty SSE lines (one event block between blank-line separators)
// into an SSEEvent. Returns nil for empty input or blocks with no data lines.
// Lines starting with ':' (comments), 'id:', or 'retry:' are silently ignored.
func ParseSSEBlock(lines []string) *SSEEvent {
	var eventType string
	var dataLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimPrefix(line, "event:")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		}
		// ignore :comments, id:, retry:
	}
	if eventType == "" || len(dataLines) == 0 {
		return nil
	}
	return &SSEEvent{
		Type: eventType,
		Data: strings.Join(dataLines, "\n"),
	}
}

// requestStream makes an authenticated POST request and returns the raw *http.Response
// for SSE reading. The caller MUST close resp.Body when done.
//
// A dedicated http.Client with Timeout=0 is used so the stream is not cut by
// the default 300 s client timeout. Lifecycle is controlled via ctx.
func (c *Client) requestStream(ctx context.Context, method, path string, data any) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s%s", c.apiURL, path)

	var reqBody io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stream request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", useragent.String())

	// Dedicated client: no timeout (ctx controls lifecycle), no compression
	// (gzip-encoded SSE is not progressively decodable).
	streamClient := &http.Client{
		Transport: &http.Transport{
			DisableCompression:    true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		// Try to extract just the error message from the JSON response body.
		var apiErr struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil {
			if apiErr.Message != "" {
				return nil, fmt.Errorf("agent invoke failed (%s): %s", resp.Status, apiErr.Message)
			}
			if apiErr.Error != "" {
				return nil, fmt.Errorf("agent invoke failed (%s): %s", resp.Status, apiErr.Error)
			}
		}
		return nil, fmt.Errorf("agent invoke failed (%s)", resp.Status)
	}

	return resp, nil
}

// InvokeAgent invokes a Port AI Agent and sends parsed SSE events to the provided
// channel. The channel is closed when the stream ends or ctx is cancelled.
// Returns nil on clean completion, ctx.Err() on cancellation, or a stream error.
func (c *Client) InvokeAgent(ctx context.Context, agentID, prompt string, events chan<- SSEEvent) error {
	body := map[string]string{"prompt": prompt}
	resp, err := c.requestStream(ctx, "POST", fmt.Sprintf("/agent/%s/invoke", url.PathEscape(agentID)), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	const maxSSELineBytes = 1 * 1024 * 1024
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineBytes)

	var blockLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// End of SSE block — parse and emit
			if event := ParseSSEBlock(blockLines); event != nil {
				select {
				case events <- *event:
				case <-ctx.Done():
					return ctx.Err()
				}
				if event.Type == "done" {
					return nil
				}
			}
			blockLines = blockLines[:0]
			continue
		}
		blockLines = append(blockLines, line)
	}

	// Flush last block if server closed without trailing blank line.
	if event := ParseSSEBlock(blockLines); event != nil {
		select {
		case events <- *event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("SSE stream read error: %w", err)
	}

	return nil
}

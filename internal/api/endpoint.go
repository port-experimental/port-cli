package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// DoJSON executes an authenticated API request and decodes the JSON response into out.
// When out is nil the response body is discarded after a successful request.
func (c *Client) DoJSON(ctx context.Context, method, path string, body any, params map[string]string, out any) error {
	resp, err := c.request(ctx, method, path, body, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// decodeEnvelope decodes a Port API response envelope keyed by envelopeKey.
func decodeEnvelope[T any](raw map[string]json.RawMessage, envelopeKey, decodeFailMsg string) (T, error) {
	var zero T
	payload, ok := raw[envelopeKey]
	if !ok {
		return zero, nil
	}
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return zero, fmt.Errorf("%s: %w", decodeFailMsg, err)
	}
	return value, nil
}

func doEnvelope[T any](c *Client, ctx context.Context, method, path string, body any, params map[string]string, envelopeKey, decodeFailMsg string) (T, error) {
	var zero T
	var wrapper map[string]json.RawMessage
	if err := c.DoJSON(ctx, method, path, body, params, &wrapper); err != nil {
		return zero, err
	}
	return decodeEnvelope[T](wrapper, envelopeKey, decodeFailMsg)
}

func (c *Client) doNoContent(ctx context.Context, method, path string, body any, params map[string]string) error {
	return c.DoJSON(ctx, method, path, body, params, nil)
}

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/port-experimental/port-cli/internal/auth"
)

type orgsCreds map[string]auth.Token

func (cm *ConfigManager) StoreToken(org string, token *auth.Token) error {
	orgsContent := orgsCreds{}
	path := cm.credsPath()
	if f, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(f, &orgsContent); err != nil {
			return err
		}
	} else {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create creds directory (%w)", err)
		}
	}

	orgsContent[org] = *token

	content, err := json.Marshal(orgsContent)
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0o600)
}

func (cm *ConfigManager) GetToken(org string) (*auth.Token, error) {
	orgsCreds := orgsCreds{}
	path := cm.credsPath()
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(f, &orgsCreds); err != nil {
		return nil, err
	}

	token, ok := orgsCreds[org]

	if !ok {
		return nil, fmt.Errorf("org %s not found", org)
	}

	return &token, nil
}

// GetOrRefreshToken returns the stored token for the org, silently refreshing it
// when it has expired and refresh metadata is available.
func (cm *ConfigManager) GetOrRefreshToken(ctx context.Context, org string) (*auth.Token, error) {
	token, err := cm.GetToken(org)
	if err != nil {
		// Missing cached OAuth token is not an error for commands that can
		// fall back to client_id/client_secret authentication.
		return nil, nil
	}

	if time.Now().Before(token.Claims.Expiry.Add(-5 * time.Minute)) {
		return token, nil
	}

	if token.RefreshToken == "" || token.AuthBaseURL == "" {
		return token, nil
	}

	refreshed, err := auth.RefreshAccessToken(ctx, token.AuthBaseURL, token.RefreshToken)
	if err != nil {
		// Best-effort refresh. Keep the existing token so the caller may still
		// authenticate via client_id/client_secret fallback if configured.
		return token, nil
	}

	if err := cm.StoreToken(org, refreshed); err != nil {
		return nil, err
	}

	return refreshed, nil
}

func (cm *ConfigManager) DeleteToken(org string) error {
	orgsContent := orgsCreds{}
	path := cm.credsPath()
	f, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(f, &orgsContent); err != nil {
		return err
	}

	delete(orgsContent, org)
	return cm.saveToFile(path, orgsContent)
}

func (cm *ConfigManager) saveToFile(path string, content orgsCreds) error {
	data, err := json.Marshal(content)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func (cm *ConfigManager) credsPath() string {
	dir := filepath.Dir(cm.configPath)
	return filepath.Join(dir, "creds.json")
}

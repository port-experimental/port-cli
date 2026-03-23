package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		os.MkdirAll(dir, 0o700)
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

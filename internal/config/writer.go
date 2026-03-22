package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-jwt/jwt/v5"
)

func (cm *ConfigManager) StoreToken(org string, token *jwt.Token) error {
	orgsContent := map[string]any{}
	dir := filepath.Dir(cm.configPath)
	path := filepath.Join(dir, "creds.json")
	if f, err := os.ReadFile(path); err == nil {
		json.Unmarshal(f, &orgsContent)
	} else {
		os.MkdirAll(dir, 0o644)
	}

	orgsContent[org] = token
	content, err := json.Marshal(orgsContent)
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0o644)
}

func (cm *ConfigManager) GetToken(org string) (*jwt.Token, error) {
	orgsContent := map[string]struct{ Raw string }{}
	dir := filepath.Dir(cm.configPath)
	path := filepath.Join(dir, "creds.json")
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(f, &orgsContent); err != nil {
		return nil, err
	}

	token, ok := orgsContent[org]
	if !ok {
		return nil, fmt.Errorf("org %s not found", org)
	}

	parsed, err := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name})).
		Parse(token.Raw, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(""), nil
		})
	return parsed, nil
}

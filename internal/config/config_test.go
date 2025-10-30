package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_Load(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a test config file
	configContent := `default_org: test
organizations:
  test:
    client_id: test-client-id
    client_secret: test-client-secret
    api_url: https://api.getport.io/v1
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DefaultOrg != "test" {
		t.Errorf("Expected default_org 'test', got '%s'", cfg.DefaultOrg)
	}

	if len(cfg.Organizations) != 1 {
		t.Errorf("Expected 1 organization, got %d", len(cfg.Organizations))
	}

	orgConfig, ok := cfg.Organizations["test"]
	if !ok {
		t.Fatal("Organization 'test' not found")
	}

	if orgConfig.ClientID != "test-client-id" {
		t.Errorf("Expected client_id 'test-client-id', got '%s'", orgConfig.ClientID)
	}
}

func TestConfigManager_LoadWithOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `default_org: default
organizations:
  default:
    client_id: default-client-id
    client_secret: default-client-secret
    api_url: https://api.getport.io/v1
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager(configPath)
	cfg, err := manager.LoadWithOverrides("override-client-id", "override-client-secret", "", "override")
	if err != nil {
		t.Fatalf("Failed to load config with overrides: %v", err)
	}

	orgConfig, err := cfg.GetOrgConfig("override")
	if err != nil {
		t.Fatalf("Failed to get org config: %v", err)
	}

	if orgConfig.ClientID != "override-client-id" {
		t.Errorf("Expected client_id 'override-client-id', got '%s'", orgConfig.ClientID)
	}
}

func TestConfig_GetOrgConfig(t *testing.T) {
	cfg := &Config{
		DefaultOrg: "default",
		Organizations: map[string]OrganizationConfig{
			"default": {
				ClientID:     "test-id",
				ClientSecret: "test-secret",
				APIURL:       "https://api.getport.io/v1",
			},
		},
	}

	orgConfig, err := cfg.GetOrgConfig("")
	if err != nil {
		t.Fatalf("Failed to get default org config: %v", err)
	}

	if orgConfig.ClientID != "test-id" {
		t.Errorf("Expected client_id 'test-id', got '%s'", orgConfig.ClientID)
	}

	// Test error case
	_, err = cfg.GetOrgConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent organization")
	}
}

func TestConfigManager_CreateDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	manager := NewConfigManager(configPath)
	err := manager.CreateDefaultConfig()
	if err != nil {
		t.Fatalf("Failed to create default config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify we can load it
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Failed to load created config: %v", err)
	}

	if cfg.DefaultOrg != "production" {
		t.Errorf("Expected default_org 'production', got '%s'", cfg.DefaultOrg)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Organizations: map[string]OrganizationConfig{
					"test": {
						ClientID:     "test-id",
						ClientSecret: "test-secret",
						APIURL:       "https://api.getport.io/v1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no organizations",
			config: &Config{
				Organizations: map[string]OrganizationConfig{},
			},
			wantErr: true,
		},
		{
			name: "missing client_id",
			config: &Config{
				Organizations: map[string]OrganizationConfig{
					"test": {
						ClientSecret: "test-secret",
						APIURL:       "https://api.getport.io/v1",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


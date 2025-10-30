package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// OrganizationConfig represents configuration for a Port organization.
type OrganizationConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	APIURL       string `yaml:"api_url"`
}

// BackendConfig represents configuration for the backend server (legacy, may not be used).
type BackendConfig struct {
	URL     string `yaml:"url"`
	Timeout int    `yaml:"timeout"`
}

// Config represents the main configuration structure.
type Config struct {
	DefaultOrg   string                       `yaml:"default_org"`
	Organizations map[string]OrganizationConfig `yaml:"organizations"`
	Backend      BackendConfig                `yaml:"backend"`
}

// DefaultConfigPath returns the default path to the configuration file.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".port/config.yaml"
	}
	return filepath.Join(home, ".port", "config.yaml")
}

// GetOrgConfig returns the configuration for a specific organization.
func (c *Config) GetOrgConfig(orgName string) (*OrganizationConfig, error) {
	// Use default org if no name specified
	if orgName == "" {
		orgName = c.DefaultOrg
	}

	// If still no org name, try to use the first available org
	if orgName == "" {
		if len(c.Organizations) == 0 {
			return nil, fmt.Errorf(`missing authentication credentials

To authenticate, use one of the following methods:

1. CLI flags (recommended for standalone binaries):
   port export --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

2. Environment variables:
   export PORT_CLIENT_ID="your-client-id"
   export PORT_CLIENT_SECRET="your-client-secret"

3. Configuration file:
   Run: port config --init
   Then edit: %s`, DefaultConfigPath())
		}

		// Use first organization
		for name := range c.Organizations {
			orgName = name
			break
		}
	}

	org, exists := c.Organizations[orgName]
	if !exists {
		orgNames := make([]string, 0, len(c.Organizations))
		for name := range c.Organizations {
			orgNames = append(orgNames, name)
		}
		return nil, fmt.Errorf("organization '%s' not found in configuration. Available organizations: %v", orgName, orgNames)
	}

	if org.ClientID == "" || org.ClientSecret == "" {
		return nil, fmt.Errorf(`missing credentials for organization '%s'

To fix this, use one of the following methods:

1. CLI flags (recommended for standalone binaries):
   port export --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

2. Environment variables:
   export PORT_CLIENT_ID="your-client-id"
   export PORT_CLIENT_SECRET="your-client-secret"

3. Configuration file:
   Run: port config --init
   Then edit: %s`, orgName, DefaultConfigPath())
	}

	return &org, nil
}

// Validate ensures the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Organizations) == 0 {
		return fmt.Errorf("no organizations configured")
	}

	for name, org := range c.Organizations {
		if org.ClientID == "" || org.ClientSecret == "" {
			return fmt.Errorf("organization '%s' missing client_id or client_secret", name)
		}
		if org.APIURL == "" {
			return fmt.Errorf("organization '%s' missing api_url", name)
		}
	}

	return nil
}


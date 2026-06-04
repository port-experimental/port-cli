package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
)

// resolveCommandAuth loads org config and the user OAuth token from creds.json (same as port api / export / import).
func resolveCommandAuth(
	ctx context.Context,
	flags GlobalFlags,
	configManager *config.ConfigManager,
	orgName string,
) (*auth.Token, *config.OrganizationConfig, error) {
	cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, orgName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	useOrg := cfg.GetOrgOrDefault(orgName)
	orgConfig, err := cfg.GetOrgConfig(useOrg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get org config: %w", err)
	}

	token, err := configManager.GetOrRefreshToken(ctx, useOrg)
	if err != nil && !config.ShouldIgnoreGetOrRefreshTokenError(err) {
		return nil, nil, err
	}
	if token == nil {
		return nil, nil, fmt.Errorf("%s", config.MissingAuthCredentialsMessage(configManager.ConfigPath()))
	}
	if token.Claims.UserID == "" {
		token.Claims.UserID = token.Claims.Email
	}
	return token, orgConfig, nil
}

package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
)

// Runtime centralizes config loading, token refresh, and API client setup for commands.
type Runtime struct {
	Flags         GlobalFlags
	ConfigManager *config.ConfigManager
}

// NewRuntime builds a command runtime from global flags stored in ctx.
func NewRuntime(ctx context.Context) *Runtime {
	flags := GetGlobalFlags(ctx)
	return &Runtime{
		Flags:         flags,
		ConfigManager: config.NewConfigManager(flags.ConfigFile),
	}
}

// LoadOrg resolves organization configuration with CLI flag overrides.
// Returns the loaded config, org-specific config, and the resolved org name.
func (r *Runtime) LoadOrg(ctx context.Context, org string) (*config.Config, *config.OrganizationConfig, string, error) {
	cfg, err := r.ConfigManager.LoadWithOverrides(
		r.Flags.ClientID,
		r.Flags.ClientSecret,
		r.Flags.APIURL,
		org,
	)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load configuration: %w", err)
	}

	useOrg := cfg.GetOrgOrDefault(org)
	orgConfig, err := cfg.GetOrgConfig(useOrg)
	if err != nil {
		return nil, nil, "", err
	}

	return cfg, orgConfig, useOrg, nil
}

// ClientForOrg loads org config, refreshes credentials when available, and returns an API client.
func (r *Runtime) ClientForOrg(ctx context.Context, org string) (*api.Client, string, error) {
	_, orgConfig, useOrg, err := r.LoadOrg(ctx, org)
	if err != nil {
		return nil, "", err
	}

	token, err := getOrRefreshToken(ctx, r.ConfigManager, useOrg)
	if err != nil {
		return nil, "", err
	}

	return newAPIClient(orgConfig, token), useOrg, nil
}

// SourceTargetClients loads dual-org configuration and returns API clients for source and target.
func (r *Runtime) SourceTargetClients(ctx context.Context, sourceOrg, targetOrg string) (*api.Client, *api.Client, error) {
	_, baseOrgConfig, targetOrgConfig, err := r.ConfigManager.LoadWithDualOverrides(
		r.Flags.ClientID,
		r.Flags.ClientSecret,
		r.Flags.APIURL,
		sourceOrg,
		r.Flags.TargetClientID,
		r.Flags.TargetClientSecret,
		r.Flags.TargetAPIURL,
		targetOrg,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if baseOrgConfig == nil {
		return nil, nil, fmt.Errorf("base organization configuration not found")
	}
	if targetOrgConfig == nil {
		return nil, nil, fmt.Errorf("target organization configuration not found")
	}

	sourceToken, err := getOrRefreshToken(ctx, r.ConfigManager, sourceOrg)
	if err != nil {
		return nil, nil, err
	}
	targetToken, err := getOrRefreshToken(ctx, r.ConfigManager, targetOrg)
	if err != nil {
		return nil, nil, err
	}

	return newAPIClient(baseOrgConfig, sourceToken), newAPIClient(targetOrgConfig, targetToken), nil
}

func getOrRefreshToken(ctx context.Context, configManager *config.ConfigManager, org string) (*auth.Token, error) {
	token, err := configManager.GetOrRefreshToken(ctx, org)
	if err != nil && !config.ShouldIgnoreGetOrRefreshTokenError(err) {
		return nil, err
	}
	return token, nil
}

func newAPIClient(orgConfig *config.OrganizationConfig, token *auth.Token) *api.Client {
	return api.NewClient(api.ClientOpts{
		Token:        token,
		ClientID:     orgConfig.ClientID,
		ClientSecret: orgConfig.ClientSecret,
		APIURL:       orgConfig.APIURL,
		Timeout:      0,
	})
}

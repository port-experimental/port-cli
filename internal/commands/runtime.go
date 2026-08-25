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
	token, orgConfig, useOrg, err := r.CredentialsForOrg(ctx, org)
	if err != nil {
		return nil, "", err
	}
	return newAPIClient(orgConfig, token), useOrg, nil
}

// CredentialsForOrg resolves org config via CLI flag overrides and returns refreshed credentials.
func (r *Runtime) CredentialsForOrg(ctx context.Context, org string) (*auth.Token, *config.OrganizationConfig, string, error) {
	_, orgConfig, useOrg, err := r.LoadOrg(ctx, org)
	if err != nil {
		return nil, nil, "", err
	}
	token, err := getOrRefreshToken(ctx, r.ConfigManager, useOrg)
	if err != nil {
		return nil, nil, "", err
	}
	return token, orgConfig, useOrg, nil
}

// CredentialsForBaseOrg resolves export-style base org config (dual overrides, base side).
func (r *Runtime) CredentialsForBaseOrg(ctx context.Context, org string) (*auth.Token, *config.OrganizationConfig, string, error) {
	orgConfig, err := r.loadBaseOrgConfig(org)
	if err != nil {
		return nil, nil, "", err
	}
	token, err := getOrRefreshToken(ctx, r.ConfigManager, org)
	if err != nil {
		return nil, nil, "", err
	}
	return token, orgConfig, org, nil
}

// CredentialsForTargetOrg resolves import-style target org config (target overrides with base fallback).
func (r *Runtime) CredentialsForTargetOrg(ctx context.Context, org string) (*auth.Token, *config.OrganizationConfig, string, error) {
	orgConfig, err := r.loadTargetOrgConfig(org)
	if err != nil {
		return nil, nil, "", err
	}
	token, err := getOrRefreshToken(ctx, r.ConfigManager, org)
	if err != nil {
		return nil, nil, "", err
	}
	return token, orgConfig, org, nil
}

// ClientForBaseOrg loads export-style base org config and returns an API client.
func (r *Runtime) ClientForBaseOrg(ctx context.Context, org string) (*api.Client, string, error) {
	token, orgConfig, useOrg, err := r.CredentialsForBaseOrg(ctx, org)
	if err != nil {
		return nil, "", err
	}
	return newAPIClient(orgConfig, token), useOrg, nil
}

// ClientForOrgWithOverrides resolves an org using explicit credential overrides (compare per-side flags).
func (r *Runtime) ClientForOrgWithOverrides(ctx context.Context, org, clientID, clientSecret, apiURL string) (*api.Client, error) {
	_, orgConfig, _, err := r.ConfigManager.LoadWithDualOverrides(
		clientID,
		clientSecret,
		apiURL,
		org,
		"", "", "", "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config for org %s: %w", org, err)
	}
	if orgConfig == nil {
		return nil, fmt.Errorf("organization %s not found in config", org)
	}
	token, err := getOrRefreshToken(ctx, r.ConfigManager, org)
	if err != nil {
		return nil, err
	}
	return newAPIClient(orgConfig, token), nil
}

func (r *Runtime) loadBaseOrgConfig(org string) (*config.OrganizationConfig, error) {
	_, baseOrgConfig, _, err := r.ConfigManager.LoadWithDualOverrides(
		r.Flags.ClientID,
		r.Flags.ClientSecret,
		r.Flags.APIURL,
		org,
		"", "", "", "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if baseOrgConfig == nil {
		return nil, fmt.Errorf("base organization configuration not found")
	}
	return baseOrgConfig, nil
}

func (r *Runtime) loadTargetOrgConfig(org string) (*config.OrganizationConfig, error) {
	targetClientID := r.Flags.TargetClientID
	targetClientSecret := r.Flags.TargetClientSecret
	targetAPIURL := r.Flags.TargetAPIURL
	if targetClientID == "" {
		targetClientID = r.Flags.ClientID
		targetClientSecret = r.Flags.ClientSecret
		targetAPIURL = r.Flags.APIURL
	}
	_, _, targetOrgConfig, err := r.ConfigManager.LoadWithDualOverrides(
		"", "", "", "",
		targetClientID,
		targetClientSecret,
		targetAPIURL,
		org,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if targetOrgConfig == nil {
		return nil, fmt.Errorf("target organization configuration not found")
	}
	return targetOrgConfig, nil
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

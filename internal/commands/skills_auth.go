package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/spf13/cobra"
)

// resolveSkillsAuth loads org config and a Port token the same way as export/import/api:
// stored OAuth from creds.json when available, otherwise client_id/client_secret from config or flags.
func resolveSkillsAuth(
	ctx context.Context,
	flags GlobalFlags,
	configManager *config.ConfigManager,
	orgName string,
) (*auth.Token, *config.OrganizationConfig, *aiservice.Client, error) {
	cfg, err := configManager.LoadWithOverrides(flags.ClientID, flags.ClientSecret, flags.APIURL, orgName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	useOrg := cfg.GetOrgOrDefault(orgName)
	orgConfig, err := cfg.GetOrgConfig(useOrg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get org config: %w", err)
	}

	aiURL := os.Getenv("PORT_AI_SERVICE_URL")
	aiClient := aiservice.NewClient(aiservice.ClientOpts{
		APIURL:       orgConfig.APIURL,
		AIServiceURL: aiURL,
	})

	token, err := configManager.GetOrRefreshToken(ctx, useOrg)
	if err != nil && !config.ShouldIgnoreGetOrRefreshTokenError(err) {
		return nil, nil, nil, err
	}

	if token == nil && orgConfig.ClientID != "" && orgConfig.ClientSecret != "" {
		apiClient := api.NewClient(api.ClientOpts{
			ClientID:     orgConfig.ClientID,
			ClientSecret: orgConfig.ClientSecret,
			APIURL:       orgConfig.APIURL,
		})
		accessToken, tokenErr := apiClient.AccessToken(ctx)
		if tokenErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to authenticate with client credentials: %w", tokenErr)
		}
		parsed, parseErr := auth.ParseToken(accessToken)
		if parseErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse access token: %w", parseErr)
		}
		token = parsed
	}

	if token == nil {
		return nil, nil, nil, fmt.Errorf("%s", config.MissingAuthCredentialsMessage(configManager.ConfigPath()))
	}
	if token.Claims.UserID == "" {
		token.Claims.UserID = token.Claims.Email
	}

	return token, orgConfig, aiClient, nil
}

func newSkillsModuleWithFlags(ctx context.Context, flags GlobalFlags, orgName string) (*skills.Module, *config.ConfigManager, error) {
	configManager := config.NewConfigManager(flags.ConfigFile)
	token, orgConfig, aiClient, err := resolveSkillsAuth(ctx, flags, configManager, orgName)
	if err != nil {
		return nil, nil, err
	}
	return skills.NewModule(token, orgConfig, aiClient, configManager), configManager, nil
}

func skillsOrgName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if cmd.Parent() != nil {
		org, _ := cmd.Parent().PersistentFlags().GetString("org")
		return org
	}
	return ""
}

func newSkillsModule(flags GlobalFlags) (*skills.Module, *config.ConfigManager, error) {
	configManager := config.NewConfigManager(flags.ConfigFile)
	cfg, err := configManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return newSkillsModuleWithFlags(context.Background(), flags, cfg.DefaultOrg)
}

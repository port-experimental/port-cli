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

// resolveSkillsAuth picks credentials for skills + ai-service calls.
// When the org has client_id and client_secret, machine credentials take precedence over stored OAuth.
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

	if orgConfig.ClientID != "" && orgConfig.ClientSecret != "" {
		apiClient := api.NewClient(api.ClientOpts{
			ClientID:     orgConfig.ClientID,
			ClientSecret: orgConfig.ClientSecret,
			APIURL:       orgConfig.APIURL,
		})
		accessToken, err := apiClient.AccessToken(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to authenticate with client credentials: %w", err)
		}
		parsed, err := auth.ParseToken(accessToken)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse access token: %w", err)
		}
		if !parsed.Claims.IsMachine {
			parsed.Claims.IsMachine = true
		}
		if parsed.Claims.UserID == "" {
			parsed.Claims.UserID = orgConfig.ClientID
		}
		return parsed, orgConfig, aiClient, nil
	}

	oauthToken, err := configManager.GetOrRefreshToken(ctx, useOrg)
	if err != nil && !config.ShouldIgnoreGetOrRefreshTokenError(err) {
		return nil, nil, nil, err
	}
	if oauthToken == nil {
		return nil, nil, nil, fmt.Errorf("%s", config.MissingAuthCredentialsMessage(configManager.ConfigPath()))
	}
	if oauthToken.Claims.UserID == "" {
		oauthToken.Claims.UserID = oauthToken.Claims.Email
	}
	return oauthToken, orgConfig, aiClient, nil
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
	orgName := cfg.DefaultOrg
	orgCfg := &config.OrganizationConfig{APIURL: "https://api.getport.io/v1"}
	if orgName != "" {
		if oc, ocErr := cfg.GetOrgConfig(orgName); ocErr == nil {
			orgCfg = oc
		}
	}
	token, _ := configManager.GetToken(orgName)
	aiClient := aiservice.NewClient(aiservice.ClientOpts{APIURL: orgCfg.APIURL, AIServiceURL: os.Getenv("PORT_AI_SERVICE_URL")})
	return skills.NewModule(token, orgCfg, aiClient, configManager), configManager, nil
}

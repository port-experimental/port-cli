package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/spf13/cobra"
)

func newSkillsModuleWithFlags(ctx context.Context, flags GlobalFlags, orgName string) (*skills.Module, *config.ConfigManager, error) {
	configManager := config.NewConfigManager(flags.ConfigFile)
	token, orgConfig, err := resolveCommandAuth(ctx, flags, configManager, orgName)
	if err != nil {
		return nil, nil, err
	}

	aiURL := os.Getenv("PORT_AI_SERVICE_URL")
	aiClient := aiservice.NewClient(aiservice.ClientOpts{
		APIURL:       orgConfig.APIURL,
		AIServiceURL: aiURL,
	})

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

package commands

import (
	"fmt"
	"log"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/port-experimental/port-cli/internal/styles"
	"github.com/spf13/cobra"
)

// RegisterAPI registers the API command and all subcommands.
func RegisterAuth(rootCmd *cobra.Command) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate the cli with Port",
		Long:  "Authenticate the cli with Port using SSO",
	}

	authCmd.AddCommand(registerLogin())
	authCmd.AddCommand(registerToken())
	authCmd.AddCommand(registerLogout())

	rootCmd.AddCommand(authCmd)
}

// registerLogin registers the login command.
func registerLogin() *cobra.Command {
	var org string
	var withToken bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Port",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, org, withToken)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read token from standard input")

	return cmd
}

func runLogin(cmd *cobra.Command, org string, withToken bool) error {
	ctx := cmd.Context()
	flags := GetGlobalFlags(cmd.Context())
	configManager := config.NewConfigManager(flags.ConfigFile)
	if withToken {
		token, err := auth.ReadTokenFromStdin()
		if err != nil {
			return fmt.Errorf("failed reading token (%w)", err)
		}

		parsed, err := auth.ParseToken(token)
		if err != nil {
			return fmt.Errorf("failed parsing token (%w)", err)
		}

		err = configManager.StoreToken(org, parsed)
		if err != nil {
			return fmt.Errorf("failed storing token (%w)", err)
		}

		lipgloss.Printf("%s Using provided token\n", styles.CheckMark)

		return err
	}

	var region string
	cfg, err := configManager.LoadWithOverrides(
		flags.ClientID,
		flags.ClientSecret,
		flags.APIURL,
		org,
	)
	if err != nil {
		return fmt.Errorf("failed to load configuration (%w)", err)
	}

	useOrg := cfg.GetOrgOrDefault(org)
	orgConfig, err := cfg.GetOrgConfig(useOrg)
	if useOrg == "" || err != nil {
		lipgloss.Printf("%s No org provided or configured as default\n", styles.QuestionMark)
	}

	apiUrl := "https://api.port.io"
	if orgConfig == nil || orgConfig.APIURL == "" {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Choose your region").
					Options(
						huh.NewOption("Europe", "eu"),
						huh.NewOption("US", "us"),
					).
					Value(&region),
			)).WithTheme(&themeBase{})
		err = form.Run()
		if err != nil {
			log.Fatal(err)
			return fmt.Errorf("unexpected error (%w)", err)
		}
		if region == "us" {
			apiUrl = "https://api.us.getport.io"
		}
	} else {
		apiUrl = orgConfig.APIURL
	}

	baseUrl := strings.Replace(apiUrl, "api", "auth", 1)
	if strings.Contains(apiUrl, "stg-01") || strings.Contains(apiUrl, "localhost") {
		baseUrl = "https://auth.staging.getport.io"
	}

	token, err := auth.TokenFromOAuth(ctx, auth.LoginOpts{
		BaseURL: baseUrl,
		APIURL:  strings.TrimSuffix(apiUrl, "/v1"),
	})
	if err != nil {
		return err
	}

	if err != nil {
		return fmt.Errorf("unexpected error (%w)", err)
	}

	if err = configManager.StoreToken(useOrg, token); err != nil {
		return fmt.Errorf("unexpected error while storing the token (%w)", err)
	}

	lipgloss.Printf(
		"%s Successfully logged in as %s to %s\n",
		styles.CheckMark,
		styles.Bold.Render(token.Claims.Email),
		styles.Bold.Render(token.Claims.OrgName),
	)

	return nil
}

// registerToken registers the token command.
func registerToken() *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the authentication token port uses for an org name",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())

			configManager := config.NewConfigManager(flags.ConfigFile)
			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				org,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration (%w)", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)

			token, err := configManager.GetToken(useOrg)
			if err != nil {
				return fmt.Errorf("failed fetching token (%w)", err)
			}
			fmt.Printf("Bearer %s", token.Token)
			return nil
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	return cmd
}

// registerLogout registers the logout command.
func registerLogout() *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout to Port",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := GetGlobalFlags(cmd.Context())
			configManager := config.NewConfigManager(flags.ConfigFile)
			cfg, err := configManager.LoadWithOverrides(
				flags.ClientID,
				flags.ClientSecret,
				flags.APIURL,
				org,
			)
			if err != nil {
				return fmt.Errorf("failed to load configuration (%w)", err)
			}

			useOrg := cfg.GetOrgOrDefault(org)
			if useOrg == "" {
				return fmt.Errorf("org not found. Configure a default org or pass the --org flag")
			}

			token, err := configManager.GetToken(useOrg)
			if err != nil {
				return fmt.Errorf("failed logging out (%w)", err)
			}
			configManager.DeleteToken(useOrg)

			lipgloss.Printf(
				"%s Successfully logged out %s from %s\n",
				styles.CheckMark,
				styles.Bold.Render(token.Claims.Email),
				styles.Bold.Render(token.Claims.OrgName),
			)

			return nil
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	return cmd
}

type themeBase struct{}

func (t *themeBase) Theme(isDark bool) *huh.Styles {
	return huh.ThemeBase(isDark)
}

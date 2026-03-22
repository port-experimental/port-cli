package commands

import (
	"fmt"
	"log"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/golang-jwt/jwt/v5"
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
	if withToken {
		token, err := auth.ReadTokenFromStdin()
		fmt.Printf("Using provided token %s\n", token)
		// TODO: save token
		return err
	}

	var region string
	configManager := config.NewConfigManager(flags.ConfigFile)
	cfg, err := configManager.LoadWithOverrides(
		flags.ClientID,
		flags.ClientSecret,
		flags.APIURL,
		org,
	)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	useOrg := org
	if useOrg == "" {
		useOrg = cfg.DefaultOrg
	}
	orgConfig, err := cfg.GetOrgConfig(useOrg)
	if useOrg == "" || err != nil {
		lipgloss.Printf("%s no org provided or configured\n", styles.QuestionMark)
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
			return fmt.Errorf("Unexpected error", err)
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

	company := token.Claims.(jwt.MapClaims)["companyName"].(string)
	aud := token.Claims.(jwt.MapClaims)["aud"].(string)
	email := token.Claims.(jwt.MapClaims)[fmt.Sprintf("%s/email", aud)].(string)
	if err != nil {
		return fmt.Errorf("Unexpected error", err)
	}

	if err = configManager.StoreToken(useOrg, token); err != nil {
		return fmt.Errorf("Unexpected error while storing the token (%v)", err)
	}

	bold := lipgloss.NewStyle().Bold(true)
	lipgloss.Printf(
		"%s Successfuly logged in as %s to %s.\n",
		styles.CheckMark,
		bold.Render(email),
		bold.Render(company),
	)

	return nil
}

// registerLogout registers the logout command.
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
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			useOrg := org
			if useOrg == "" {
				useOrg = cfg.DefaultOrg
			}
			if useOrg == "" {
				return fmt.Errorf("Org not found. Configure a default org or pass the --org flag.")
			}

			token, err := configManager.GetToken(useOrg)
			if err != nil {
				return err
			}
			fmt.Printf("Bearer %s", token.Raw)
			return nil
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	return cmd
}

// registerLogout registers the logout command.
func registerLogout() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout to Port",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}

type themeBase struct{}

func (t *themeBase) Theme(isDark bool) *huh.Styles {
	return huh.ThemeBase(isDark)
}

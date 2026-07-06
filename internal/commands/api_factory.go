package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

// APIResourceSpec describes a port api <resource> command group.
type APIResourceSpec struct {
	Name      string
	Short     string
	Singular  string
	Plural    string
	Operations []APIOperationSpec
}

// APIOperationSpec describes one subcommand under port api <resource>.
type APIOperationSpec struct {
	Name          string
	Use           string
	Short         string
	Args          cobra.PositionalArgs
	HasFormat     bool
	DataFile      bool
	HasForce      bool
	ConfirmDelete bool
	SuccessPrint  func(args []string) string
	ErrorMessage  func(spec APIResourceSpec, err error) string
	Run           func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}) (any, error)
}

func registerAPIResource(spec APIResourceSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   spec.Name,
		Short: spec.Short,
	}
	for _, op := range spec.Operations {
		cmd.AddCommand(buildAPIOperationCommand(spec, op))
	}
	return cmd
}

func buildAPIOperationCommand(spec APIResourceSpec, op APIOperationSpec) *cobra.Command {
	var org, format, dataFile string
	var force bool

	cmd := &cobra.Command{
		Use:   op.Use,
		Short: op.Short,
		Args:  op.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			if op.ConfirmDelete && !force {
				id := args[0]
				cmd.Printf("Are you sure you want to delete %s '%s'? [y/N]: ", spec.Singular, id)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					cmd.Println("Operation cancelled")
					return nil
				}
			}

			rt := NewRuntime(cmd.Context())
			client, _, err := rt.ClientForOrg(cmd.Context(), org)
			if err != nil {
				return err
			}
			defer client.Close()

			var data map[string]interface{}
			if op.DataFile {
				data, err = loadJSONFile(dataFile)
				if err != nil {
					return fmt.Errorf("failed to load data file: %w", err)
				}
			}

			result, err := op.Run(cmd.Context(), client, args, data)
			if err != nil {
				if op.ErrorMessage != nil {
					return fmt.Errorf("%s: %w", op.ErrorMessage(spec, err), err)
				}
				return err
			}

			if op.SuccessPrint != nil {
				if msg := op.SuccessPrint(args); msg != "" {
					cmd.Print(msg)
				}
			}

			if op.HasFormat {
				return formatOutput(result, format)
			}
			if op.DataFile {
				return formatOutput(result, "json")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization name (uses default if not specified)")
	if op.HasFormat {
		cmd.Flags().StringVarP(&format, "format", "f", "json", "Output format: json, yaml")
	}
	if op.DataFile {
		cmd.Flags().StringVar(&dataFile, "data", "", fmt.Sprintf("JSON file with %s data", spec.Singular))
		cmd.MarkFlagRequired("data")
	}
	if op.HasForce {
		cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	}

	return cmd
}

func teamsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "teams",
		Short:    "Team operations",
		Singular: "team",
		Plural:   "teams",
	}

	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all teams",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}) (any, error) {
				return client.GetTeams(ctx)
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new team",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Team created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}) (any, error) {
				return client.CreateTeam(ctx, api.Team(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [team-name]",
			Short:    "Update an existing team",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Team updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}) (any, error) {
				return client.UpdateTeam(ctx, args[0], api.Team(data))
			},
		},
		{
			Name:          "delete",
			Use:           "delete [team-name]",
			Short:         "Delete a team",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Team '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}) (any, error) {
				return nil, client.DeleteTeam(ctx, args[0])
			},
		},
	}

	return spec
}

func usersResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "users",
		Short:    "User operations",
		Singular: "user",
		Plural:   "users",
	}

	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all users",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}) (any, error) {
				return client.GetUsers(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [email]",
			Short:     "Get a specific user by email",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}) (any, error) {
				return client.GetUser(ctx, args[0])
			},
		},
	}

	return spec
}

package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

// APIResourceSpec describes a port api <resource> command group.
type APIResourceSpec struct {
	Name       string
	Short      string
	Singular   string
	Plural     string
	Operations []APIOperationSpec
}

// APIExtraValues holds per-operation flag values registered via APIExtraFlagSpec.
type APIExtraValues struct {
	Strings map[string]string
	Bools   map[string]bool
}

// APIExtraFlagSpec describes an additional operation flag beyond org/format/data/force.
type APIExtraFlagSpec struct {
	Name        string
	Shorthand   string
	Usage       string
	Required    bool
	Bool        bool
	DefaultBool bool
}

// APIOperationSpec describes one subcommand under port api <resource>.
type APIOperationSpec struct {
	Name                string
	Use                 string
	Short               string
	Args                cobra.PositionalArgs
	HasFormat           bool
	DataFile            bool
	HasForce            bool
	ConfirmDelete       bool
	ConfirmDeletePrompt func(args []string) string
	ExtraFlags          []APIExtraFlagSpec
	SuccessPrint        func(args []string) string
	ErrorMessage        func(spec APIResourceSpec, err error) string
	Run                 func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, extra APIExtraValues) (any, error)
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
	extraStrings := make(map[string]*string)
	extraBools := make(map[string]*bool)

	cmd := &cobra.Command{
		Use:   op.Use,
		Short: op.Short,
		Args:  op.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			if op.ConfirmDelete && !force {
				prompt := fmt.Sprintf("Are you sure you want to delete %s '%s'? [y/N]: ", spec.Singular, args[0])
				if op.ConfirmDeletePrompt != nil {
					prompt = op.ConfirmDeletePrompt(args)
				}
				cmd.Print(prompt)
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

			extra := APIExtraValues{
				Strings: make(map[string]string, len(extraStrings)),
				Bools:   make(map[string]bool, len(extraBools)),
			}
			for name, ptr := range extraStrings {
				extra.Strings[name] = *ptr
			}
			for name, ptr := range extraBools {
				extra.Bools[name] = *ptr
			}

			result, err := op.Run(cmd.Context(), client, args, data, extra)
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
	for _, f := range op.ExtraFlags {
		if f.Bool {
			v := f.DefaultBool
			extraBools[f.Name] = &v
			cmd.Flags().BoolVar(extraBools[f.Name], f.Name, f.DefaultBool, f.Usage)
			continue
		}
		var v string
		extraStrings[f.Name] = &v
		if f.Shorthand != "" {
			cmd.Flags().StringVarP(extraStrings[f.Name], f.Name, f.Shorthand, "", f.Usage)
		} else {
			cmd.Flags().StringVar(extraStrings[f.Name], f.Name, "", f.Usage)
		}
		if f.Required {
			cmd.MarkFlagRequired(f.Name)
		}
	}

	return cmd
}

func blueprintExtraFlag(required bool) APIExtraFlagSpec {
	usage := "Filter by blueprint ID"
	if required {
		usage = "Blueprint ID"
	}
	return APIExtraFlagSpec{Name: "blueprint", Shorthand: "b", Usage: usage, Required: required}
}

func deleteChildFromBlueprintPrompt(child string) func([]string) string {
	return func(args []string) string {
		return fmt.Sprintf("Are you sure you want to delete %s '%s' from blueprint '%s'? [y/N]: ", child, args[1], args[0])
	}
}

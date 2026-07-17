package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func actionsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "actions",
		Short:    "Action operations",
		Singular: "action",
		Plural:   "actions",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List actions",
			HasFormat: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(false),
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, extra APIExtraValues) (any, error) {
				if bp := extra.Strings["blueprint"]; bp != "" {
					return client.GetActions(ctx, bp)
				}
				return client.GetAllActions(ctx)
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new action",
			DataFile: true,
			ExtraFlags: []APIExtraFlagSpec{
				blueprintExtraFlag(true),
			},
			SuccessPrint: func(_ []string) string {
				return "✓ Action created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, extra APIExtraValues) (any, error) {
				return client.CreateAction(ctx, extra.Strings["blueprint"], api.Action(data))
			},
		},
		{
			Name:     "update",
			Use:      "update [blueprint-id] [action-id]",
			Short:    "Update an existing action",
			Args:     cobra.ExactArgs(2),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateAction(ctx, args[0], args[1], api.Action(data))
			},
		},
		{
			Name:                "delete",
			Use:                 "delete [blueprint-id] [action-id]",
			Short:               "Delete an action",
			Args:                cobra.ExactArgs(2),
			HasForce:            true,
			ConfirmDelete:       true,
			ConfirmDeletePrompt: deleteChildFromBlueprintPrompt("action"),
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Action '%s' deleted successfully!\n", args[1])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteAction(ctx, args[0], args[1])
			},
		},
	}
	return spec
}

func actionRunsResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "action-runs",
		Short:    "Action run operations",
		Singular: "action run",
		Plural:   "action runs",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all action runs",
			HasFormat: true,
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to list action runs"
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetActionRuns(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [run-id]",
			Short:     "Get a specific action run",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to get action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetActionRun(ctx, args[0])
			},
		},
		{
			Name:     "update",
			Use:      "update [run-id]",
			Short:    "Update an action run",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action run updated successfully!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to update action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateActionRun(ctx, args[0], data)
			},
		},
		{
			Name:     "approve",
			Use:      "approve [run-id]",
			Short:    "Approve or decline an action run",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action run approval submitted!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to approve action run"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.ApproveActionRun(ctx, args[0], data)
			},
		},
		{
			Name:     "execute",
			Use:      "execute [action-id]",
			Short:    "Execute an action (create a new action run)",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Action executed successfully!\n"
			},
			ErrorMessage: func(_ APIResourceSpec, _ error) string {
				return "failed to execute action"
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.ExecuteAction(ctx, args[0], data)
			},
		},
	}
	return spec
}

func webhooksResourceSpec() APIResourceSpec {
	spec := APIResourceSpec{
		Name:     "webhooks",
		Short:    "Webhook operations",
		Singular: "webhook",
		Plural:   "webhooks",
	}
	spec.Operations = []APIOperationSpec{
		{
			Name:      "list",
			Use:       "list",
			Short:     "List all webhooks",
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to list %s", s.Plural)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetWebhooks(ctx)
			},
		},
		{
			Name:      "get",
			Use:       "get [webhook-id]",
			Short:     "Get a specific webhook",
			Args:      cobra.ExactArgs(1),
			HasFormat: true,
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to get %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.GetWebhook(ctx, args[0])
			},
		},
		{
			Name:     "create",
			Use:      "create",
			Short:    "Create a new webhook",
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Webhook created successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to create %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.CreateWebhook(ctx, data)
			},
		},
		{
			Name:     "update",
			Use:      "update [webhook-id]",
			Short:    "Update an existing webhook",
			Args:     cobra.ExactArgs(1),
			DataFile: true,
			SuccessPrint: func(_ []string) string {
				return "✓ Webhook updated successfully!\n"
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to update %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
				return client.UpdateWebhook(ctx, args[0], data)
			},
		},
		{
			Name:          "delete",
			Use:           "delete [webhook-id]",
			Short:         "Delete a webhook",
			Args:          cobra.ExactArgs(1),
			HasForce:      true,
			ConfirmDelete: true,
			SuccessPrint: func(args []string) string {
				return fmt.Sprintf("✓ Webhook '%s' deleted successfully!\n", args[0])
			},
			ErrorMessage: func(s APIResourceSpec, _ error) string {
				return fmt.Sprintf("failed to delete %s", s.Singular)
			},
			Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
				return nil, client.DeleteWebhook(ctx, args[0])
			},
		},
	}
	return spec
}

func workflowsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "workflows",
		Short:    "Workflow operations",
		Singular: "workflow",
		Plural:   "workflows",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List all workflows",
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to list %s", s.Plural)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflows(ctx)
				},
			},
			{
				Name:      "get",
				Use:       "get [identifier]",
				Short:     "Get a specific workflow",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to get %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflow(ctx, args[0])
				},
			},
			{
				Name:     "create",
				Use:      "create",
				Short:    "Create a workflow",
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Workflow created successfully!\n"
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to create %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CreateWorkflow(ctx, api.Workflow(data))
				},
			},
			{
				Name:     "update",
				Use:      "update [identifier]",
				Short:    "Update a workflow",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Workflow updated successfully!\n"
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to update %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateWorkflow(ctx, args[0], api.Workflow(data))
				},
			},
			{
				Name:          "delete",
				Use:           "delete [identifier]",
				Short:         "Delete a workflow",
				Args:          cobra.ExactArgs(1),
				HasForce:      true,
				ConfirmDelete: true,
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ Workflow '%s' deleted successfully!\n", args[0])
				},
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to delete %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return nil, client.DeleteWorkflow(ctx, args[0])
				},
			},
			{
				Name:      "get-node",
				Use:       "get-node [workflow-identifier] [node-identifier]",
				Short:     "Get a workflow node definition",
				Args:      cobra.ExactArgs(2),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get workflow node"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowNode(ctx, args[0], args[1])
				},
			},
			{
				Name:      "list-triggers",
				Use:       "list-triggers",
				Short:     "List self-service workflow triggers",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list workflow self-service triggers"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowSelfServiceTriggers(ctx)
				},
			},
		},
	}
}

func workflowRunsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "workflow-runs",
		Short:    "Workflow run operations",
		Singular: "workflow run",
		Plural:   "workflow runs",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List workflow runs",
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to list %s", s.Plural)
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowRuns(ctx, nil)
				},
			},
			{
				Name:      "get",
				Use:       "get [identifier]",
				Short:     "Get a specific workflow run",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(s APIResourceSpec, _ error) string {
					return fmt.Sprintf("failed to get %s", s.Singular)
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowRun(ctx, args[0])
				},
			},
			{
				Name:     "trigger",
				Use:      "trigger [workflow-identifier]",
				Short:    "Trigger a workflow run",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Workflow run triggered successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to trigger workflow run"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.TriggerWorkflowRun(ctx, args[0], data)
				},
			},
			{
				Name:     "cancel",
				Use:      "cancel [identifier]",
				Short:    "Cancel a workflow run",
				Args:     cobra.ExactArgs(1),
				SuccessPrint: func(args []string) string {
					return fmt.Sprintf("✓ Workflow run '%s' cancel requested!\n", args[0])
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to cancel workflow run"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.CancelWorkflowRun(ctx, args[0])
				},
			},
			{
				Name:      "logs",
				Use:       "logs [identifier]",
				Short:     "Get logs for all node runs in a workflow run",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get workflow run logs"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowRunLogs(ctx, args[0])
				},
			},
			{
				Name:      "node-logs",
				Use:       "node-logs [node-run-identifier]",
				Short:     "Get logs for a workflow node run",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get workflow node run logs"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetWorkflowNodeRunLogs(ctx, args[0])
				},
			},
			{
				Name:     "update-node-run",
				Use:      "update-node-run [node-run-identifier]",
				Short:    "Update a workflow node run",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Workflow node run updated successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to update workflow node run"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.UpdateWorkflowNodeRun(ctx, args[0], data)
				},
			},
			{
				Name:     "write-node-logs",
				Use:      "write-node-logs [node-run-identifier]",
				Short:    "Write logs for a workflow node run",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				SuccessPrint: func(_ []string) string {
					return "✓ Workflow node run logs written successfully!\n"
				},
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to write workflow node run logs"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.WriteWorkflowNodeRunLogs(ctx, args[0], data)
				},
			},
		},
	}
}

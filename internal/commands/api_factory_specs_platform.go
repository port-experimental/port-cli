package commands

import (
	"context"
	"fmt"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/spf13/cobra"
)

func auditResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "audit",
		Short:    "Audit log operations",
		Singular: "audit log entry",
		Plural:   "audit log entries",
		Operations: []APIOperationSpec{
			{
				Name:      "list",
				Use:       "list",
				Short:     "List audit log entries",
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to list audit logs"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.GetAuditLogs(ctx)
				},
			},
		},
	}
}

func agentsResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "agents",
		Short:    "Agent operations",
		Singular: "agent",
		Plural:   "agents",
		Operations: []APIOperationSpec{
			{
				Name:     "invoke",
				Use:      "invoke [agent-id]",
				Short:    "Invoke an agent",
				Args:     cobra.ExactArgs(1),
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to invoke agent"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "POST",
						Endpoint: fmt.Sprintf("/agent/%s/invoke", args[0]),
						Data:     data,
					})
				},
			},
		},
	}
}

func aiResourceSpec() APIResourceSpec {
	return APIResourceSpec{
		Name:     "ai",
		Short:    "Port AI operations",
		Singular: "AI invocation",
		Plural:   "AI invocations",
		Operations: []APIOperationSpec{
			{
				Name:     "invoke",
				Use:      "invoke",
				Short:    "Invoke Port AI",
				DataFile: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to invoke AI"
				},
				Run: func(ctx context.Context, client *api.Client, _ []string, data map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "POST",
						Endpoint: "/ai/invoke",
						Data:     data,
					})
				},
			},
			{
				Name:      "get",
				Use:       "get [invocation-id]",
				Short:     "Get an AI invocation result",
				Args:      cobra.ExactArgs(1),
				HasFormat: true,
				ErrorMessage: func(_ APIResourceSpec, _ error) string {
					return "failed to get AI invocation"
				},
				Run: func(ctx context.Context, client *api.Client, args []string, _ map[string]interface{}, _ APIExtraValues) (any, error) {
					return client.Request(ctx, api.RequestParams{
						Method:   "GET",
						Endpoint: fmt.Sprintf("/ai/invoke/%s", args[0]),
					})
				},
			},
		},
	}
}

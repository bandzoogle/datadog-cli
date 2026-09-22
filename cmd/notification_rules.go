package cmd

import (
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var notificationRulesCmd = &cobra.Command{
	Use:     "notification-rules",
	Aliases: []string{"notification-rule"},
	Short:   "List and retrieve Datadog monitor notification rules",
}

var notificationRulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List monitor notification rules",
	RunE:  runNotificationRulesList,
}

var notificationRulesGetCmd = &cobra.Command{
	Use:   "get <rule-id>",
	Short: "Get a monitor notification rule",
	Args:  cobra.ExactArgs(1),
	RunE:  runNotificationRulesGet,
}

func init() {
	rootCmd.AddCommand(notificationRulesCmd)
	notificationRulesCmd.AddCommand(notificationRulesListCmd, notificationRulesGetCmd)

	notificationRulesListCmd.Flags().Int32("page", 0, "Page number")
	notificationRulesListCmd.Flags().Int32("per-page", 100, "Notification rules per page")
	notificationRulesListCmd.Flags().String("sort", "", "Sort order, e.g. name:asc or created_at:desc")
	notificationRulesListCmd.Flags().String("filters", "", "Server-side filter query")
	notificationRulesListCmd.Flags().String("include", "", "Comma-separated related resources to include, e.g. created_by")

	notificationRulesGetCmd.Flags().String("include", "", "Comma-separated related resources to include, e.g. created_by")
}

func runNotificationRulesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	page, _ := cmd.Flags().GetInt32("page")
	perPage, _ := cmd.Flags().GetInt32("per-page")
	sort, _ := cmd.Flags().GetString("sort")
	filters, _ := cmd.Flags().GetString("filters")
	include, _ := cmd.Flags().GetString("include")

	params := buildNotificationRulesListParams(page, perPage, sort, filters, include)
	api := datadogV2.NewMonitorsApi(client.API)
	resp, httpResp, err := api.GetMonitorNotificationRules(client.Context, params)
	if err != nil {
		return apiError("notification-rules list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "notification-rules list", "filter": filters},
		meta(client.Site, map[string]any{
			"page":     page,
			"per_page": perPage,
			"sort":     sort,
			"filters":  filters,
			"include":  include,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func buildNotificationRulesListParams(page, perPage int32, sort, filters, include string) datadogV2.GetMonitorNotificationRulesOptionalParameters {
	params := *datadogV2.NewGetMonitorNotificationRulesOptionalParameters()
	if page > 0 {
		params.WithPage(page)
	}
	if perPage > 0 {
		params.WithPerPage(perPage)
	}
	if sort != "" {
		params.WithSort(sort)
	}
	if filters != "" {
		params.WithFilters(filters)
	}
	if include != "" {
		params.WithInclude(include)
	}
	return params
}

func runNotificationRulesGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	ruleID := args[0]
	include, _ := cmd.Flags().GetString("include")

	params := *datadogV2.NewGetMonitorNotificationRuleOptionalParameters()
	if include != "" {
		params.WithInclude(include)
	}

	api := datadogV2.NewMonitorsApi(client.API)
	resp, httpResp, err := api.GetMonitorNotificationRule(client.Context, ruleID, params)
	if err != nil {
		return apiError("notification-rules get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "notification-rules get", "rule_id": ruleID},
		meta(client.Site, map[string]any{"include": include}, httpResp),
		resp,
		outputOptions(),
	)
}

package cmd

import (
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var dashboardsCmd = &cobra.Command{
	Use:     "dashboards",
	Aliases: []string{"dashboard"},
	Short:   "List and retrieve Datadog dashboards",
}

var dashboardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List dashboards",
	RunE:  runDashboardsList,
}

var dashboardsGetCmd = &cobra.Command{
	Use:   "get <dashboard-id>",
	Short: "Get a dashboard definition including widgets",
	Args:  cobra.ExactArgs(1),
	RunE:  runDashboardsGet,
}

func init() {
	rootCmd.AddCommand(dashboardsCmd)
	dashboardsCmd.AddCommand(dashboardsListCmd, dashboardsGetCmd)

	dashboardsListCmd.Flags().String("query", "", "Client-side filter for dashboard title, id, or URL")
	dashboardsListCmd.Flags().Int64("limit", 100, "Maximum dashboards to request")
	dashboardsListCmd.Flags().Int64("start", 0, "Offset for dashboard listing")
	dashboardsListCmd.Flags().Bool("deleted", false, "Include deleted dashboards")
	dashboardsListCmd.Flags().Bool("shared", false, "Only include shared dashboards")
}

func runDashboardsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	limit, _ := cmd.Flags().GetInt64("limit")
	start, _ := cmd.Flags().GetInt64("start")
	deleted, _ := cmd.Flags().GetBool("deleted")
	shared, _ := cmd.Flags().GetBool("shared")

	params := datadogV1.NewListDashboardsOptionalParameters().
		WithCount(limit).
		WithStart(start)
	if deleted {
		params.WithFilterDeleted(true)
	}
	if shared {
		params.WithFilterShared(true)
	}

	api := datadogV1.NewDashboardsApi(client.API)
	resp, httpResp, err := api.ListDashboards(client.Context, *params)
	if err != nil {
		return apiError("dashboards list", httpResp, err)
	}
	filtered := resp
	if query != "" {
		filtered.Dashboards = filterDashboards(resp.Dashboards, query)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "dashboards list", "filter": query},
		meta(client.Site, map[string]any{
			"limit":   limit,
			"start":   start,
			"deleted": deleted,
			"shared":  shared,
		}, httpResp),
		filtered,
		outputOptions(),
	)
}

func runDashboardsGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	id := args[0]

	api := datadogV1.NewDashboardsApi(client.API)
	resp, httpResp, err := api.GetDashboard(client.Context, id)
	if err != nil {
		return apiError("dashboards get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "dashboards get", "dashboard_id": id},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func filterDashboards(dashboards []datadogV1.DashboardSummaryDefinition, query string) []datadogV1.DashboardSummaryDefinition {
	query = strings.ToLower(query)
	filtered := make([]datadogV1.DashboardSummaryDefinition, 0, len(dashboards))
	for _, dashboard := range dashboards {
		if strings.Contains(strings.ToLower(dashboard.GetTitle()), query) ||
			strings.Contains(strings.ToLower(dashboard.GetId()), query) ||
			strings.Contains(strings.ToLower(dashboard.GetUrl()), query) {
			filtered = append(filtered, dashboard)
		}
	}
	return filtered
}

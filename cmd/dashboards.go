package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var dashboardsCmd = &cobra.Command{
	Use:     "dashboards",
	Aliases: []string{"dashboard"},
	Short:   "List, retrieve, and apply Datadog dashboards",
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

var dashboardsApplyCmd = &cobra.Command{
	Use:   "apply <dashboard.json>",
	Short: "Create or update a dashboard from JSON",
	Long: `Create or update a dashboard from a canonical JSON definition.

When the JSON contains an id, apply updates that dashboard. Otherwise apply
matches an existing dashboard by exact title. It refuses to choose when more
than one dashboard has the same title, preventing accidental overwrites.`,
	Args: cobra.ExactArgs(1),
	RunE: runDashboardsApply,
}

func init() {
	rootCmd.AddCommand(dashboardsCmd)
	dashboardsCmd.AddCommand(dashboardsListCmd, dashboardsGetCmd, dashboardsApplyCmd)

	dashboardsListCmd.Flags().String("query", "", "Client-side filter for dashboard title, id, or URL")
	dashboardsListCmd.Flags().Int64("limit", 100, "Maximum dashboards to request")
	dashboardsListCmd.Flags().Int64("start", 0, "Offset for dashboard listing")
	dashboardsListCmd.Flags().Bool("deleted", false, "Include deleted dashboards")
	dashboardsListCmd.Flags().Bool("shared", false, "Only include shared dashboards")
	dashboardsApplyCmd.Flags().Bool("dry-run", false, "Validate JSON and show the intended match without writing")
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

func runDashboardsApply(cmd *cobra.Command, args []string) error {
	dashboard, err := loadDashboard(args[0])
	if err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "dashboards apply", "file": args[0]},
			map[string]any{"dry_run": true},
			map[string]any{
				"action":       intendedDashboardAction(dashboard),
				"dashboard_id": dashboard.GetId(),
				"title":        dashboard.Title,
				"widgets":      len(dashboard.Widgets),
			},
			outputOptions(),
		)
	}

	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewDashboardsApi(client.API)
	id := dashboard.GetId()
	action := "update"
	if id == "" {
		resp, httpResp, err := api.ListDashboards(client.Context,
			*datadogV1.NewListDashboardsOptionalParameters().WithCount(1000))
		if err != nil {
			return apiError("dashboards apply lookup", httpResp, err)
		}
		id, err = exactDashboardID(resp.Dashboards, dashboard.Title)
		if err != nil {
			return err
		}
	}

	var result datadogV1.Dashboard
	if id == "" {
		action = "create"
		created, response, createErr := api.CreateDashboard(client.Context, dashboard)
		if createErr != nil {
			return apiError("dashboards apply create", response, createErr)
		}
		result = created
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "dashboards apply", "file": args[0]},
			meta(client.Site, map[string]any{"action": action}, response),
			result,
			outputOptions(),
		)
	}

	dashboard.Id = nil
	updated, response, updateErr := api.UpdateDashboard(client.Context, id, dashboard)
	if updateErr != nil {
		return apiError("dashboards apply update", response, updateErr)
	}
	result = updated
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "dashboards apply", "file": args[0]},
		meta(client.Site, map[string]any{"action": action}, response),
		result,
		outputOptions(),
	)
}

func loadDashboard(path string) (datadogV1.Dashboard, error) {
	var dashboard datadogV1.Dashboard
	body, err := os.ReadFile(path)
	if err != nil {
		return dashboard, fmt.Errorf("read dashboard JSON: %w", err)
	}
	if err := json.Unmarshal(body, &dashboard); err != nil {
		return dashboard, fmt.Errorf("parse dashboard JSON: %w", err)
	}
	if strings.TrimSpace(dashboard.Title) == "" {
		return dashboard, fmt.Errorf("dashboard title is required")
	}
	if len(dashboard.Widgets) == 0 {
		return dashboard, fmt.Errorf("dashboard widgets are required")
	}
	return dashboard, nil
}

func intendedDashboardAction(dashboard datadogV1.Dashboard) string {
	if dashboard.GetId() != "" {
		return "update-by-id"
	}
	return "create-or-update-by-title"
}

func exactDashboardID(dashboards []datadogV1.DashboardSummaryDefinition, title string) (string, error) {
	var ids []string
	for _, dashboard := range dashboards {
		if dashboard.GetTitle() == title {
			ids = append(ids, dashboard.GetId())
		}
	}
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("refusing to apply: %d dashboards have exact title %q", len(ids), title)
	}
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

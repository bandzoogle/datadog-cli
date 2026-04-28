package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/bandzoogle/datadog-cli/internal/timeutil"
	"github.com/spf13/cobra"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Analyze Datadog Cloud Cost Management data",
	Long: `Analyze Datadog Cloud Cost Management data.

Cost data is queried through Datadog's metrics API with the cloud_cost data
source. Configuration and governance resources are read through the Cloud Cost
Management API.`,
}

var costAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Group cloud costs to find cost-cutting targets",
	Long: `Group cloud costs with the cloud_cost data source.

The default query highlights spend by service over recent complete days. Use
--group-by to pivot by owner, team, subaccountname, region, or other normalized
cost tags.`,
	RunE: runCostAnalyze,
}

var costAccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Inspect Cloud Cost Management accounts",
}

var costAccountsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured Cloud Cost Management accounts",
	RunE:  runCostAccountsList,
}

var costBudgetsCmd = &cobra.Command{
	Use:   "budgets",
	Short: "Inspect Cloud Cost Management budgets",
}

var costBudgetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Cloud Cost Management budgets",
	RunE:  runCostBudgetsList,
}

var costAllocationRulesCmd = &cobra.Command{
	Use:   "allocation-rules",
	Short: "Inspect custom cost allocation rules",
}

var costAllocationRulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List custom cost allocation rules",
	RunE:  runCostAllocationRulesList,
}

var costTagPipelinesCmd = &cobra.Command{
	Use:   "tag-pipelines",
	Short: "Inspect Cloud Cost tag pipelines",
}

var costTagPipelinesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Cloud Cost tag pipeline rulesets",
	RunE:  runCostTagPipelinesList,
}

var costCustomCostsCmd = &cobra.Command{
	Use:   "custom-costs",
	Short: "Inspect uploaded custom cost files",
}

var costCustomCostsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List custom cost files",
	RunE:  runCostCustomCostsList,
}

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.AddCommand(costAnalyzeCmd, costAccountsCmd, costBudgetsCmd, costAllocationRulesCmd, costTagPipelinesCmd, costCustomCostsCmd)
	costAccountsCmd.AddCommand(costAccountsListCmd)
	costBudgetsCmd.AddCommand(costBudgetsListCmd)
	costAllocationRulesCmd.AddCommand(costAllocationRulesListCmd)
	costTagPipelinesCmd.AddCommand(costTagPipelinesListCmd)
	costCustomCostsCmd.AddCommand(costCustomCostsListCmd)

	costAnalyzeCmd.Flags().String("query", "", "Full cloud_cost query; overrides --metric, --filter, and --group-by")
	costAnalyzeCmd.Flags().String("metric", "all.cost", "Cloud cost metric, e.g. all.cost or aws.cost.amortized")
	costAnalyzeCmd.Flags().String("filter", "*", "Metric filter inside braces")
	costAnalyzeCmd.Flags().String("group-by", "service", "Tag to group by; empty disables grouping")
	costAnalyzeCmd.Flags().String("from", "now-30d", "Start time, e.g. now-30d or RFC3339")
	costAnalyzeCmd.Flags().String("to", "now-2d", "End time; default skips incomplete recent cost days")
	costAnalyzeCmd.Flags().Int32("limit", 25, "Maximum grouped values to return")
	costAnalyzeCmd.Flags().String("aggregator", "sum", "Metric aggregator: sum, avg, min, max, last, percentile, mean, l2norm, or area")

	costAccountsListCmd.Flags().String("provider", "all", "Provider to list: all, aws, azure, or gcp")
	costCustomCostsListCmd.Flags().Int64("page-number", 0, "Custom costs files page number")
	costCustomCostsListCmd.Flags().Int64("page-size", 100, "Custom costs files page size")
	costCustomCostsListCmd.Flags().String("status", "", "Filter custom cost files by status")
	costCustomCostsListCmd.Flags().String("name", "", "Filter custom cost files by name")
	costCustomCostsListCmd.Flags().StringSlice("provider", nil, "Filter custom cost files by provider")
}

func runCostAnalyze(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	metric, _ := cmd.Flags().GetString("metric")
	filter, _ := cmd.Flags().GetString("filter")
	groupBy, _ := cmd.Flags().GetString("group-by")
	fromValue, _ := cmd.Flags().GetString("from")
	toValue, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt32("limit")
	aggregatorValue, _ := cmd.Flags().GetString("aggregator")
	if query == "" {
		query = buildCostQuery(metric, filter, groupBy)
	}
	aggregator, err := datadogV2.NewMetricsAggregatorFromValue(aggregatorValue)
	if err != nil {
		return err
	}
	now := time.Now()
	from, err := timeutil.UnixMillis(fromValue, now)
	if err != nil {
		return err
	}
	to, err := timeutil.UnixMillis(toValue, now)
	if err != nil {
		return err
	}

	metricQuery := datadogV2.NewMetricsScalarQuery(*aggregator, datadogV2.METRICSDATASOURCE_CLOUD_COST, query)
	metricQuery.Name = datadog.PtrString("cost")
	formula := datadogV2.NewQueryFormula("cost")
	formula.Limit = &datadogV2.FormulaLimit{
		Count: datadog.PtrInt32(limit),
		Order: datadogV2.QUERYSORTORDER_DESC.Ptr(),
	}
	body := *datadogV2.NewScalarFormulaQueryRequest(*datadogV2.NewScalarFormulaRequest(
		*datadogV2.NewScalarFormulaRequestAttributes(from, []datadogV2.ScalarQuery{
			datadogV2.MetricsScalarQueryAsScalarQuery(metricQuery),
		}, to),
		datadogV2.SCALARFORMULAREQUESTTYPE_SCALAR_REQUEST,
	))
	body.Data.Attributes.Formulas = []datadogV2.QueryFormula{*formula}

	api := datadogV2.NewMetricsApi(client.API)
	resp, httpResp, err := api.QueryScalarData(client.Context, body)
	if err != nil {
		return apiError("cost analyze", httpResp, err)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost analyze", "filter": query},
		meta(client.Site, map[string]any{
			"from":        fromValue,
			"to":          toValue,
			"from_millis": from,
			"to_millis":   to,
			"limit":       limit,
			"data_source": "cloud_cost",
			"metric":      metric,
			"group_by":    groupBy,
			"analysis_hints": []string{
				"Sort the returned groups by descending cost and inspect the largest services, teams, accounts, or regions first.",
				"Rerun with --group-by subaccountname, region, team, env, or owner to isolate accountable spend.",
				"Compare aws.cost.amortized or provider-specific metrics when all.cost is too broad for resource-level tags.",
			},
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func runCostAccountsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	provider, _ := cmd.Flags().GetString("provider")
	provider = strings.ToLower(provider)
	api := datadogV2.NewCloudCostManagementApi(client.API)
	data := map[string]any{}
	status := map[string]int{}
	switch provider {
	case "all", "aws":
		resp, httpResp, err := api.ListCostAWSCURConfigs(client.Context)
		if err != nil {
			return apiError("cost accounts list aws", httpResp, err)
		}
		data["aws"] = resp
		status["aws"] = httpResp.StatusCode
	case "azure", "gcp":
	default:
		return fmt.Errorf("--provider must be one of all, aws, azure, or gcp")
	}
	switch provider {
	case "all", "azure":
		resp, httpResp, err := api.ListCostAzureUCConfigs(client.Context)
		if err != nil {
			return apiError("cost accounts list azure", httpResp, err)
		}
		data["azure"] = resp
		status["azure"] = httpResp.StatusCode
	}
	switch provider {
	case "all", "gcp":
		resp, httpResp, err := api.ListCostGCPUsageCostConfigs(client.Context)
		if err != nil {
			return apiError("cost accounts list gcp", httpResp, err)
		}
		data["gcp"] = resp
		status["gcp"] = httpResp.StatusCode
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost accounts list", "filter": provider},
		meta(client.Site, map[string]any{
			"provider":      provider,
			"http_statuses": status,
		}, nil),
		data,
		outputOptions(),
	)
}

func runCostBudgetsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV2.NewCloudCostManagementApi(client.API)
	resp, httpResp, err := api.ListBudgets(client.Context)
	if err != nil {
		return apiError("cost budgets list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost budgets list"},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func runCostAllocationRulesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV2.NewCloudCostManagementApi(client.API)
	resp, httpResp, err := api.ListCustomAllocationRules(client.Context)
	if err != nil {
		return apiError("cost allocation-rules list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost allocation-rules list"},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func runCostTagPipelinesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV2.NewCloudCostManagementApi(client.API)
	resp, httpResp, err := api.ListTagPipelinesRulesets(client.Context)
	if err != nil {
		return apiError("cost tag-pipelines list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost tag-pipelines list"},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func runCostCustomCostsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	pageNumber, _ := cmd.Flags().GetInt64("page-number")
	pageSize, _ := cmd.Flags().GetInt64("page-size")
	status, _ := cmd.Flags().GetString("status")
	name, _ := cmd.Flags().GetString("name")
	providers, _ := cmd.Flags().GetStringSlice("provider")
	params := datadogV2.NewListCustomCostsFilesOptionalParameters()
	if pageNumber > 0 {
		params.WithPageNumber(pageNumber)
	}
	if pageSize > 0 {
		params.WithPageSize(pageSize)
	}
	if status != "" {
		params.WithFilterStatus(status)
	}
	if name != "" {
		params.WithFilterName(name)
	}
	if len(providers) > 0 {
		params.WithFilterProvider(providers)
	}
	api := datadogV2.NewCloudCostManagementApi(client.API)
	resp, httpResp, err := api.ListCustomCostsFiles(client.Context, *params)
	if err != nil {
		return apiError("cost custom-costs list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "cost custom-costs list"},
		meta(client.Site, map[string]any{
			"page_number": pageNumber,
			"page_size":   pageSize,
			"status":      status,
			"name":        name,
			"provider":    providers,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func buildCostQuery(metric string, filter string, groupBy string) string {
	metric = strings.TrimSpace(metric)
	filter = strings.TrimSpace(filter)
	groupBy = strings.TrimSpace(groupBy)
	if metric == "" {
		metric = "all.cost"
	}
	if filter == "" {
		filter = "*"
	}
	query := fmt.Sprintf("sum:%s{%s}", metric, filter)
	if groupBy != "" {
		query += fmt.Sprintf(" by {%s}", groupBy)
	}
	return query
}

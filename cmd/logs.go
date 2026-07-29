package cmd

import (
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Search logs and read log configuration",
}

var logsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search logs with Datadog log query syntax",
	Example: `  ddcli logs search --query 'service:web error' --from now-15m --to now --limit 50
  ddcli logs search --query '*' --index main --pretty`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runLogsSearch,
}

var logsIndexesCmd = &cobra.Command{
	Use:   "indexes",
	Short: "Inspect log indexes and exclusion filters",
}

var logsIndexesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List log indexes in evaluation order",
	RunE:  runLogsIndexesList,
}

var logsIndexesGetCmd = &cobra.Command{
	Use:   "get <index-name>",
	Short: "Get a log index including exclusion filters",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogsIndexesGet,
}

var logsIndexesOrderCmd = &cobra.Command{
	Use:   "order",
	Short: "Get authoritative log index routing order",
	RunE:  runLogsIndexesOrder,
}

var logsPipelinesCmd = &cobra.Command{
	Use:   "pipelines",
	Short: "Inspect log processing pipelines",
}

var logsPipelinesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List log pipelines in evaluation order",
	RunE:  runLogsPipelinesList,
}

var logsPipelinesGetCmd = &cobra.Command{
	Use:   "get <pipeline-id>",
	Short: "Get a log pipeline including processors",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogsPipelinesGet,
}

var logsPipelinesOrderCmd = &cobra.Command{
	Use:   "order",
	Short: "Get authoritative log pipeline order",
	RunE:  runLogsPipelinesOrder,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsSearchCmd, logsIndexesCmd, logsPipelinesCmd)
	logsIndexesCmd.AddCommand(logsIndexesListCmd, logsIndexesGetCmd, logsIndexesOrderCmd)
	logsPipelinesCmd.AddCommand(logsPipelinesListCmd, logsPipelinesGetCmd, logsPipelinesOrderCmd)

	logsSearchCmd.Flags().String("query", "", "Log search query")
	logsSearchCmd.Flags().String("from", "now-15m", "Start time, e.g. now-15m or RFC3339")
	logsSearchCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	logsSearchCmd.Flags().Int32("limit", 50, "Maximum logs to return")
	logsSearchCmd.Flags().String("cursor", "", "Pagination cursor from a previous response")
	logsSearchCmd.Flags().StringSlice("index", nil, "Log indexes to search; repeat or comma-separate")
	logsIndexesListCmd.Flags().String("query", "", "Client-side filter for index name")
	logsPipelinesListCmd.Flags().String("query", "", "Client-side filter for pipeline name or ID")
}

func runLogsIndexesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	api := datadogV1.NewLogsIndexesApi(client.API)
	indexes, response, err := api.ListLogIndexes(client.Context)
	if err != nil {
		return apiError("logs indexes list", response, err)
	}
	if query != "" {
		indexes.Indexes = filterLogIndexes(indexes.Indexes, query)
	}
	order, orderResponse, err := api.GetLogsIndexOrder(client.Context)
	if err != nil {
		return apiError("logs indexes order", orderResponse, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs indexes list", "filter": query},
		meta(client.Site, map[string]any{"count": len(indexes.Indexes)}, response),
		map[string]any{"indexes": indexes, "order": order},
		outputOptions(),
	)
}

func runLogsIndexesGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	name := args[0]
	api := datadogV1.NewLogsIndexesApi(client.API)
	index, response, err := api.GetLogsIndex(client.Context, name)
	if err != nil {
		return apiError("logs indexes get", response, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs indexes get", "index_name": name},
		meta(client.Site, nil, response),
		index,
		outputOptions(),
	)
}

func runLogsIndexesOrder(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewLogsIndexesApi(client.API)
	order, response, err := api.GetLogsIndexOrder(client.Context)
	if err != nil {
		return apiError("logs indexes order", response, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs indexes order"},
		meta(client.Site, nil, response),
		order,
		outputOptions(),
	)
}

func runLogsPipelinesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	api := datadogV1.NewLogsPipelinesApi(client.API)
	pipelines, response, err := api.ListLogsPipelines(client.Context)
	if err != nil {
		return apiError("logs pipelines list", response, err)
	}
	if query != "" {
		pipelines = filterLogPipelines(pipelines, query)
	}
	order, orderResponse, err := api.GetLogsPipelineOrder(client.Context)
	if err != nil {
		return apiError("logs pipelines order", orderResponse, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs pipelines list", "filter": query},
		meta(client.Site, map[string]any{"count": len(pipelines)}, response),
		map[string]any{"pipelines": pipelines, "order": order},
		outputOptions(),
	)
}

func runLogsPipelinesGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	id := args[0]
	api := datadogV1.NewLogsPipelinesApi(client.API)
	pipeline, response, err := api.GetLogsPipeline(client.Context, id)
	if err != nil {
		return apiError("logs pipelines get", response, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs pipelines get", "pipeline_id": id},
		meta(client.Site, nil, response),
		pipeline,
		outputOptions(),
	)
}

func runLogsPipelinesOrder(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewLogsPipelinesApi(client.API)
	order, response, err := api.GetLogsPipelineOrder(client.Context)
	if err != nil {
		return apiError("logs pipelines order", response, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs pipelines order"},
		meta(client.Site, nil, response),
		order,
		outputOptions(),
	)
}

func runLogsSearch(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	indexes, _ := cmd.Flags().GetStringSlice("index")

	body := *datadogV2.NewLogsListRequest()
	body.Filter = &datadogV2.LogsQueryFilter{
		From:  datadog.PtrString(from),
		Query: datadog.PtrString(query),
		To:    datadog.PtrString(to),
	}
	if len(indexes) > 0 {
		body.Filter.Indexes = indexes
	}
	body.Page = &datadogV2.LogsListRequestPage{
		Limit: datadog.PtrInt32(limit),
	}
	if cursor != "" {
		body.Page.Cursor = datadog.PtrString(cursor)
	}
	body.Sort = datadogV2.LOGSSORT_TIMESTAMP_DESCENDING.Ptr()

	api := datadogV2.NewLogsApi(client.API)
	resp, httpResp, err := api.ListLogs(client.Context, *datadogV2.NewListLogsOptionalParameters().WithBody(body))
	if err != nil {
		return apiError("logs search", httpResp, err)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs search", "filter": query},
		meta(client.Site, map[string]any{
			"from":    from,
			"to":      to,
			"limit":   limit,
			"cursor":  cursor,
			"indexes": indexes,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func filterLogIndexes(indexes []datadogV1.LogsIndex, query string) []datadogV1.LogsIndex {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]datadogV1.LogsIndex, 0, len(indexes))
	for _, index := range indexes {
		if strings.Contains(strings.ToLower(index.GetName()), query) {
			filtered = append(filtered, index)
		}
	}
	return filtered
}

func filterLogPipelines(pipelines []datadogV1.LogsPipeline, query string) []datadogV1.LogsPipeline {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]datadogV1.LogsPipeline, 0, len(pipelines))
	for _, pipeline := range pipelines {
		if strings.Contains(strings.ToLower(pipeline.GetName()), query) ||
			strings.Contains(strings.ToLower(pipeline.GetId()), query) {
			filtered = append(filtered, pipeline)
		}
	}
	return filtered
}

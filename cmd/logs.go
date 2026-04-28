package cmd

import (
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Search Datadog logs",
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

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsSearchCmd)

	logsSearchCmd.Flags().String("query", "", "Log search query")
	logsSearchCmd.Flags().String("from", "now-15m", "Start time, e.g. now-15m or RFC3339")
	logsSearchCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	logsSearchCmd.Flags().Int32("limit", 50, "Maximum logs to return")
	logsSearchCmd.Flags().String("cursor", "", "Pagination cursor from a previous response")
	logsSearchCmd.Flags().StringSlice("index", nil, "Log indexes to search; repeat or comma-separate")
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

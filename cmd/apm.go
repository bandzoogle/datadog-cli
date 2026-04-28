package cmd

import (
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var apmCmd = &cobra.Command{
	Use:   "apm",
	Short: "Query Datadog APM data",
}

var apmSpansCmd = &cobra.Command{
	Use:   "spans",
	Short: "Search APM spans",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runAPMSpans,
}

func init() {
	rootCmd.AddCommand(apmCmd)
	apmCmd.AddCommand(apmSpansCmd)

	apmSpansCmd.Flags().String("query", "", "Span search query")
	apmSpansCmd.Flags().String("from", "now-15m", "Start time, e.g. now-15m or RFC3339")
	apmSpansCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	apmSpansCmd.Flags().Int32("limit", 25, "Maximum spans to return")
	apmSpansCmd.Flags().String("cursor", "", "Pagination cursor from a previous response")
}

func runAPMSpans(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	request := datadogV2.SpansListRequest{
		Data: &datadogV2.SpansListRequestData{
			Attributes: &datadogV2.SpansListRequestAttributes{
				Filter: &datadogV2.SpansQueryFilter{
					From:  datadog.PtrString(from),
					Query: datadog.PtrString(query),
					To:    datadog.PtrString(to),
				},
				Page: &datadogV2.SpansListRequestPage{
					Limit: datadog.PtrInt32(limit),
				},
				Sort: datadogV2.SPANSSORT_TIMESTAMP_DESCENDING.Ptr(),
			},
			Type: datadogV2.SPANSLISTREQUESTTYPE_SEARCH_REQUEST.Ptr(),
		},
	}
	if cursor != "" {
		request.Data.Attributes.Page.Cursor = datadog.PtrString(cursor)
	}

	api := datadogV2.NewSpansApi(client.API)
	resp, httpResp, err := api.ListSpans(client.Context, request)
	if err != nil {
		return apiError("apm spans", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "apm spans", "filter": query},
		meta(client.Site, map[string]any{
			"from":   from,
			"to":     to,
			"limit":  limit,
			"cursor": cursor,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

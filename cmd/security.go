package cmd

import (
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/bandzoogle/datadog-cli/internal/timeutil"
	"github.com/spf13/cobra"
)

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Query Datadog Cloud SIEM security data",
}

var securitySignalsCmd = &cobra.Command{
	Use:   "signals",
	Short: "Search and retrieve Cloud SIEM security signals",
}

var securitySignalsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search Cloud SIEM security signals",
	Example: `  ddcli security signals search --query 'team:bandzoogle' --from now-1h --to now
  ddcli security signals search --query 'status:high' --limit 25 --pretty`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runSecuritySignalsSearch,
}

var securitySignalsGetCmd = &cobra.Command{
	Use:   "get <signal-id>",
	Short: "Get a Cloud SIEM security signal",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecuritySignalsGet,
}

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.AddCommand(securitySignalsCmd)
	securitySignalsCmd.AddCommand(securitySignalsSearchCmd, securitySignalsGetCmd)

	securitySignalsSearchCmd.Flags().String("query", "", "Security signal search query")
	securitySignalsSearchCmd.Flags().String("from", "now-1h", "Start time, e.g. now-1h or RFC3339")
	securitySignalsSearchCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	securitySignalsSearchCmd.Flags().Int32("limit", 50, "Maximum signals to return")
	securitySignalsSearchCmd.Flags().String("cursor", "", "Pagination cursor from a previous response")
}

func runSecuritySignalsSearch(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	fromValue, _ := cmd.Flags().GetString("from")
	toValue, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	now := time.Now()
	from, err := timeutil.Parse(fromValue, now)
	if err != nil {
		return err
	}
	to, err := timeutil.Parse(toValue, now)
	if err != nil {
		return err
	}

	body := buildSecuritySignalsSearchRequest(query, from, to, limit, cursor)
	api := datadogV2.NewSecurityMonitoringApi(client.API)
	resp, httpResp, err := api.SearchSecurityMonitoringSignals(
		client.Context,
		*datadogV2.NewSearchSecurityMonitoringSignalsOptionalParameters().WithBody(body),
	)
	if err != nil {
		return apiError("security signals search", httpResp, err)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "security signals search", "filter": query},
		meta(client.Site, map[string]any{
			"from":   fromValue,
			"to":     toValue,
			"limit":  limit,
			"cursor": cursor,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func buildSecuritySignalsSearchRequest(query string, from, to time.Time, limit int32, cursor string) datadogV2.SecurityMonitoringSignalListRequest {
	body := *datadogV2.NewSecurityMonitoringSignalListRequest()
	body.Filter = &datadogV2.SecurityMonitoringSignalListRequestFilter{
		From:  datadog.PtrTime(from),
		Query: datadog.PtrString(query),
		To:    datadog.PtrTime(to),
	}
	body.Page = &datadogV2.SecurityMonitoringSignalListRequestPage{
		Limit: datadog.PtrInt32(limit),
	}
	if cursor != "" {
		body.Page.Cursor = datadog.PtrString(cursor)
	}
	body.Sort = datadogV2.SECURITYMONITORINGSIGNALSSORT_TIMESTAMP_DESCENDING.Ptr()
	return body
}

func runSecuritySignalsGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	signalID := args[0]

	api := datadogV2.NewSecurityMonitoringApi(client.API)
	resp, httpResp, err := api.GetSecurityMonitoringSignal(client.Context, signalID)
	if err != nil {
		return apiError("security signals get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "security signals get", "signal_id": signalID},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

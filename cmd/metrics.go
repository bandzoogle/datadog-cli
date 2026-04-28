package cmd

import (
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/bandzoogle/datadog-cli/internal/timeutil"
	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "List and query Datadog metrics",
}

var metricsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Search metric names",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runMetricsList,
}

var metricsQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query metric timeseries",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runMetricsQuery,
}

var metricsMetadataCmd = &cobra.Command{
	Use:   "metadata <metric-name>",
	Short: "Get metric metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runMetricsMetadata,
}

func init() {
	rootCmd.AddCommand(metricsCmd)
	metricsCmd.AddCommand(metricsListCmd, metricsQueryCmd, metricsMetadataCmd)

	metricsListCmd.Flags().String("query", "", "Metric name search query")

	metricsQueryCmd.Flags().String("query", "", "Datadog metric query")
	metricsQueryCmd.Flags().String("from", "now-1h", "Start time, e.g. now-1h or RFC3339")
	metricsQueryCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
}

func runMetricsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")

	api := datadogV1.NewMetricsApi(client.API)
	resp, httpResp, err := api.ListMetrics(client.Context, query)
	if err != nil {
		return apiError("metrics list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "metrics list", "filter": query},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func runMetricsQuery(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	fromValue, _ := cmd.Flags().GetString("from")
	toValue, _ := cmd.Flags().GetString("to")
	now := time.Now()
	from, err := timeutil.UnixSeconds(fromValue, now)
	if err != nil {
		return err
	}
	to, err := timeutil.UnixSeconds(toValue, now)
	if err != nil {
		return err
	}

	api := datadogV1.NewMetricsApi(client.API)
	resp, httpResp, err := api.QueryMetrics(client.Context, from, to, query)
	if err != nil {
		return apiError("metrics query", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "metrics query", "filter": query},
		meta(client.Site, map[string]any{
			"from":      fromValue,
			"to":        toValue,
			"from_unix": from,
			"to_unix":   to,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func runMetricsMetadata(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	name := args[0]

	api := datadogV1.NewMetricsApi(client.API)
	resp, httpResp, err := api.GetMetricMetadata(client.Context, name)
	if err != nil {
		return apiError("metrics metadata", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "metrics metadata", "metric": name},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

package cmd

import (
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var syntheticsCmd = &cobra.Command{
	Use:     "synthetics",
	Aliases: []string{"synthetic"},
	Short:   "List and retrieve Datadog Synthetic tests",
}

var syntheticsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List or search Synthetic tests",
	RunE:  runSyntheticsList,
}

var syntheticsGetCmd = &cobra.Command{
	Use:   "get <public-id>",
	Short: "Get a Synthetic test by public ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runSyntheticsGet,
}

func init() {
	rootCmd.AddCommand(syntheticsCmd)
	syntheticsCmd.AddCommand(syntheticsListCmd, syntheticsGetCmd)

	syntheticsListCmd.Flags().String("query", "", "Search text for Synthetic tests")
	syntheticsListCmd.Flags().Int64("limit", 100, "Maximum tests to return")
	syntheticsListCmd.Flags().Int64("start", 0, "Offset for search results")
	syntheticsListCmd.Flags().Bool("full", false, "Include full test configuration when searching")
}

func runSyntheticsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	limit, _ := cmd.Flags().GetInt64("limit")
	start, _ := cmd.Flags().GetInt64("start")
	full, _ := cmd.Flags().GetBool("full")

	api := datadogV1.NewSyntheticsApi(client.API)
	var data any
	var httpResp *http.Response
	if query != "" || full || start != 0 {
		params := datadogV1.NewSearchTestsOptionalParameters().
			WithText(query).
			WithCount(limit).
			WithStart(start).
			WithIncludeFullConfig(full)
		resp, respHTTP, err := api.SearchTests(client.Context, *params)
		if err != nil {
			return apiError("synthetics search", respHTTP, err)
		}
		data = resp
		httpResp = respHTTP
	} else {
		resp, respHTTP, err := api.ListTests(client.Context, *datadogV1.NewListTestsOptionalParameters().WithPageSize(limit))
		if err != nil {
			return apiError("synthetics list", respHTTP, err)
		}
		data = resp
		httpResp = respHTTP
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "synthetics list", "filter": query},
		meta(client.Site, map[string]any{
			"limit": limit,
			"start": start,
			"full":  full,
		}, httpResp),
		data,
		outputOptions(),
	)
}

func runSyntheticsGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	publicID := args[0]

	api := datadogV1.NewSyntheticsApi(client.API)
	resp, httpResp, err := api.GetTest(client.Context, publicID)
	if err != nil {
		return apiError("synthetics get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "synthetics get", "public_id": publicID},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

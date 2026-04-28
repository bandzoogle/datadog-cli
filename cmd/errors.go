package cmd

import (
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/bandzoogle/datadog-cli/internal/timeutil"
	"github.com/spf13/cobra"
)

var errorsCmd = &cobra.Command{
	Use:     "errors",
	Aliases: []string{"error-tracking"},
	Short:   "Search Datadog Error Tracking issues",
}

var errorsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search Error Tracking issues",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireStringFlag(cmd, "query")
	},
	RunE: runErrorsSearch,
}

var errorsGetCmd = &cobra.Command{
	Use:   "get <issue-id>",
	Short: "Get an Error Tracking issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runErrorsGet,
}

func init() {
	rootCmd.AddCommand(errorsCmd)
	errorsCmd.AddCommand(errorsSearchCmd, errorsGetCmd)

	errorsSearchCmd.Flags().String("query", "", "Error Tracking issue search query")
	errorsSearchCmd.Flags().String("track", "trace", "Error Tracking track: trace, logs, or rum")
	errorsSearchCmd.Flags().String("from", "now-1h", "Start time, e.g. now-1h or RFC3339")
	errorsSearchCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	errorsSearchCmd.Flags().Int("limit", 100, "Documented maximum issues returned by Datadog per request")
	errorsSearchCmd.Flags().StringSlice("include", nil, "Relationships to include, e.g. issue,case,team_owners")
}

func runErrorsSearch(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	trackValue, _ := cmd.Flags().GetString("track")
	fromValue, _ := cmd.Flags().GetString("from")
	toValue, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt("limit")
	includes, _ := cmd.Flags().GetStringSlice("include")
	now := time.Now()
	from, err := timeutil.UnixMillis(fromValue, now)
	if err != nil {
		return err
	}
	to, err := timeutil.UnixMillis(toValue, now)
	if err != nil {
		return err
	}
	track, err := datadogV2.NewIssuesSearchRequestDataAttributesTrackFromValue(trackValue)
	if err != nil {
		return err
	}

	attrs := *datadogV2.NewIssuesSearchRequestDataAttributes(from, query, to)
	attrs.Track = track
	body := *datadogV2.NewIssuesSearchRequest(datadogV2.IssuesSearchRequestData{
		Attributes: attrs,
		Type:       datadogV2.ISSUESSEARCHREQUESTDATATYPE_SEARCH_REQUEST,
	})

	params := datadogV2.NewSearchIssuesOptionalParameters()
	if len(includes) > 0 {
		includeValues := make([]datadogV2.SearchIssuesIncludeQueryParameterItem, 0, len(includes))
		for _, include := range includes {
			value, err := datadogV2.NewSearchIssuesIncludeQueryParameterItemFromValue(include)
			if err != nil {
				return err
			}
			includeValues = append(includeValues, *value)
		}
		params.WithInclude(includeValues)
	}

	api := datadogV2.NewErrorTrackingApi(client.API)
	resp, httpResp, err := api.SearchIssues(client.Context, body, *params)
	if err != nil {
		return apiError("errors search", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "errors search", "filter": query},
		meta(client.Site, map[string]any{
			"from":        fromValue,
			"to":          toValue,
			"from_millis": from,
			"to_millis":   to,
			"track":       trackValue,
			"limit":       limit,
			"include":     includes,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func runErrorsGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	issueID := args[0]

	api := datadogV2.NewErrorTrackingApi(client.API)
	resp, httpResp, err := api.GetIssue(client.Context, issueID, *datadogV2.NewGetIssueOptionalParameters())
	if err != nil {
		return apiError("errors get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "errors get", "issue_id": issueID},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

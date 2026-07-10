package cmd

import (
	"fmt"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/bandzoogle/datadog-cli/internal/appsec"
	"github.com/bandzoogle/datadog-cli/internal/dd"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var appsecCmd = &cobra.Command{
	Use:   "appsec",
	Short: "Application Security (ASM / in-app WAF) read-only queries",
	Long: `Read-only Datadog Application Security configuration and blocked-rule summaries.

Custom rules and exclusion filters require appsec_protect_read on the DD_APP_KEY.
Blocked-rule summaries use APM spans (apm_read) and work without appsec_protect_read.`,
}

var appsecCustomRulesCmd = &cobra.Command{
	Use:   "custom-rules",
	Short: "List or get WAF custom rules",
}

var appsecCustomRulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all WAF custom rules",
	RunE:  runAppsecCustomRulesList,
}

var appsecCustomRulesGetCmd = &cobra.Command{
	Use:   "get RULE_ID",
	Short: "Get a WAF custom rule by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppsecCustomRulesGet,
}

var appsecExclusionFiltersCmd = &cobra.Command{
	Use:     "exclusion-filters",
	Aliases: []string{"exclusions", "passlist"},
	Short:   "List or get WAF exclusion filters (passlist entries)",
}

var appsecExclusionFiltersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all WAF exclusion filters",
	RunE:  runAppsecExclusionFiltersList,
}

var appsecExclusionFiltersGetCmd = &cobra.Command{
	Use:   "get FILTER_ID",
	Short: "Get a WAF exclusion filter by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppsecExclusionFiltersGet,
}

var appsecBlockedRulesCmd = &cobra.Command{
	Use:   "blocked-rules",
	Short: "Summarize blocked AppSec rules from APM spans",
}

var appsecBlockedRulesSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize blocked AppSec rules from APM spans",
	Long: `Aggregate Datadog in-app WAF rule hits from blocked AppSec spans.

Uses the Spans API (apm_read). Default query:
  @appsec.blocked:true env:production service:web`,
	RunE: runAppsecBlockedRulesSummary,
}

func init() {
	rootCmd.AddCommand(appsecCmd)
	appsecCmd.AddCommand(appsecCustomRulesCmd)
	appsecCmd.AddCommand(appsecExclusionFiltersCmd)
	appsecCmd.AddCommand(appsecBlockedRulesCmd)
	appsecBlockedRulesCmd.AddCommand(appsecBlockedRulesSummaryCmd)

	appsecCustomRulesCmd.AddCommand(appsecCustomRulesListCmd)
	appsecCustomRulesCmd.AddCommand(appsecCustomRulesGetCmd)
	appsecExclusionFiltersCmd.AddCommand(appsecExclusionFiltersListCmd)
	appsecExclusionFiltersCmd.AddCommand(appsecExclusionFiltersGetCmd)

	appsecBlockedRulesSummaryCmd.Flags().String("query", "@appsec.blocked:true env:production service:web", "Span search query")
	appsecBlockedRulesSummaryCmd.Flags().String("from", "now-7d", "Start time, e.g. now-7d or RFC3339")
	appsecBlockedRulesSummaryCmd.Flags().String("to", "now", "End time, e.g. now or RFC3339")
	appsecBlockedRulesSummaryCmd.Flags().Int32("limit", 200, "Maximum spans to scan")
	appsecBlockedRulesSummaryCmd.Flags().String("cursor", "", "Pagination cursor from a previous response")
}

func runAppsecCustomRulesList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV2.NewApplicationSecurityApi(client.API)
	resp, httpResp, err := api.ListApplicationSecurityWAFCustomRules(client.Context)
	if err != nil {
		return appsecAPIError("appsec custom-rules list", httpResp, err)
	}
	count := 0
	if resp.Data != nil {
		count = len(resp.Data)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "appsec custom-rules list"},
		meta(client.Site, map[string]any{"count": count}, httpResp),
		resp,
		outputOptions(),
	)
}

func runAppsecCustomRulesGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	ruleID := args[0]
	api := datadogV2.NewApplicationSecurityApi(client.API)
	resp, httpResp, err := api.GetApplicationSecurityWafCustomRule(client.Context, ruleID)
	if err != nil {
		return appsecAPIError("appsec custom-rules get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "appsec custom-rules get", "rule_id": ruleID},
		meta(client.Site, map[string]any{"rule_id": ruleID}, httpResp),
		resp,
		outputOptions(),
	)
}

func runAppsecExclusionFiltersList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV2.NewApplicationSecurityApi(client.API)
	resp, httpResp, err := api.ListApplicationSecurityWafExclusionFilters(client.Context)
	if err != nil {
		return appsecAPIError("appsec exclusion-filters list", httpResp, err)
	}
	count := 0
	if resp.Data != nil {
		count = len(resp.Data)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "appsec exclusion-filters list"},
		meta(client.Site, map[string]any{"count": count}, httpResp),
		resp,
		outputOptions(),
	)
}

func runAppsecExclusionFiltersGet(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	filterID := args[0]
	api := datadogV2.NewApplicationSecurityApi(client.API)
	resp, httpResp, err := api.GetApplicationSecurityWafExclusionFilter(client.Context, filterID)
	if err != nil {
		return appsecAPIError("appsec exclusion-filters get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "appsec exclusion-filters get", "filter_id": filterID},
		meta(client.Site, map[string]any{"filter_id": filterID}, httpResp),
		resp,
		outputOptions(),
	)
}

func runAppsecBlockedRulesSummary(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	query, _ := cmd.Flags().GetString("query")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	spans, httpResp, nextCursor, err := listSpans(client, query, from, to, limit, cursor)
	if err != nil {
		return apiError("appsec blocked-rules summary", httpResp, err)
	}

	summary := appsec.AggregateBlockedRules(spans)
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "appsec blocked-rules summary", "filter": query},
		meta(client.Site, map[string]any{
			"from":          from,
			"to":            to,
			"limit":         limit,
			"cursor":        cursor,
			"next_cursor":   nextCursor,
			"spans_scanned": summary.SpansScanned,
			"unique_rules":  len(summary.Rules),
		}, httpResp),
		summary,
		outputOptions(),
	)
}

func listSpans(client *dd.Client, query, from, to string, limit int32, cursor string) ([]datadogV2.Span, *http.Response, string, error) {
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
		return nil, httpResp, "", err
	}

	spans := []datadogV2.Span{}
	if resp.Data != nil {
		spans = resp.Data
	}

	nextCursor := ""
	if resp.Meta != nil && resp.Meta.Page != nil && resp.Meta.Page.After != nil {
		nextCursor = *resp.Meta.Page.After
	}
	return spans, httpResp, nextCursor, nil
}

func appsecAPIError(operation string, resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	if resp != nil && resp.StatusCode == 403 {
		return fmt.Errorf("%s failed: %w (http 403: grant appsec_protect_read to the DD_APP_KEY owner; see ddcli scopes --command appsec)", operation, err)
	}
	return apiError(operation, resp, err)
}

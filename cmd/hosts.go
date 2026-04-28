package cmd

import (
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/bandzoogle/datadog-cli/internal/timeutil"
	"github.com/spf13/cobra"
)

var hostsCmd = &cobra.Command{
	Use:     "hosts",
	Aliases: []string{"host"},
	Short:   "Search Datadog hosts and host totals",
}

var hostsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hosts by name, alias, or tag",
	Long: `List hosts by name, alias, or tag.

Hosts live within the past 3 hours are included by default. Datadog retains
hosts for 7 days, and results are paginated with a maximum of 1000 per page.`,
	RunE: runHostsList,
}

var hostsTotalsCmd = &cobra.Command{
	Use:   "totals",
	Short: "Get active and up host totals",
	RunE:  runHostsTotals,
}

func init() {
	rootCmd.AddCommand(hostsCmd)
	hostsCmd.AddCommand(hostsListCmd, hostsTotalsCmd)

	hostsListCmd.Flags().String("filter", "", "Search by host name, alias, or tag")
	hostsListCmd.Flags().Int64("start", 0, "Pagination offset")
	hostsListCmd.Flags().Int64("limit", 100, "Maximum hosts to return, up to 1000")
	hostsListCmd.Flags().String("from", "", "Unix seconds, RFC3339, or relative time; defaults to Datadog's recent hosts window")
	hostsListCmd.Flags().String("sort-field", "", "Sort field, e.g. status or apps")
	hostsListCmd.Flags().String("sort-dir", "", "Sort direction: asc or desc")
	hostsListCmd.Flags().Bool("include-muted", false, "Include muted hosts data")
	hostsListCmd.Flags().Bool("include-metadata", false, "Include host metadata")

	hostsTotalsCmd.Flags().String("from", "", "Unix seconds, RFC3339, or relative time; defaults to Datadog's recent hosts window")
}

func runHostsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	filter, _ := cmd.Flags().GetString("filter")
	start, _ := cmd.Flags().GetInt64("start")
	limit, _ := cmd.Flags().GetInt64("limit")
	fromValue, _ := cmd.Flags().GetString("from")
	sortField, _ := cmd.Flags().GetString("sort-field")
	sortDir, _ := cmd.Flags().GetString("sort-dir")
	includeMuted, _ := cmd.Flags().GetBool("include-muted")
	includeMetadata, _ := cmd.Flags().GetBool("include-metadata")

	params := datadogV1.NewListHostsOptionalParameters().
		WithStart(start).
		WithCount(limit)
	if filter != "" {
		params.WithFilter(filter)
	}
	if sortField != "" {
		params.WithSortField(sortField)
	}
	if sortDir != "" {
		params.WithSortDir(sortDir)
	}
	if includeMuted {
		params.WithIncludeMutedHostsData(true)
	}
	if includeMetadata {
		params.WithIncludeHostsMetadata(true)
	}
	var from int64
	if fromValue != "" {
		from, err = timeutil.UnixSeconds(fromValue, time.Now())
		if err != nil {
			return err
		}
		params.WithFrom(from)
	}

	api := datadogV1.NewHostsApi(client.API)
	resp, httpResp, err := api.ListHosts(client.Context, *params)
	if err != nil {
		return apiError("hosts list", httpResp, err)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "hosts list", "filter": filter},
		meta(client.Site, map[string]any{
			"start":            start,
			"limit":            limit,
			"from":             fromValue,
			"from_unix":        from,
			"sort_field":       sortField,
			"sort_dir":         sortDir,
			"include_muted":    includeMuted,
			"include_metadata": includeMetadata,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func runHostsTotals(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	fromValue, _ := cmd.Flags().GetString("from")
	params := datadogV1.NewGetHostTotalsOptionalParameters()
	var from int64
	if fromValue != "" {
		from, err = timeutil.UnixSeconds(fromValue, time.Now())
		if err != nil {
			return err
		}
		params.WithFrom(from)
	}

	api := datadogV1.NewHostsApi(client.API)
	resp, httpResp, err := api.GetHostTotals(client.Context, *params)
	if err != nil {
		return apiError("hosts totals", httpResp, err)
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "hosts totals"},
		meta(client.Site, map[string]any{
			"from":      fromValue,
			"from_unix": from,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

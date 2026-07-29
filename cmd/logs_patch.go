package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

type logExclusionPatch struct {
	IndexName    string                         `json:"index_name"`
	Replacements []logExclusionQueryReplacement `json:"replacements"`
}

type logExclusionQueryReplacement struct {
	Name             string `json:"name"`
	ExpectedQuery    string `json:"expected_query"`
	ReplacementQuery string `json:"replacement_query"`
}

type logExclusionQueryChange struct {
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
}

var logsIndexesPatchExclusionsCmd = &cobra.Command{
	Use:   "patch-exclusions <patch.json>",
	Short: "Apply preconditioned exclusion-query replacements",
	Long: `Fetch an index, require every named exclusion to have its exact expected
query, then replace only those query strings in one index update request.

The command rejects missing, duplicate, stale, empty, and no-op replacements.
All unrelated index properties and exclusion fields are preserved.

The guarded read requires the Logs Configuration Read RBAC permission
(logs_read_config). The V1 index update requires Logs Modify Indexes
(logs_modify_indexes). Datadog does not offer these Logs RBAC permissions as
OAuth client scopes.`,
	Args: cobra.ExactArgs(1),
	RunE: runLogsIndexesPatchExclusions,
}

func init() {
	logsIndexesCmd.AddCommand(logsIndexesPatchExclusionsCmd)
	logsIndexesPatchExclusionsCmd.Flags().Bool("dry-run", false, "Validate and print the exact diff without updating Datadog")
}

func runLogsIndexesPatchExclusions(cmd *cobra.Command, args []string) error {
	patch, err := loadLogExclusionPatch(args[0])
	if err != nil {
		return err
	}
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewLogsIndexesApi(client.API)
	current, getResponse, err := api.GetLogsIndex(client.Context, patch.IndexName)
	if err != nil {
		return apiError("logs indexes patch-exclusions get", getResponse, err)
	}
	updatedFilters, changes, err := applyLogExclusionPatch(current.ExclusionFilters, patch)
	if err != nil {
		return err
	}
	invariants := logIndexInvariants(current)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "logs indexes patch-exclusions", "file": args[0]},
			meta(client.Site, map[string]any{"action": "dry-run"}, getResponse),
			map[string]any{
				"index_name": patch.IndexName,
				"changes":    changes,
				"invariants": invariants,
			},
			outputOptions(),
		)
	}

	update := logIndexUpdateRequest(current, updatedFilters)
	result, updateResponse, err := api.UpdateLogsIndex(client.Context, patch.IndexName, update)
	if err != nil {
		return apiError("logs indexes patch-exclusions update", updateResponse, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs indexes patch-exclusions", "file": args[0]},
		meta(client.Site, map[string]any{"action": "update"}, updateResponse),
		map[string]any{
			"index_name": patch.IndexName,
			"changes":    changes,
			"invariants": invariants,
			"index":      result,
		},
		outputOptions(),
	)
}

func loadLogExclusionPatch(path string) (logExclusionPatch, error) {
	var patch logExclusionPatch
	body, err := os.ReadFile(path)
	if err != nil {
		return patch, fmt.Errorf("read log exclusion patch: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		return patch, fmt.Errorf("parse log exclusion patch: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return patch, fmt.Errorf("parse log exclusion patch: trailing JSON content")
	}
	if strings.TrimSpace(patch.IndexName) == "" {
		return patch, fmt.Errorf("index_name is required")
	}
	if len(patch.Replacements) == 0 {
		return patch, fmt.Errorf("at least one replacement is required")
	}
	return patch, nil
}

func applyLogExclusionPatch(
	exclusions []datadogV1.LogsExclusion,
	patch logExclusionPatch,
) ([]datadogV1.LogsExclusion, []logExclusionQueryChange, error) {
	replacements := make(map[string]logExclusionQueryReplacement, len(patch.Replacements))
	for _, replacement := range patch.Replacements {
		if strings.TrimSpace(replacement.Name) == "" {
			return nil, nil, fmt.Errorf("replacement name is required")
		}
		if replacement.ExpectedQuery == "" || replacement.ReplacementQuery == "" {
			return nil, nil, fmt.Errorf("replacement %q requires non-empty expected_query and replacement_query", replacement.Name)
		}
		if replacement.ExpectedQuery == replacement.ReplacementQuery {
			return nil, nil, fmt.Errorf("replacement %q is a no-op", replacement.Name)
		}
		if _, exists := replacements[replacement.Name]; exists {
			return nil, nil, fmt.Errorf("duplicate replacement for exclusion %q", replacement.Name)
		}
		replacements[replacement.Name] = replacement
	}

	counts := make(map[string]int, len(replacements))
	for _, exclusion := range exclusions {
		if _, exists := replacements[exclusion.Name]; exists {
			counts[exclusion.Name]++
		}
	}
	for name := range replacements {
		switch counts[name] {
		case 0:
			return nil, nil, fmt.Errorf("exclusion %q is missing", name)
		case 1:
		default:
			return nil, nil, fmt.Errorf("exclusion %q appears %d times", name, counts[name])
		}
	}

	updated := append([]datadogV1.LogsExclusion(nil), exclusions...)
	changes := make([]logExclusionQueryChange, 0, len(replacements))
	for i, exclusion := range updated {
		replacement, exists := replacements[exclusion.Name]
		if !exists {
			continue
		}
		filter, ok := exclusion.GetFilterOk()
		if !ok {
			return nil, nil, fmt.Errorf("exclusion %q has no filter", exclusion.Name)
		}
		query, ok := filter.GetQueryOk()
		if !ok {
			return nil, nil, fmt.Errorf("exclusion %q has no query", exclusion.Name)
		}
		if *query != replacement.ExpectedQuery {
			return nil, nil, fmt.Errorf(
				"stale exclusion %q: expected query %q, got %q",
				exclusion.Name,
				replacement.ExpectedQuery,
				*query,
			)
		}
		filterCopy := *filter
		filterCopy.SetQuery(replacement.ReplacementQuery)
		exclusion.SetFilter(filterCopy)
		updated[i] = exclusion
		changes = append(changes, logExclusionQueryChange{
			Name:   exclusion.Name,
			Before: replacement.ExpectedQuery,
			After:  replacement.ReplacementQuery,
		})
	}
	return updated, changes, nil
}

func logIndexUpdateRequest(
	index datadogV1.LogsIndex,
	exclusions []datadogV1.LogsExclusion,
) datadogV1.LogsIndexUpdateRequest {
	request := datadogV1.NewLogsIndexUpdateRequest(index.Filter)
	request.SetExclusionFilters(exclusions)
	if value, ok := index.GetDailyLimitOk(); ok {
		request.SetDailyLimit(*value)
	}
	if value, ok := index.GetDailyLimitResetOk(); ok {
		request.SetDailyLimitReset(*value)
	}
	if value, ok := index.GetDailyLimitWarningThresholdPercentageOk(); ok {
		request.SetDailyLimitWarningThresholdPercentage(*value)
	}
	if value, ok := index.GetNumRetentionDaysOk(); ok {
		request.SetNumRetentionDays(*value)
	}
	if value, ok := index.GetNumFlexLogsRetentionDaysOk(); ok {
		request.SetNumFlexLogsRetentionDays(*value)
	}
	if value, ok := index.GetTagsOk(); ok {
		request.SetTags(*value)
	}
	return *request
}

func logIndexInvariants(index datadogV1.LogsIndex) map[string]any {
	return map[string]any{
		"daily_limit":                  optionalInt64(index.GetDailyLimitOk()),
		"filter_query":                 index.Filter.GetQuery(),
		"num_retention_days":           optionalInt64(index.GetNumRetentionDaysOk()),
		"num_flex_logs_retention_days": optionalInt64(index.GetNumFlexLogsRetentionDaysOk()),
		"exclusion_count":              len(index.ExclusionFilters),
	}
}

func optionalInt64(value *int64, ok bool) any {
	if !ok {
		return nil
	}
	return *value
}

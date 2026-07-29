package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func TestApplyLogExclusionPatchChangesOnlyExpectedQueries(t *testing.T) {
	exclusions := []datadogV1.LogsExclusion{
		logExclusion("first", "query one", 1, true),
		logExclusion("unrelated", "leave me", 0.95, false),
		logExclusion("second", "query two", 1, true),
	}
	patch := logExclusionPatch{
		IndexName: "production",
		Replacements: []logExclusionQueryReplacement{
			{Name: "second", ExpectedQuery: "query two", ReplacementQuery: "new two"},
			{Name: "first", ExpectedQuery: "query one", ReplacementQuery: "new one"},
		},
	}

	updated, changes, err := applyLogExclusionPatch(exclusions, patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].Name != "first" || changes[1].Name != "second" {
		t.Fatalf("expected changes in live exclusion order, got %#v", changes)
	}
	firstFilter := updated[0].GetFilter()
	secondFilter := updated[2].GetFilter()
	if firstFilter.GetQuery() != "new one" ||
		secondFilter.GetQuery() != "new two" {
		t.Fatalf("queries were not replaced: %#v", updated)
	}
	if !reflect.DeepEqual(updated[1], exclusions[1]) {
		t.Fatal("unrelated exclusion changed")
	}
	expectedFirst := exclusions[0]
	filter := expectedFirst.GetFilter()
	filter.SetQuery("new one")
	expectedFirst.SetFilter(filter)
	if !reflect.DeepEqual(updated[0], expectedFirst) {
		t.Fatal("changed exclusion fields other than query")
	}
	originalFilter := exclusions[0].GetFilter()
	if originalFilter.GetQuery() != "query one" {
		t.Fatal("input exclusions were mutated")
	}
}

func TestApplyLogExclusionPatchRejectsStaleQuery(t *testing.T) {
	exclusions := []datadogV1.LogsExclusion{
		logExclusion("target", "current", 1, true),
	}
	patch := patchFor("target", "expected", "replacement")

	_, _, err := applyLogExclusionPatch(exclusions, patch)

	if err == nil || !strings.Contains(err.Error(), "stale exclusion") {
		t.Fatalf("expected stale query error, got %v", err)
	}
}

func TestApplyLogExclusionPatchRejectsMissingAndDuplicateLiveFilters(t *testing.T) {
	patch := patchFor("target", "expected", "replacement")

	_, _, missingErr := applyLogExclusionPatch(nil, patch)
	if missingErr == nil || !strings.Contains(missingErr.Error(), "is missing") {
		t.Fatalf("expected missing error, got %v", missingErr)
	}

	exclusions := []datadogV1.LogsExclusion{
		logExclusion("target", "expected", 1, true),
		logExclusion("target", "expected", 1, true),
	}
	_, _, duplicateErr := applyLogExclusionPatch(exclusions, patch)
	if duplicateErr == nil || !strings.Contains(duplicateErr.Error(), "appears 2 times") {
		t.Fatalf("expected duplicate live filter error, got %v", duplicateErr)
	}
}

func TestApplyLogExclusionPatchRejectsDuplicatePatchFilters(t *testing.T) {
	patch := logExclusionPatch{
		IndexName: "production",
		Replacements: []logExclusionQueryReplacement{
			{Name: "target", ExpectedQuery: "one", ReplacementQuery: "two"},
			{Name: "target", ExpectedQuery: "two", ReplacementQuery: "three"},
		},
	}

	_, _, err := applyLogExclusionPatch(nil, patch)

	if err == nil || !strings.Contains(err.Error(), "duplicate replacement") {
		t.Fatalf("expected duplicate replacement error, got %v", err)
	}
}

func TestLogIndexUpdateRequestPreservesUpdateableProperties(t *testing.T) {
	index := datadogV1.NewLogsIndex(*datadogV1.NewLogsFilterWithDefaults(), "production")
	index.Filter.SetQuery("account:production")
	index.SetDailyLimit(500_000_000)
	index.SetDailyLimitWarningThresholdPercentage(80)
	index.SetNumRetentionDays(7)
	index.SetNumFlexLogsRetentionDays(30)
	index.SetTags([]string{"team:server-admin"})
	exclusions := []datadogV1.LogsExclusion{
		logExclusion("target", "replacement", 1, true),
	}

	request := logIndexUpdateRequest(*index, exclusions)

	requestFilter := request.GetFilter()
	if requestFilter.GetQuery() != "account:production" ||
		request.GetDailyLimit() != 500_000_000 ||
		request.GetDailyLimitWarningThresholdPercentage() != 80 ||
		request.GetNumRetentionDays() != 7 ||
		request.GetNumFlexLogsRetentionDays() != 30 ||
		!reflect.DeepEqual(request.GetTags(), []string{"team:server-admin"}) ||
		!reflect.DeepEqual(request.GetExclusionFilters(), exclusions) {
		t.Fatalf("update request did not preserve index properties: %#v", request)
	}
}

func TestLoadLogExclusionPatchIsStrict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patch.json")
	err := os.WriteFile(path, []byte(`{
		"index_name": "production",
		"replacements": [{
			"name": "target",
			"expected_query": "one",
			"replacement_query": "two"
		}],
		"unexpected": true
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = loadLogExclusionPatch(path)

	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict JSON error, got %v", err)
	}
}

func TestLogExclusionPatchHelpNamesRBACPermissions(t *testing.T) {
	help := logsIndexesPatchExclusionsCmd.Long
	if !strings.Contains(help, "logs_read_config") ||
		!strings.Contains(help, "logs_modify_indexes") {
		t.Fatalf("expected exact Logs RBAC permissions in help, got:\n%s", help)
	}
	if strings.Contains(help, "logs_write_config") {
		t.Fatalf("help includes nonexistent permission:\n%s", help)
	}
}

func patchFor(name, expected, replacement string) logExclusionPatch {
	return logExclusionPatch{
		IndexName: "production",
		Replacements: []logExclusionQueryReplacement{
			{Name: name, ExpectedQuery: expected, ReplacementQuery: replacement},
		},
	}
}

func logExclusion(name, query string, sampleRate float64, enabled bool) datadogV1.LogsExclusion {
	filter := datadogV1.NewLogsExclusionFilter(sampleRate)
	filter.SetQuery(query)
	exclusion := datadogV1.NewLogsExclusion(name)
	exclusion.SetFilter(*filter)
	exclusion.SetIsEnabled(enabled)
	return *exclusion
}

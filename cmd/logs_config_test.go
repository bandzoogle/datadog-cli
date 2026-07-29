package cmd

import (
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func TestFilterLogIndexesPreservesOrder(t *testing.T) {
	indexes := []datadogV1.LogsIndex{
		logIndex("bandzoogle-production"),
		logIndex("bandzoogle-development"),
		logIndex("other"),
	}

	got := filterLogIndexes(indexes, "bandzoogle")

	if len(got) != 2 {
		t.Fatalf("expected two indexes, got %d", len(got))
	}
	if got[0].GetName() != "bandzoogle-production" ||
		got[1].GetName() != "bandzoogle-development" {
		t.Fatalf("unexpected filtered order: %#v", got)
	}
}

func TestFilterLogPipelinesMatchesNameOrID(t *testing.T) {
	pipelines := []datadogV1.LogsPipeline{
		logPipeline("abc-123", "OpenResty"),
		logPipeline("def-456", "Rails"),
	}

	byName := filterLogPipelines(pipelines, "openresty")
	if len(byName) != 1 || byName[0].GetId() != "abc-123" {
		t.Fatalf("unexpected name match: %#v", byName)
	}

	byID := filterLogPipelines(pipelines, "DEF")
	if len(byID) != 1 || byID[0].GetName() != "Rails" {
		t.Fatalf("unexpected ID match: %#v", byID)
	}
}

func logIndex(name string) datadogV1.LogsIndex {
	index := datadogV1.NewLogsIndex(*datadogV1.NewLogsFilter(), name)
	return *index
}

func logPipeline(id, name string) datadogV1.LogsPipeline {
	pipeline := datadogV1.NewLogsPipeline(name)
	pipeline.SetId(id)
	return *pipeline
}

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func TestLoadPipelineApply(t *testing.T) {
	path := writePipeline(t, `{
		"name": "Varnish severity override",
		"filter": {"query": "source:varnish"},
		"insert_after_pipeline_id": "4DOu6dYaR46nARsjyjShbg",
		"processors": [
			{
				"type": "category-processor",
				"target": "http.varnish_severity",
				"categories": [
					{"filter": {"query": "@http.status_code:[500 TO 599]"}, "name": "error"},
					{"filter": {"query": "@http.status_code:[200 TO 499]"}, "name": "info"}
				]
			},
			{
				"type": "status-remapper",
				"sources": ["http.varnish_severity"]
			}
		]
	}`)

	pipeline, insertAfter, err := loadPipelineApply(path)
	if err != nil {
		t.Fatalf("load pipeline: %v", err)
	}
	if pipeline.GetName() != "Varnish severity override" {
		t.Fatalf("unexpected name: %q", pipeline.GetName())
	}
	if insertAfter == nil || *insertAfter != "4DOu6dYaR46nARsjyjShbg" {
		t.Fatalf("expected insert_after_pipeline_id to be parsed, got %v", insertAfter)
	}
	if len(pipeline.GetProcessors()) != 2 {
		t.Fatalf("expected 2 processors, got %d", len(pipeline.GetProcessors()))
	}
}

func TestLoadPipelineApplyRequiresName(t *testing.T) {
	// LogsPipeline.Name is a required JSON field in the SDK model, so an
	// absent key fails during unmarshal before our own validation runs.
	path := writePipeline(t, `{
		"filter": {"query": "source:varnish"},
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected a name-related error, got %v", err)
	}
}

func TestLoadPipelineApplyRejectsBlankName(t *testing.T) {
	path := writePipeline(t, `{
		"name": "   ",
		"filter": {"query": "source:varnish"},
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
}

func TestLoadPipelineApplyRequiresFilterQuery(t *testing.T) {
	path := writePipeline(t, `{
		"name": "No filter",
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "filter.query is required") {
		t.Fatalf("expected filter.query error, got %v", err)
	}
}

func TestLoadPipelineApplyRequiresAtLeastOneProcessor(t *testing.T) {
	path := writePipeline(t, `{
		"name": "No processors",
		"filter": {"query": "source:varnish"},
		"processors": []
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "at least one processor") {
		t.Fatalf("expected processor-required error, got %v", err)
	}
}

func TestLoadPipelineApplyRejectsReadOnly(t *testing.T) {
	path := writePipeline(t, `{
		"name": "Varnish",
		"filter": {"query": "source:varnish"},
		"is_read_only": true,
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "is_read_only") {
		t.Fatalf("expected read-only refusal, got %v", err)
	}
}

func TestLoadPipelineApplyRejectsUnsupportedFields(t *testing.T) {
	path := writePipeline(t, `{
		"name": "Typo",
		"filter": {"query": "source:varnish"},
		"processors": [{"type": "status-remapper", "sources": ["status"]}],
		"is_enable": true
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported fields") {
		t.Fatalf("expected unsupported-field error, got %v", err)
	}
}

func TestLoadPipelineApplyRejectsEmptyInsertAfter(t *testing.T) {
	path := writePipeline(t, `{
		"name": "Blank anchor",
		"filter": {"query": "source:varnish"},
		"insert_after_pipeline_id": "",
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, _, err := loadPipelineApply(path)
	if err == nil || !strings.Contains(err.Error(), "insert_after_pipeline_id must not be empty") {
		t.Fatalf("expected empty-anchor error, got %v", err)
	}
}

func TestLoadPipelineApplyWithoutInsertAfter(t *testing.T) {
	path := writePipeline(t, `{
		"name": "No anchor",
		"filter": {"query": "source:varnish"},
		"processors": [{"type": "status-remapper", "sources": ["status"]}]
	}`)

	_, insertAfter, err := loadPipelineApply(path)
	if err != nil {
		t.Fatal(err)
	}
	if insertAfter != nil {
		t.Fatalf("expected nil insert_after_pipeline_id, got %v", *insertAfter)
	}
}

func TestMatchPipelineByName(t *testing.T) {
	pipelines := []datadogV1.LogsPipeline{
		pipelineSummary("p1", "Varnish severity override", false),
		pipelineSummary("p2", "Other pipeline", false),
	}

	id, readOnly, err := matchPipelineByName(pipelines, "Varnish severity override")
	if err != nil {
		t.Fatal(err)
	}
	if id != "p1" || readOnly {
		t.Fatalf("expected p1/not-read-only, got %q/%v", id, readOnly)
	}
}

func TestMatchPipelineByNameNoMatch(t *testing.T) {
	id, readOnly, err := matchPipelineByName(nil, "Missing")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" || readOnly {
		t.Fatalf("expected empty no-match result, got %q/%v", id, readOnly)
	}
}

func TestMatchPipelineByNameRefusesDuplicates(t *testing.T) {
	pipelines := []datadogV1.LogsPipeline{
		pipelineSummary("p1", "Duplicate", false),
		pipelineSummary("p2", "Duplicate", false),
	}

	_, _, err := matchPipelineByName(pipelines, "Duplicate")
	if err == nil || !strings.Contains(err.Error(), "2 pipelines") {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}
}

func TestMatchPipelineByNameReportsReadOnly(t *testing.T) {
	pipelines := []datadogV1.LogsPipeline{
		pipelineSummary("p1", "Varnish", true),
	}

	id, readOnly, err := matchPipelineByName(pipelines, "Varnish")
	if err != nil {
		t.Fatal(err)
	}
	if id != "p1" || !readOnly {
		t.Fatalf("expected read-only match, got %q/%v", id, readOnly)
	}
}

func TestDesiredPipelineOrderInsertsAfterAnchor(t *testing.T) {
	current := []string{"a", "b", "c"}

	desired, err := desiredPipelineOrder(current, "new", "b")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"a", "b", "new", "c"}
	if !equalStringSlices(desired, expected) {
		t.Fatalf("expected %v, got %v", expected, desired)
	}
}

func TestDesiredPipelineOrderMovesExistingPipeline(t *testing.T) {
	current := []string{"new", "a", "b", "c"}

	desired, err := desiredPipelineOrder(current, "new", "b")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"a", "b", "new", "c"}
	if !equalStringSlices(desired, expected) {
		t.Fatalf("expected %v, got %v", expected, desired)
	}
}

func TestDesiredPipelineOrderIsIdempotentWhenAlreadyPlaced(t *testing.T) {
	current := []string{"a", "b", "new", "c"}

	desired, err := desiredPipelineOrder(current, "new", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !equalStringSlices(current, desired) {
		t.Fatalf("expected no-op order, got %v", desired)
	}
}

func TestDesiredPipelineOrderRejectsMissingAnchor(t *testing.T) {
	_, err := desiredPipelineOrder([]string{"a", "b"}, "new", "missing")
	if err == nil || !strings.Contains(err.Error(), "not found in the current pipeline order") {
		t.Fatalf("expected missing-anchor error, got %v", err)
	}
}

func TestDesiredPipelineOrderRejectsSelfAnchor(t *testing.T) {
	_, err := desiredPipelineOrder([]string{"a", "b"}, "a", "a")
	if err == nil || !strings.Contains(err.Error(), "own id") {
		t.Fatalf("expected self-anchor error, got %v", err)
	}
}

func TestEqualStringSlices(t *testing.T) {
	if !equalStringSlices([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal("expected equal slices to match")
	}
	if equalStringSlices([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("expected different order to not match")
	}
	if equalStringSlices([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("expected different length to not match")
	}
}

func writePipeline(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pipeline.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func pipelineSummary(id, name string, readOnly bool) datadogV1.LogsPipeline {
	pipeline := datadogV1.NewLogsPipeline(name)
	pipeline.SetId(id)
	pipeline.SetIsReadOnly(readOnly)
	return *pipeline
}

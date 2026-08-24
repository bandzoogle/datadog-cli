package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/dd"
	"github.com/spf13/cobra"
)

// multistepTestJSON is the shape this command exists for: fetch a page, extract
// a digest-stamped asset URL from the body, then assert the asset itself is
// served with the right status and content type.
const multistepTestJSON = `{
  "name": "Production asset delivery end-to-end",
  "type": "api",
  "subtype": "multi",
  "message": "Asset delivery failed while pages still returned 200.",
  "tags": ["env:production", "service:openresty"],
  "locations": ["aws:us-east-1", "aws:eu-west-1"],
  "config": {
    "steps": [
      {
        "name": "Fetch a member page",
        "subtype": "http",
        "request": {"method": "GET", "url": "https://www.bandzoogle.com/", "timeout": 30},
        "assertions": [{"type": "statusCode", "operator": "is", "target": 200}],
        "extractedValues": [
          {
            "name": "ASSET_URL",
            "type": "http_body",
            "parser": {"type": "regex", "value": "https://[^\"]+/assets/application-[0-9a-f]+\\.css"}
          }
        ]
      },
      {
        "name": "Fetch the digest asset",
        "subtype": "http",
        "request": {"method": "GET", "url": "{{ ASSET_URL }}", "timeout": 30},
        "assertions": [
          {"type": "statusCode", "operator": "is", "target": 200},
          {"type": "header", "property": "content-type", "operator": "contains", "target": "text/css"}
        ]
      }
    ]
  },
  "options": {"tick_every": 300, "min_location_failed": 1, "retry": {"count": 1, "interval": 300}},
  "status": "live"
}`

const simpleTestJSON = `{
  "name": "Production home page",
  "type": "api",
  "subtype": "http",
  "message": "Home page is down.",
  "locations": ["aws:us-east-1"],
  "config": {
    "request": {"method": "GET", "url": "https://www.bandzoogle.com/"},
    "assertions": [{"type": "statusCode", "operator": "is", "target": 200}]
  },
  "options": {"tick_every": 300}
}`

func TestLoadSyntheticsTestAcceptsAMultistepDefinition(t *testing.T) {
	test, err := loadSyntheticsTest(writeSyntheticsTest(t, multistepTestJSON))
	if err != nil {
		t.Fatalf("load Synthetic test: %v", err)
	}
	if test.Name != "Production asset delivery end-to-end" {
		t.Fatalf("unexpected name: %q", test.Name)
	}
	if test.GetSubtype() != datadogV1.SYNTHETICSTESTDETAILSSUBTYPE_MULTI {
		t.Fatalf("unexpected subtype: %q", test.GetSubtype())
	}
	if len(test.Config.Steps) != 2 {
		t.Fatalf("expected two steps, got %d", len(test.Config.Steps))
	}
	for i, step := range test.Config.Steps {
		if step.SyntheticsAPITestStep == nil {
			t.Fatalf("step %d did not decode as an API test step: %#v", i, step)
		}
	}
}

func TestLoadSyntheticsTestAcceptsASimpleAPIDefinition(t *testing.T) {
	test, err := loadSyntheticsTest(writeSyntheticsTest(t, simpleTestJSON))
	if err != nil {
		t.Fatalf("load Synthetic test: %v", err)
	}
	if test.GetSubtype() != datadogV1.SYNTHETICSTESTDETAILSSUBTYPE_HTTP {
		t.Fatalf("unexpected subtype: %q", test.GetSubtype())
	}
	if test.Config.Request == nil || test.Config.Request.GetUrl() == "" {
		t.Fatalf("expected a decoded request, got %#v", test.Config.Request)
	}
}

func TestLoadSyntheticsTestRejectsNestedUnsupportedFields(t *testing.T) {
	body := strings.Replace(multistepTestJSON, `"tick_every": 300`, `"tick_evry": 300`, 1)
	body = strings.Replace(body, `"extractedValues": [`, `"extracedValues": [`, 1)

	_, err := loadSyntheticsTest(writeSyntheticsTest(t, body))
	if err == nil {
		t.Fatal("expected an unsupported-field error")
	}
	for _, want := range []string{"options.tick_evry", "config.steps[0].extracedValues"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error, got %v", want, err)
		}
	}
}

func TestLoadSyntheticsTestRejectsNonAPITests(t *testing.T) {
	body := strings.Replace(simpleTestJSON, `"type": "api"`, `"type": "browser"`, 1)

	_, err := loadSyntheticsTest(writeSyntheticsTest(t, body))
	if err == nil || !strings.Contains(err.Error(), "type must be") {
		t.Fatalf("expected a type error, got %v", err)
	}
}

func TestLoadSyntheticsTestRequiresTheConfigEachSubtypeNeeds(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "multi without steps",
			body: strings.Replace(multistepTestJSON, `"subtype": "multi"`, `"subtype": "http"`, 1),
			want: `config.steps requires subtype "multi"`,
		},
		{
			name: "http without a request",
			body: strings.Replace(simpleTestJSON, `"request": {"method": "GET", "url": "https://www.bandzoogle.com/"},`, "", 1),
			want: "requires config.request",
		},
		{
			name: "missing subtype",
			body: strings.Replace(simpleTestJSON, `"subtype": "http",`, "", 1),
			want: "subtype is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadSyntheticsTest(writeSyntheticsTest(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestExactSyntheticsTestID(t *testing.T) {
	tests := []datadogV1.SyntheticsTestDetailsWithoutSteps{
		syntheticsSummary("abc-def-ghi", "Production asset delivery end-to-end", datadogV1.SYNTHETICSTESTDETAILSTYPE_API),
		syntheticsSummary("jkl-mno-pqr", "Other test", datadogV1.SYNTHETICSTESTDETAILSTYPE_API),
	}

	publicID, err := exactSyntheticsTestID(tests, "Production asset delivery end-to-end")
	if err != nil {
		t.Fatal(err)
	}
	if publicID != "abc-def-ghi" {
		t.Fatalf("expected abc-def-ghi, got %q", publicID)
	}
}

func TestExactSyntheticsTestIDReturnsNoMatchForACreate(t *testing.T) {
	tests := []datadogV1.SyntheticsTestDetailsWithoutSteps{
		syntheticsSummary("jkl-mno-pqr", "Other test", datadogV1.SYNTHETICSTESTDETAILSTYPE_API),
	}

	publicID, err := exactSyntheticsTestID(tests, "Production asset delivery end-to-end")
	if err != nil {
		t.Fatal(err)
	}
	if publicID != "" {
		t.Fatalf("expected no match, got %q", publicID)
	}
}

func TestExactSyntheticsTestIDRefusesDuplicateNames(t *testing.T) {
	tests := []datadogV1.SyntheticsTestDetailsWithoutSteps{
		syntheticsSummary("abc-def-ghi", "Production asset delivery end-to-end", datadogV1.SYNTHETICSTESTDETAILSTYPE_API),
		syntheticsSummary("jkl-mno-pqr", "Production asset delivery end-to-end", datadogV1.SYNTHETICSTESTDETAILSTYPE_API),
	}

	_, err := exactSyntheticsTestID(tests, "Production asset delivery end-to-end")
	if err == nil || !strings.Contains(err.Error(), "2 Synthetic tests") {
		t.Fatalf("expected a duplicate-name error, got %v", err)
	}
}

func TestExactSyntheticsTestIDRefusesABrowserTestWithTheSameName(t *testing.T) {
	tests := []datadogV1.SyntheticsTestDetailsWithoutSteps{
		syntheticsSummary("abc-def-ghi", "Production asset delivery end-to-end", datadogV1.SYNTHETICSTESTDETAILSTYPE_BROWSER),
	}

	_, err := exactSyntheticsTestID(tests, "Production asset delivery end-to-end")
	if err == nil || !strings.Contains(err.Error(), `is a "browser" test`) {
		t.Fatalf("expected a test-type error, got %v", err)
	}
}

func TestSyntheticsApplyCreatesWhenNoTestMatchesTheName(t *testing.T) {
	recorder := &syntheticsRecorder{
		listPages: [][]any{{syntheticsListEntry("jkl-mno-pqr", "Other test", "api")}},
		writeBody: map[string]any{"public_id": "new-tes-tid"},
	}
	out, err := runApply(t, recorder, multistepTestJSON, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	recorder.wantRequests(t,
		"GET /api/v1/synthetics/tests",
		"POST /api/v1/synthetics/tests/api",
	)
	if action := out.Meta["action"]; action != "create" {
		t.Fatalf("expected a create action, got %v", action)
	}
}

func TestSyntheticsApplyUpdatesTheTestNamedByPublicID(t *testing.T) {
	recorder := &syntheticsRecorder{writeBody: map[string]any{"public_id": "abc-def-ghi"}}
	body := strings.Replace(multistepTestJSON,
		`"type": "api",`,
		`"type": "api",
  "public_id": "abc-def-ghi",
  "monitor_id": 316230301,`, 1)

	out, err := runApply(t, recorder, body, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// No listing: a public_id in the file is the match, so nothing is scanned.
	recorder.wantRequests(t, "PUT /api/v1/synthetics/tests/api/abc-def-ghi")
	if out.Meta["action"] != "update" || out.Meta["matched_by"] != "public_id" {
		t.Fatalf("unexpected meta: %#v", out.Meta)
	}
	sent := recorder.writes[0]
	for _, serverOwned := range []string{"public_id", "monitor_id"} {
		if _, present := sent[serverOwned]; present {
			t.Fatalf("%s should be stripped from the request body: %#v", serverOwned, sent)
		}
	}
	if sent["name"] != "Production asset delivery end-to-end" {
		t.Fatalf("request body lost the definition: %#v", sent)
	}
}

func TestSyntheticsApplyUpdatesTheTestMatchedByName(t *testing.T) {
	recorder := &syntheticsRecorder{
		listPages: [][]any{{
			syntheticsListEntry("jkl-mno-pqr", "Other test", "api"),
			syntheticsListEntry("abc-def-ghi", "Production asset delivery end-to-end", "api"),
		}},
		writeBody: map[string]any{"public_id": "abc-def-ghi"},
	}

	out, err := runApply(t, recorder, multistepTestJSON, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	recorder.wantRequests(t,
		"GET /api/v1/synthetics/tests",
		"PUT /api/v1/synthetics/tests/api/abc-def-ghi",
	)
	if out.Meta["action"] != "update" || out.Meta["matched_by"] != "name" {
		t.Fatalf("unexpected meta: %#v", out.Meta)
	}
}

func TestSyntheticsApplyPagesThroughEveryTestBeforeMatchingAName(t *testing.T) {
	firstPage := make([]any, syntheticsListPageSize)
	for i := range firstPage {
		firstPage[i] = syntheticsListEntry(fmt.Sprintf("pad-%03d-xxx", i), fmt.Sprintf("Padding %d", i), "api")
	}
	recorder := &syntheticsRecorder{
		listPages: [][]any{
			firstPage,
			{syntheticsListEntry("abc-def-ghi", "Production asset delivery end-to-end", "api")},
		},
		writeBody: map[string]any{"public_id": "abc-def-ghi"},
	}

	if _, err := runApply(t, recorder, multistepTestJSON, false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	recorder.wantRequests(t,
		"GET /api/v1/synthetics/tests",
		"GET /api/v1/synthetics/tests",
		"PUT /api/v1/synthetics/tests/api/abc-def-ghi",
	)
	if got := recorder.listQueries; !reflect.DeepEqual(got, []string{"0", "1"}) {
		t.Fatalf("expected sequential page numbers, got %#v", got)
	}
}

func TestSyntheticsApplyRefusesAnAmbiguousNameWithoutWriting(t *testing.T) {
	recorder := &syntheticsRecorder{
		listPages: [][]any{{
			syntheticsListEntry("abc-def-ghi", "Production asset delivery end-to-end", "api"),
			syntheticsListEntry("jkl-mno-pqr", "Production asset delivery end-to-end", "api"),
		}},
	}

	_, err := runApply(t, recorder, multistepTestJSON, false)
	if err == nil || !strings.Contains(err.Error(), "2 Synthetic tests have exact name") {
		t.Fatalf("expected a refusal, got %v", err)
	}
	recorder.wantRequests(t, "GET /api/v1/synthetics/tests")
	if len(recorder.writes) != 0 {
		t.Fatalf("an ambiguous match must not write: %#v", recorder.writes)
	}
}

func TestSyntheticsApplyDryRunResolvesTheMatchWithoutWriting(t *testing.T) {
	recorder := &syntheticsRecorder{
		listPages: [][]any{{
			syntheticsListEntry("abc-def-ghi", "Production asset delivery end-to-end", "api"),
		}},
	}

	out, err := runApply(t, recorder, multistepTestJSON, true)
	if err != nil {
		t.Fatalf("apply --dry-run: %v", err)
	}

	recorder.wantRequests(t, "GET /api/v1/synthetics/tests")
	if len(recorder.writes) != 0 {
		t.Fatalf("--dry-run must not write: %#v", recorder.writes)
	}
	if out.Meta["dry_run"] != true {
		t.Fatalf("expected dry_run in meta, got %#v", out.Meta)
	}
	if out.Data["action"] != "update" ||
		out.Data["public_id"] != "abc-def-ghi" ||
		out.Data["matched_by"] != "name" {
		t.Fatalf("expected the resolved match in data, got %#v", out.Data)
	}
}

func TestSyntheticsApplyDryRunReportsACreateWhenNothingMatches(t *testing.T) {
	recorder := &syntheticsRecorder{listPages: [][]any{{}}}

	out, err := runApply(t, recorder, simpleTestJSON, true)
	if err != nil {
		t.Fatalf("apply --dry-run: %v", err)
	}

	recorder.wantRequests(t, "GET /api/v1/synthetics/tests")
	if out.Data["action"] != "create" {
		t.Fatalf("expected a create action, got %#v", out.Data)
	}
	if _, present := out.Data["public_id"]; present {
		t.Fatalf("a create has no public ID yet: %#v", out.Data)
	}
}

// syntheticsRecorder serves the Synthetics endpoints apply uses and records
// every request, so a test can assert that no write was attempted.
type syntheticsRecorder struct {
	listPages   [][]any
	writeBody   map[string]any
	requests    []string
	listQueries []string
	writes      []map[string]any
	page        int
}

func (r *syntheticsRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.requests = append(r.requests, req.Method+" "+req.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	if req.Method == http.MethodGet && req.URL.Path == "/api/v1/synthetics/tests" {
		r.listQueries = append(r.listQueries, req.URL.Query().Get("page_number"))
		tests := []any{}
		if r.page < len(r.listPages) {
			tests = r.listPages[r.page]
		}
		r.page++
		_ = json.NewEncoder(w).Encode(map[string]any{"tests": tests})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.writes = append(r.writes, body)

	// Datadog echoes the stored test back, with the identifiers it owns.
	response := map[string]any{}
	for key, value := range body {
		response[key] = value
	}
	for key, value := range r.writeBody {
		response[key] = value
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (r *syntheticsRecorder) wantRequests(t *testing.T, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(r.requests, want) {
		t.Fatalf("expected requests %#v, got %#v", want, r.requests)
	}
}

type applyEnvelope struct {
	Query map[string]any `json:"query"`
	Meta  map[string]any `json:"meta"`
	Data  map[string]any `json:"data"`
}

// runApply drives applySyntheticsTest against a recorded Datadog, which is the
// only way to see whether a write happened.
func runApply(t *testing.T, recorder *syntheticsRecorder, body string, dryRun bool) (applyEnvelope, error) {
	t.Helper()
	var envelope applyEnvelope

	path := writeSyntheticsTest(t, body)
	test, err := loadSyntheticsTest(path)
	if err != nil {
		t.Fatalf("load Synthetic test: %v", err)
	}

	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)

	cfg := datadog.NewConfiguration()
	cfg.Compress = false
	cfg.Servers = datadog.ServerConfigurations{{URL: server.URL}}
	ctx := context.WithValue(context.Background(), datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {Key: "api-key"},
		"appKeyAuth": {Key: "app-key"},
	})
	client := &dd.Client{
		Site:      "datadoghq.test",
		API:       datadog.NewAPIClient(cfg),
		Context:   ctx,
		UsingAuth: "api_app_key",
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if applyErr := applySyntheticsTest(cmd, client, path, test, dryRun); applyErr != nil {
		return envelope, applyErr
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope %q: %v", out.String(), err)
	}
	return envelope, nil
}

func writeSyntheticsTest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func syntheticsSummary(publicID, name string, testType datadogV1.SyntheticsTestDetailsType) datadogV1.SyntheticsTestDetailsWithoutSteps {
	test := datadogV1.NewSyntheticsTestDetailsWithoutSteps()
	test.SetPublicId(publicID)
	test.SetName(name)
	test.SetType(testType)
	return *test
}

func syntheticsListEntry(publicID, name, testType string) any {
	return map[string]any{"public_id": publicID, "name": name, "type": testType}
}

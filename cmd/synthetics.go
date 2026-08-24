package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/dd"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

// Name matching pages through the full test list rather than trusting a single
// large page, and refuses to guess once the list is bigger than it can scan.
const (
	syntheticsListPageSize = 100
	syntheticsListMaxPages = 100
)

var syntheticsCmd = &cobra.Command{
	Use:     "synthetics",
	Aliases: []string{"synthetic"},
	Short:   "List, retrieve, validate, and apply Datadog Synthetic tests",
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

var syntheticsValidateCmd = &cobra.Command{
	Use:   "validate <test.json>",
	Short: "Validate a Synthetic API test definition locally",
	Long: `Validate a Synthetic API test definition.

Datadog publishes no Synthetics validate endpoint, so unlike monitors validate
this runs entirely locally and needs no credentials. It applies the same checks
apply does: required fields, an api test type with a known subtype, the config
each subtype needs, and a scan for JSON keys ddcli does not recognize.

Use apply --dry-run when you also want to see which test would be written.`,
	Args: cobra.ExactArgs(1),
	RunE: runSyntheticsValidate,
}

var syntheticsApplyCmd = &cobra.Command{
	Use:   "apply <test.json>",
	Short: "Create or update a Synthetic API test from JSON",
	Long: `Create or update a Synthetic API test from a canonical JSON definition.

When the JSON contains a public_id, apply updates that test. Otherwise apply
matches an existing test by exact name. It refuses to choose when more than one
test has the same name, and refuses to write over a name that belongs to a
browser or mobile test, preventing accidental overwrites.

Single-request API tests (subtype http, ssl, tcp, dns, icmp, udp, websocket,
grpc) and multistep API tests (subtype multi) are supported. Browser and mobile
tests are not: their definitions use different endpoints and schemas.

public_id and monitor_id are assigned by Datadog, so apply strips both from the
request body and takes the public ID from the match instead.

--dry-run resolves the match read-only and reports what would be written.`,
	Args: cobra.ExactArgs(1),
	RunE: runSyntheticsApply,
}

func init() {
	rootCmd.AddCommand(syntheticsCmd)
	syntheticsCmd.AddCommand(syntheticsListCmd, syntheticsGetCmd, syntheticsValidateCmd, syntheticsApplyCmd)

	syntheticsListCmd.Flags().String("query", "", "Search text for Synthetic tests")
	syntheticsListCmd.Flags().Int64("limit", 100, "Maximum tests to return")
	syntheticsListCmd.Flags().Int64("start", 0, "Offset for search results")
	syntheticsListCmd.Flags().Bool("full", false, "Include full test configuration when searching")
	syntheticsApplyCmd.Flags().Bool("dry-run", false, "Resolve the match and show the intended write without performing it")
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

func runSyntheticsValidate(cmd *cobra.Command, args []string) error {
	test, err := loadSyntheticsTest(args[0])
	if err != nil {
		return err
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "synthetics validate", "file": args[0]},
		map[string]any{"valid": true, "local_only": true},
		syntheticsTestSummary(test),
		outputOptions(),
	)
}

func runSyntheticsApply(cmd *cobra.Command, args []string) error {
	test, err := loadSyntheticsTest(args[0])
	if err != nil {
		return err
	}
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	return applySyntheticsTest(cmd, client, args[0], test, dryRun)
}

// applySyntheticsTest resolves the target test read-only, then writes unless
// dryRun is set. It never writes before the match is unambiguous.
func applySyntheticsTest(cmd *cobra.Command, client *dd.Client, path string, test datadogV1.SyntheticsAPITest, dryRun bool) error {
	api := datadogV1.NewSyntheticsApi(client.API)
	publicID := test.GetPublicId()
	matchedBy := "public_id"
	var lookupResp *http.Response
	if publicID == "" {
		matchedBy = "name"
		tests, httpResp, err := listAllSyntheticsTests(client.Context, api)
		if err != nil {
			return apiError("synthetics apply lookup", httpResp, err)
		}
		lookupResp = httpResp
		publicID, err = exactSyntheticsTestID(tests, test.Name)
		if err != nil {
			return err
		}
	}

	action := "update"
	if publicID == "" {
		action = "create"
		matchedBy = ""
	}
	if dryRun {
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "synthetics apply", "file": path},
			meta(client.Site, map[string]any{"dry_run": true, "action": action}, lookupResp),
			syntheticsApplyPlan(test, action, publicID, matchedBy),
			outputOptions(),
		)
	}

	// Datadog owns both identifiers; the public ID travels in the URL instead.
	test.PublicId = nil
	test.MonitorId = nil

	if action == "create" {
		created, httpResp, createErr := api.CreateSyntheticsAPITest(client.Context, test)
		if createErr != nil {
			return apiError("synthetics apply create", httpResp, createErr)
		}
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "synthetics apply", "file": path},
			meta(client.Site, map[string]any{"action": action}, httpResp),
			created,
			outputOptions(),
		)
	}

	updated, httpResp, updateErr := api.UpdateAPITest(client.Context, publicID, test)
	if updateErr != nil {
		return apiError("synthetics apply update", httpResp, updateErr)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "synthetics apply", "file": path},
		meta(client.Site, map[string]any{"action": action, "matched_by": matchedBy}, httpResp),
		updated,
		outputOptions(),
	)
}

func loadSyntheticsTest(path string) (datadogV1.SyntheticsAPITest, error) {
	var test datadogV1.SyntheticsAPITest
	body, err := os.ReadFile(path)
	if err != nil {
		return test, fmt.Errorf("read Synthetic test JSON: %w", err)
	}
	if err := json.Unmarshal(body, &test); err != nil {
		return test, fmt.Errorf("parse Synthetic test JSON: %w", err)
	}
	// A type other than "api" leaves the whole object unparsed, so name it
	// before the generic scan reports a useless "(root)".
	if len(test.UnparsedObject) > 0 || test.Type != datadogV1.SYNTHETICSAPITESTTYPE_API {
		return test, fmt.Errorf("Synthetic test type must be %q, browser and mobile tests are not supported",
			datadogV1.SYNTHETICSAPITESTTYPE_API)
	}
	if unknown := dd.UnknownFields(test); len(unknown) > 0 {
		return test, fmt.Errorf("Synthetic test JSON contains unsupported fields: %s", strings.Join(unknown, ", "))
	}
	if strings.TrimSpace(test.Name) == "" {
		return test, fmt.Errorf("Synthetic test name is required")
	}
	if len(test.Locations) == 0 {
		return test, fmt.Errorf("Synthetic test locations are required")
	}
	subtype, ok := test.GetSubtypeOk()
	if !ok {
		return test, fmt.Errorf("Synthetic test subtype is required")
	}
	if !subtype.IsValid() {
		return test, fmt.Errorf("Synthetic test subtype %q is invalid", *subtype)
	}
	if err := validateSyntheticsConfig(*subtype, test.Config); err != nil {
		return test, err
	}
	return test, nil
}

func validateSyntheticsConfig(subtype datadogV1.SyntheticsTestDetailsSubType, config datadogV1.SyntheticsAPITestConfig) error {
	if subtype == datadogV1.SYNTHETICSTESTDETAILSSUBTYPE_MULTI {
		if len(config.Steps) == 0 {
			return fmt.Errorf("multistep Synthetic test requires config.steps")
		}
		if len(config.Assertions) > 0 {
			return fmt.Errorf("multistep Synthetic test asserts per step, not in config.assertions")
		}
		return nil
	}
	if len(config.Steps) > 0 {
		return fmt.Errorf("config.steps requires subtype %q", datadogV1.SYNTHETICSTESTDETAILSSUBTYPE_MULTI)
	}
	if config.Request == nil {
		return fmt.Errorf("Synthetic test requires config.request")
	}
	if len(config.Assertions) == 0 {
		return fmt.Errorf("Synthetic test requires config.assertions")
	}
	return nil
}

// listAllSyntheticsTests pages through every test so a name match is decided
// against the whole org, and errors rather than matching a partial list.
func listAllSyntheticsTests(ctx context.Context, api *datadogV1.SyntheticsApi) ([]datadogV1.SyntheticsTestDetailsWithoutSteps, *http.Response, error) {
	var all []datadogV1.SyntheticsTestDetailsWithoutSteps
	var lastResp *http.Response
	for page := int64(0); page < syntheticsListMaxPages; page++ {
		params := datadogV1.NewListTestsOptionalParameters().
			WithPageSize(syntheticsListPageSize).
			WithPageNumber(page)
		resp, httpResp, err := api.ListTests(ctx, *params)
		if err != nil {
			return nil, httpResp, err
		}
		lastResp = httpResp
		tests := resp.GetTests()
		all = append(all, tests...)
		if int64(len(tests)) < syntheticsListPageSize {
			return all, lastResp, nil
		}
	}
	return nil, lastResp, fmt.Errorf("refusing to apply: more than %d Synthetic tests to scan for an exact name match, set public_id in the JSON instead",
		syntheticsListMaxPages*syntheticsListPageSize)
}

func exactSyntheticsTestID(tests []datadogV1.SyntheticsTestDetailsWithoutSteps, name string) (string, error) {
	var matches []datadogV1.SyntheticsTestDetailsWithoutSteps
	for _, test := range tests {
		if test.GetName() == name {
			matches = append(matches, test)
		}
	}
	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("refusing to apply: %d Synthetic tests have exact name %q", len(matches), name)
	}
	match := matches[0]
	if match.GetType() != datadogV1.SYNTHETICSTESTDETAILSTYPE_API {
		return "", fmt.Errorf("refusing to apply: Synthetic test %q is a %q test, not an %q test",
			name, match.GetType(), datadogV1.SYNTHETICSTESTDETAILSTYPE_API)
	}
	publicID := match.GetPublicId()
	if publicID == "" {
		return "", fmt.Errorf("refusing to apply: Synthetic test %q has no public ID", name)
	}
	return publicID, nil
}

func syntheticsApplyPlan(test datadogV1.SyntheticsAPITest, action, publicID, matchedBy string) map[string]any {
	plan := syntheticsTestSummary(test)
	plan["action"] = action
	if publicID != "" {
		plan["public_id"] = publicID
	}
	if matchedBy != "" {
		plan["matched_by"] = matchedBy
	}
	return plan
}

func syntheticsTestSummary(test datadogV1.SyntheticsAPITest) map[string]any {
	return map[string]any{
		"name":      test.Name,
		"type":      test.Type,
		"subtype":   test.GetSubtype(),
		"locations": test.Locations,
		"steps":     len(test.Config.Steps),
	}
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/dd"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var logsPipelinesApplyCmd = &cobra.Command{
	Use:   "apply <pipeline.json>",
	Short: "Create or update a log pipeline from JSON",
	Long: `Create or update a custom log processing pipeline from a canonical JSON
definition, the same way monitors/dashboards/synthetics apply do.

If the JSON contains an id, apply updates that pipeline. Otherwise apply
matches an existing pipeline by exact name: no match creates a pipeline, one
match updates it, and more than one match is refused rather than guessed.

Apply refuses to touch a pipeline that is read-only on either side of the
write: a submitted definition with "is_read_only": true, or a live pipeline
Datadog already marks read-only (its bundled integration pipelines, such as
"Varnish"). This keeps an exact-name collision from ever overwriting one of
Datadog's own integration pipelines.

An optional top-level "insert_after_pipeline_id" places the pipeline
immediately after a named pipeline in the evaluation order — for example,
after a read-only integration pipeline whose category-processor output this
pipeline needs to re-map. Order placement is idempotent: a pipeline already
positioned correctly is left alone and reported as unchanged.

Requires an application key with Datadog's Logs Pipelines write permission
(Datadog UI: Logs > Pipelines write access). The exact RBAC permission name
has not been confirmed against a live 403 from this tool; if apply is
rejected on authorization, the error detail will name the permission Datadog
actually requires. --dry-run performs no write and needs no write
permission.`,
	Args: cobra.ExactArgs(1),
	RunE: runLogsPipelinesApply,
}

func init() {
	logsPipelinesCmd.AddCommand(logsPipelinesApplyCmd)
	logsPipelinesApplyCmd.Flags().Bool("dry-run", false, "Validate JSON and show the intended match/order without writing")
}

func runLogsPipelinesApply(cmd *cobra.Command, args []string) error {
	pipeline, insertAfter, err := loadPipelineApply(args[0])
	if err != nil {
		return err
	}

	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewLogsPipelinesApi(client.API)

	id := pipeline.GetId()
	if id != "" {
		current, getResp, getErr := api.GetLogsPipeline(client.Context, id)
		if getErr != nil {
			return apiError("logs pipelines apply get", getResp, getErr)
		}
		if current.GetIsReadOnly() {
			return fmt.Errorf("refusing to apply: pipeline %q is read-only", id)
		}
	} else {
		existing, listResp, listErr := api.ListLogsPipelines(client.Context)
		if listErr != nil {
			return apiError("logs pipelines apply lookup", listResp, listErr)
		}
		matchID, matchIsReadOnly, matchErr := matchPipelineByName(existing, pipeline.GetName())
		if matchErr != nil {
			return matchErr
		}
		if matchIsReadOnly {
			return fmt.Errorf("refusing to apply: an existing read-only pipeline is already named %q", pipeline.GetName())
		}
		id = matchID
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if dryRun {
		return runLogsPipelinesApplyDryRun(cmd, args[0], client, api, pipeline, id, insertAfter)
	}

	if id == "" {
		created, resp, createErr := api.CreateLogsPipeline(client.Context, pipeline)
		if createErr != nil {
			return apiError("logs pipelines apply create", resp, createErr)
		}
		orderChanged, currentOrder, desiredOrder, orderErr := syncPipelineOrder(client, api, created.GetId(), insertAfter)
		if orderErr != nil {
			return orderErr
		}
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "logs pipelines apply", "file": args[0]},
			meta(client.Site, map[string]any{"action": "create"}, resp),
			map[string]any{
				"pipeline":      created,
				"order_changed": orderChanged,
				"current_order": currentOrder,
				"desired_order": desiredOrder,
			},
			outputOptions(),
		)
	}

	pipeline.Id = nil
	updated, resp, updateErr := api.UpdateLogsPipeline(client.Context, id, pipeline)
	if updateErr != nil {
		return apiError("logs pipelines apply update", resp, updateErr)
	}
	orderChanged, currentOrder, desiredOrder, orderErr := syncPipelineOrder(client, api, id, insertAfter)
	if orderErr != nil {
		return orderErr
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs pipelines apply", "file": args[0]},
		meta(client.Site, map[string]any{"action": "update"}, resp),
		map[string]any{
			"pipeline":      updated,
			"order_changed": orderChanged,
			"current_order": currentOrder,
			"desired_order": desiredOrder,
		},
		outputOptions(),
	)
}

func runLogsPipelinesApplyDryRun(
	cmd *cobra.Command,
	file string,
	client *dd.Client,
	api *datadogV1.LogsPipelinesApi,
	pipeline datadogV1.LogsPipeline,
	id string,
	insertAfter *string,
) error {
	action := "update"
	if id == "" {
		action = "create"
	}
	result := map[string]any{
		"action":      action,
		"pipeline_id": id,
		"name":        pipeline.GetName(),
	}

	if insertAfter != nil {
		order, orderResp, orderErr := api.GetLogsPipelineOrder(client.Context)
		if orderErr != nil {
			return apiError("logs pipelines apply order", orderResp, orderErr)
		}
		current := order.GetPipelineIds()
		found := false
		for _, existingID := range current {
			if existingID == *insertAfter {
				found = true
				break
			}
		}
		result["insert_after_pipeline_id"] = *insertAfter
		result["insert_after_found"] = found
		if !found {
			result["order_note"] = "insert_after_pipeline_id was not found in the current pipeline order"
		} else if id == "" {
			result["order_note"] = "pipeline does not exist yet; it will be inserted after insert_after_pipeline_id once created"
		} else {
			desired, deriveErr := desiredPipelineOrder(current, id, *insertAfter)
			if deriveErr != nil {
				return deriveErr
			}
			result["current_order"] = current
			result["desired_order"] = desired
			result["order_changed"] = !equalStringSlices(current, desired)
		}
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "logs pipelines apply", "file": file},
		map[string]any{"dry_run": true},
		result,
		outputOptions(),
	)
}

// syncPipelineOrder moves pipelineID to immediately after insertAfter in the
// live pipeline order, if it is not already there. It is a no-op when
// insertAfter is nil.
func syncPipelineOrder(
	client *dd.Client,
	api *datadogV1.LogsPipelinesApi,
	pipelineID string,
	insertAfter *string,
) (changed bool, current []string, desired []string, err error) {
	if insertAfter == nil {
		return false, nil, nil, nil
	}
	order, orderResp, orderErr := api.GetLogsPipelineOrder(client.Context)
	if orderErr != nil {
		return false, nil, nil, apiError("logs pipelines apply order get", orderResp, orderErr)
	}
	current = order.GetPipelineIds()
	desired, err = desiredPipelineOrder(current, pipelineID, *insertAfter)
	if err != nil {
		return false, current, nil, err
	}
	if equalStringSlices(current, desired) {
		return false, current, desired, nil
	}
	newOrder := datadogV1.NewLogsPipelinesOrder(desired)
	_, updateResp, updateErr := api.UpdateLogsPipelineOrder(client.Context, *newOrder)
	if updateErr != nil {
		return false, current, desired, apiError("logs pipelines apply order update", updateResp, updateErr)
	}
	return true, current, desired, nil
}

// desiredPipelineOrder returns current with pipelineID removed from wherever
// it already sits and reinserted immediately after insertAfterID. It fails if
// insertAfterID is not present, or equals pipelineID itself.
func desiredPipelineOrder(current []string, pipelineID string, insertAfterID string) ([]string, error) {
	if pipelineID == insertAfterID {
		return nil, fmt.Errorf("insert_after_pipeline_id %q cannot be the pipeline's own id", insertAfterID)
	}
	filtered := make([]string, 0, len(current))
	for _, id := range current {
		if id != pipelineID {
			filtered = append(filtered, id)
		}
	}
	anchorIndex := -1
	for i, id := range filtered {
		if id == insertAfterID {
			anchorIndex = i
			break
		}
	}
	if anchorIndex == -1 {
		return nil, fmt.Errorf("insert_after_pipeline_id %q not found in the current pipeline order", insertAfterID)
	}
	desired := make([]string, 0, len(filtered)+1)
	desired = append(desired, filtered[:anchorIndex+1]...)
	desired = append(desired, pipelineID)
	desired = append(desired, filtered[anchorIndex+1:]...)
	return desired, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func matchPipelineByName(pipelines []datadogV1.LogsPipeline, name string) (id string, isReadOnly bool, err error) {
	var matches []datadogV1.LogsPipeline
	for _, p := range pipelines {
		if p.GetName() == name {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return "", false, nil
	case 1:
		return matches[0].GetId(), matches[0].GetIsReadOnly(), nil
	default:
		return "", false, fmt.Errorf("refusing to apply: %d pipelines have exact name %q", len(matches), name)
	}
}

func loadPipelineApply(path string) (datadogV1.LogsPipeline, *string, error) {
	var pipeline datadogV1.LogsPipeline
	body, err := os.ReadFile(path)
	if err != nil {
		return pipeline, nil, fmt.Errorf("read pipeline JSON: %w", err)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return pipeline, nil, fmt.Errorf("parse pipeline JSON: %w", err)
	}
	var insertAfter *string
	if raw, ok := envelope["insert_after_pipeline_id"]; ok {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return pipeline, nil, fmt.Errorf("parse insert_after_pipeline_id: %w", err)
		}
		if strings.TrimSpace(value) == "" {
			return pipeline, nil, fmt.Errorf("insert_after_pipeline_id must not be empty")
		}
		insertAfter = &value
		delete(envelope, "insert_after_pipeline_id")
	}

	pipelineBody, err := json.Marshal(envelope)
	if err != nil {
		return pipeline, nil, fmt.Errorf("re-encode pipeline JSON: %w", err)
	}
	if err := json.Unmarshal(pipelineBody, &pipeline); err != nil {
		return pipeline, nil, fmt.Errorf("parse pipeline JSON: %w", err)
	}

	if len(pipeline.UnparsedObject) > 0 || len(pipeline.AdditionalProperties) > 0 {
		return pipeline, nil, fmt.Errorf("pipeline JSON contains unsupported fields")
	}
	if strings.TrimSpace(pipeline.GetName()) == "" {
		return pipeline, nil, fmt.Errorf("pipeline name is required")
	}
	if pipeline.GetIsReadOnly() {
		return pipeline, nil, fmt.Errorf("refusing to apply a pipeline definition with is_read_only: true")
	}
	filter, hasFilter := pipeline.GetFilterOk()
	if !hasFilter || strings.TrimSpace(filter.GetQuery()) == "" {
		return pipeline, nil, fmt.Errorf("pipeline filter.query is required")
	}
	processors, hasProcessors := pipeline.GetProcessorsOk()
	if !hasProcessors || len(*processors) == 0 {
		return pipeline, nil, fmt.Errorf("at least one processor is required")
	}
	for i, processor := range *processors {
		if processor.UnparsedObject != nil {
			return pipeline, nil, fmt.Errorf("processor %d has an unrecognized or malformed type", i)
		}
	}

	return pipeline, insertAfter, nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "List, retrieve, validate, and apply Datadog monitors",
}

var monitorsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List monitors",
	RunE:  runMonitorsList,
}

var monitorsGetCmd = &cobra.Command{
	Use:   "get <monitor-id>",
	Short: "Get a monitor definition",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsGet,
}

var monitorsValidateCmd = &cobra.Command{
	Use:   "validate <monitor.json>",
	Short: "Validate a monitor definition with Datadog",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsValidate,
}

var monitorsApplyCmd = &cobra.Command{
	Use:   "apply <monitor.json>",
	Short: "Create or update a monitor from JSON",
	Long: `Create or update a monitor from a canonical JSON definition.

When the JSON contains an id, apply updates that monitor. Otherwise apply
matches an existing monitor by exact name. It refuses to choose when more than
one monitor has the same name, preventing accidental overwrites.

Use --require-non-notifying to refuse definitions that are neither draft nor
globally silenced without notification mentions.`,
	Args: cobra.ExactArgs(1),
	RunE: runMonitorsApply,
}

func init() {
	rootCmd.AddCommand(monitorsCmd)
	monitorsCmd.AddCommand(monitorsListCmd, monitorsGetCmd, monitorsValidateCmd, monitorsApplyCmd)

	monitorsListCmd.Flags().String("name", "", "Server-side monitor name filter")
	monitorsListCmd.Flags().String("tags", "", "Filter by service or host tags")
	monitorsListCmd.Flags().String("monitor-tags", "", "Filter by monitor tags")
	monitorsListCmd.Flags().Int64("page", 0, "Page number")
	monitorsListCmd.Flags().Int32("page-size", 100, "Monitors per page")
	monitorsApplyCmd.Flags().Bool("dry-run", false, "Validate JSON and show the intended match without writing")
	monitorsApplyCmd.Flags().Bool("require-non-notifying", false, "Require draft or globally silenced monitor configuration")
}

func runMonitorsList(cmd *cobra.Command, args []string) error {
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")
	tags, _ := cmd.Flags().GetString("tags")
	monitorTags, _ := cmd.Flags().GetString("monitor-tags")
	page, _ := cmd.Flags().GetInt64("page")
	pageSize, _ := cmd.Flags().GetInt32("page-size")

	params := datadogV1.NewListMonitorsOptionalParameters().
		WithPage(page).
		WithPageSize(pageSize)
	if name != "" {
		params.WithName(name)
	}
	if tags != "" {
		params.WithTags(tags)
	}
	if monitorTags != "" {
		params.WithMonitorTags(monitorTags)
	}

	api := datadogV1.NewMonitorsApi(client.API)
	resp, httpResp, err := api.ListMonitors(client.Context, *params)
	if err != nil {
		return apiError("monitors list", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "monitors list", "name": name},
		meta(client.Site, map[string]any{
			"page":         page,
			"page_size":    pageSize,
			"tags":         tags,
			"monitor_tags": monitorTags,
		}, httpResp),
		resp,
		outputOptions(),
	)
}

func runMonitorsGet(cmd *cobra.Command, args []string) error {
	id, err := parseMonitorID(args[0])
	if err != nil {
		return err
	}
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewMonitorsApi(client.API)
	resp, httpResp, err := api.GetMonitor(client.Context, id)
	if err != nil {
		return apiError("monitors get", httpResp, err)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "monitors get", "monitor_id": id},
		meta(client.Site, nil, httpResp),
		resp,
		outputOptions(),
	)
}

func runMonitorsValidate(cmd *cobra.Command, args []string) error {
	monitor, err := loadMonitor(args[0])
	if err != nil {
		return err
	}
	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewMonitorsApi(client.API)
	id := monitor.GetId()
	monitor.Id = nil

	var result any
	var httpRespStatus int
	if id == 0 {
		validated, httpResp, validateErr := api.ValidateMonitor(client.Context, monitor)
		if validateErr != nil {
			return apiError("monitors validate", httpResp, validateErr)
		}
		result = validated
		if httpResp != nil {
			httpRespStatus = httpResp.StatusCode
		}
	} else {
		validated, httpResp, validateErr := api.ValidateExistingMonitor(client.Context, id, monitor)
		if validateErr != nil {
			return apiError("monitors validate existing", httpResp, validateErr)
		}
		result = validated
		if httpResp != nil {
			httpRespStatus = httpResp.StatusCode
		}
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "monitors validate", "file": args[0]},
		map[string]any{"site": client.Site, "http_status": httpRespStatus},
		result,
		outputOptions(),
	)
}

func runMonitorsApply(cmd *cobra.Command, args []string) error {
	monitor, err := loadMonitor(args[0])
	if err != nil {
		return err
	}
	requireNonNotifying, _ := cmd.Flags().GetBool("require-non-notifying")
	if requireNonNotifying && !monitorIsNonNotifying(monitor) {
		return fmt.Errorf("refusing to apply: monitor must be draft or globally silenced without notification mentions")
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "monitors apply", "file": args[0]},
			map[string]any{"dry_run": true},
			map[string]any{
				"action":        intendedMonitorAction(monitor),
				"monitor_id":    monitor.GetId(),
				"name":          monitor.GetName(),
				"type":          monitor.GetType(),
				"non_notifying": monitorIsNonNotifying(monitor),
			},
			outputOptions(),
		)
	}

	client, err := datadogClient(cmd)
	if err != nil {
		return err
	}
	api := datadogV1.NewMonitorsApi(client.API)
	id := monitor.GetId()
	action := "update"
	if id == 0 {
		resp, httpResp, listErr := api.ListMonitors(client.Context,
			*datadogV1.NewListMonitorsOptionalParameters().
				WithName(monitor.GetName()).
				WithPageSize(1000))
		if listErr != nil {
			return apiError("monitors apply lookup", httpResp, listErr)
		}
		id, err = exactMonitorID(resp, monitor.GetName())
		if err != nil {
			return err
		}
	}

	if id == 0 {
		action = "create"
		created, httpResp, createErr := api.CreateMonitor(client.Context, monitor)
		if createErr != nil {
			return apiError("monitors apply create", httpResp, createErr)
		}
		return output.WriteEnvelope(cmd.OutOrStdout(),
			map[string]any{"command": "monitors apply", "file": args[0]},
			meta(client.Site, map[string]any{"action": action}, httpResp),
			created,
			outputOptions(),
		)
	}

	monitor.Id = nil
	update, err := monitorUpdateRequest(monitor)
	if err != nil {
		return err
	}
	updated, httpResp, updateErr := api.UpdateMonitor(client.Context, id, update)
	if updateErr != nil {
		return apiError("monitors apply update", httpResp, updateErr)
	}
	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "monitors apply", "file": args[0]},
		meta(client.Site, map[string]any{"action": action}, httpResp),
		updated,
		outputOptions(),
	)
}

func loadMonitor(path string) (datadogV1.Monitor, error) {
	var monitor datadogV1.Monitor
	body, err := os.ReadFile(path)
	if err != nil {
		return monitor, fmt.Errorf("read monitor JSON: %w", err)
	}
	if err := json.Unmarshal(body, &monitor); err != nil {
		return monitor, fmt.Errorf("parse monitor JSON: %w", err)
	}
	if len(monitor.UnparsedObject) > 0 || len(monitor.AdditionalProperties) > 0 {
		return monitor, fmt.Errorf("monitor JSON contains unsupported fields")
	}
	if strings.TrimSpace(monitor.GetName()) == "" {
		return monitor, fmt.Errorf("monitor name is required")
	}
	if strings.TrimSpace(monitor.Query) == "" {
		return monitor, fmt.Errorf("monitor query is required")
	}
	if !monitor.Type.IsValid() {
		return monitor, fmt.Errorf("monitor type is invalid")
	}
	return monitor, nil
}

func parseMonitorID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("monitor id must be a positive integer")
	}
	return id, nil
}

func intendedMonitorAction(monitor datadogV1.Monitor) string {
	if monitor.GetId() != 0 {
		return "update-by-id"
	}
	return "create-or-update-by-name"
}

func exactMonitorID(monitors []datadogV1.Monitor, name string) (int64, error) {
	var ids []int64
	for _, monitor := range monitors {
		if monitor.GetName() == name {
			ids = append(ids, monitor.GetId())
		}
	}
	switch len(ids) {
	case 0:
		return 0, nil
	case 1:
		return ids[0], nil
	default:
		return 0, fmt.Errorf("refusing to apply: %d monitors have exact name %q", len(ids), name)
	}
}

func monitorUpdateRequest(monitor datadogV1.Monitor) (datadogV1.MonitorUpdateRequest, error) {
	var update datadogV1.MonitorUpdateRequest
	body, err := json.Marshal(monitor)
	if err != nil {
		return update, fmt.Errorf("encode monitor update: %w", err)
	}
	if err := json.Unmarshal(body, &update); err != nil {
		return update, fmt.Errorf("prepare monitor update: %w", err)
	}
	return update, nil
}

func monitorIsNonNotifying(monitor datadogV1.Monitor) bool {
	if draft, ok := monitor.GetDraftStatusOk(); ok && *draft == datadogV1.MONITORDRAFTSTATUS_DRAFT {
		return true
	}
	message := monitor.GetMessage()
	if strings.Contains(message, "@") {
		return false
	}
	options, ok := monitor.GetOptionsOk()
	if !ok {
		return false
	}
	silenced, ok := options.GetSilencedOk()
	if !ok {
		return false
	}
	until, ok := (*silenced)["*"]
	return ok && until == 0
}

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func TestLoadMonitor(t *testing.T) {
	path := writeMonitor(t, `{
		"name": "Production Kamal edge telemetry",
		"type": "query alert",
		"query": "avg(last_5m):avg:openresty.requests{env:production} < 1",
		"message": "No notifications while draft",
		"draft_status": "draft",
		"options": {
			"thresholds": {"critical": 1},
			"notify_no_data": false
		}
	}`)

	monitor, err := loadMonitor(path)
	if err != nil {
		t.Fatalf("load monitor: %v", err)
	}
	if monitor.GetName() != "Production Kamal edge telemetry" {
		t.Fatalf("unexpected name: %q", monitor.GetName())
	}
	if !monitorIsNonNotifying(monitor) {
		t.Fatal("expected draft monitor to be non-notifying")
	}
}

func TestLoadMonitorRejectsInvalidDefinition(t *testing.T) {
	path := writeMonitor(t, `{
		"name": "Missing query",
		"type": "query alert",
		"query": ""
	}`)

	_, err := loadMonitor(path)
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query validation error, got %v", err)
	}
}

func TestLoadMonitorRejectsUnsupportedFields(t *testing.T) {
	path := writeMonitor(t, `{
		"name": "Typo",
		"type": "query alert",
		"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
		"notify_no_data_typo": true
	}`)

	_, err := loadMonitor(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported fields") {
		t.Fatalf("expected unsupported-field error, got %v", err)
	}
}

func TestExactMonitorID(t *testing.T) {
	monitors := []datadogV1.Monitor{
		monitorSummary(101, "Production Kamal edge telemetry"),
		monitorSummary(202, "Other monitor"),
	}

	id, err := exactMonitorID(monitors, "Production Kamal edge telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if id != 101 {
		t.Fatalf("expected 101, got %d", id)
	}
}

func TestExactMonitorIDRefusesDuplicateNames(t *testing.T) {
	monitors := []datadogV1.Monitor{
		monitorSummary(101, "Production Kamal edge telemetry"),
		monitorSummary(202, "Production Kamal edge telemetry"),
	}

	_, err := exactMonitorID(monitors, "Production Kamal edge telemetry")
	if err == nil || !strings.Contains(err.Error(), "2 monitors") {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}
}

func TestMonitorIsNonNotifyingRequiresDraftOrGlobalSilence(t *testing.T) {
	published := monitorSummary(101, "Published")
	published.SetMessage("No recipients")
	if monitorIsNonNotifying(published) {
		t.Fatal("expected published unsilenced monitor to be notifying")
	}

	options := datadogV1.NewMonitorOptions()
	options.SetSilenced(map[string]int64{"*": 0})
	published.SetOptions(*options)
	if !monitorIsNonNotifying(published) {
		t.Fatal("expected globally silenced monitor to be non-notifying")
	}

	published.SetMessage("@pagerduty")
	if monitorIsNonNotifying(published) {
		t.Fatal("expected notification mention to fail the safety check")
	}
}

func TestMonitorUpdateRequestPreservesCanonicalFields(t *testing.T) {
	monitor := monitorSummary(101, "Production Kamal edge telemetry")
	monitor.Query = "avg(last_5m):avg:openresty.requests{env:production} < 1"
	monitor.SetMessage("Draft only")
	monitor.SetDraftStatus(datadogV1.MONITORDRAFTSTATUS_DRAFT)
	monitor.Id = nil

	update, err := monitorUpdateRequest(monitor)
	if err != nil {
		t.Fatal(err)
	}
	if update.GetName() != monitor.GetName() || update.GetQuery() != monitor.Query {
		t.Fatalf("update lost canonical fields: %#v", update)
	}
	if update.GetDraftStatus() != datadogV1.MONITORDRAFTSTATUS_DRAFT {
		t.Fatalf("update lost draft status: %#v", update.GetDraftStatus())
	}
}

func writeMonitor(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "monitor.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func monitorSummary(id int64, name string) datadogV1.Monitor {
	monitor := datadogV1.NewMonitor(
		"avg(last_5m):avg:system.cpu.user{*} > 90",
		datadogV1.MONITORTYPE_QUERY_ALERT,
	)
	monitor.SetId(id)
	monitor.SetName(name)
	return *monitor
}

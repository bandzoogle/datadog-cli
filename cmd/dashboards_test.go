package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func TestLoadDashboard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.json")
	err := os.WriteFile(path, []byte(`{
		"title": "Review Edge Overview",
		"layout_type": "ordered",
		"widgets": [{"definition": {"type": "note", "content": "test"}}]
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	dashboard, err := loadDashboard(path)
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	if dashboard.Title != "Review Edge Overview" {
		t.Fatalf("unexpected title: %q", dashboard.Title)
	}
	if len(dashboard.Widgets) != 1 {
		t.Fatalf("expected one widget, got %d", len(dashboard.Widgets))
	}
}

func TestLoadDashboardRejectsInvalidDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.json")
	if err := os.WriteFile(path, []byte(`{
		"title":"No widgets",
		"layout_type":"ordered",
		"widgets":[]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadDashboard(path)
	if err == nil || !strings.Contains(err.Error(), "widgets are required") {
		t.Fatalf("expected widgets validation error, got %v", err)
	}
}

func TestExactDashboardID(t *testing.T) {
	dashboards := []datadogV1.DashboardSummaryDefinition{
		summary("abc", "Review Edge Overview"),
		summary("def", "Other dashboard"),
	}

	id, err := exactDashboardID(dashboards, "Review Edge Overview")
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc" {
		t.Fatalf("expected abc, got %q", id)
	}
}

func TestExactDashboardIDRefusesDuplicateTitles(t *testing.T) {
	dashboards := []datadogV1.DashboardSummaryDefinition{
		summary("abc", "Review Edge Overview"),
		summary("def", "Review Edge Overview"),
	}

	_, err := exactDashboardID(dashboards, "Review Edge Overview")
	if err == nil || !strings.Contains(err.Error(), "2 dashboards") {
		t.Fatalf("expected duplicate-title error, got %v", err)
	}
}

func summary(id, title string) datadogV1.DashboardSummaryDefinition {
	dashboard := datadogV1.NewDashboardSummaryDefinition()
	dashboard.SetId(id)
	dashboard.SetTitle(title)
	return *dashboard
}

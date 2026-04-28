package cmd

import (
	"strings"
	"testing"
)

func TestUniquePermissionsDedupesAndSorts(t *testing.T) {
	got := uniquePermissions([]scopeInfo{
		{Permission: "metrics_read"},
		{Permission: "logs_read_data"},
		{Permission: "metrics_read"},
	})
	want := []string{"logs_read_data", "metrics_read"}
	if len(got) != len(want) {
		t.Fatalf("expected %d permissions, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func TestFilterScopesByCommandPrefix(t *testing.T) {
	got := filterScopes(requiredScopes(), "cost")
	if len(got) != 2 {
		t.Fatalf("expected two cost scope rows, got %d", len(got))
	}
	for _, scope := range got {
		if scope.Permission != "cloud_cost_management_read" {
			t.Fatalf("unexpected cost permission: %s", scope.Permission)
		}
	}
}

func TestRenderScopesTextShowsUniquePermissionsTogether(t *testing.T) {
	resp := scopesResponse{
		Required: []scopeInfo{
			{Command: "metrics query", Permission: "metrics_read"},
			{Command: "dashboards list", Permission: "dashboards_read"},
		},
		Unique: []string{"dashboards_read", "metrics_read"},
		RecommendedRole: []string{
			"Create a service account for ddcli.",
		},
	}

	got := renderScopesText(resp, "")
	if !strings.Contains(got, "dashboards_read\nmetrics_read") {
		t.Fatalf("expected permissions grouped together, got:\n%s", got)
	}
	if !strings.Contains(got, "- metrics query: metrics_read") {
		t.Fatalf("expected command coverage, got:\n%s", got)
	}
}

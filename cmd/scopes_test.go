package cmd

import (
	"reflect"
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

func TestLogIndexPatchUsesRBACPermissionWithoutOAuthScope(t *testing.T) {
	got := filterScopes(requiredScopes(), "logs indexes patch-exclusions")
	if len(got) != 2 {
		t.Fatalf("expected two patch permission rows, got %d", len(got))
	}
	permissions := uniquePermissions(got)
	want := []string{"logs_modify_indexes", "logs_read_config"}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("expected permissions %#v, got %#v", want, permissions)
	}
	for _, item := range got {
		if item.OAuthScope != "" {
			t.Fatalf("expected no OAuth scope, got %s", item.OAuthScope)
		}
	}
}

func TestUniqueOAuthScopesOmitsUnavailableScopes(t *testing.T) {
	got := uniqueOAuthScopes([]scopeInfo{
		{Permission: "logs_modify_indexes"},
		{Permission: "dashboards_read", OAuthScope: "dashboards_read"},
		{Permission: "dashboards_write", OAuthScope: "dashboards_write"},
		{Permission: "other", OAuthScope: "dashboards_read"},
	})
	want := []string{"dashboards_read", "dashboards_write"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestRenderScopesTextShowsUniquePermissionsTogether(t *testing.T) {
	resp := scopesResponse{
		Required: []scopeInfo{
			{Command: "metrics query", Permission: "metrics_read", OAuthScope: "metrics_read"},
			{Command: "logs indexes patch-exclusions", Permission: "logs_modify_indexes"},
		},
		Unique:      []string{"logs_modify_indexes", "metrics_read"},
		OAuthScopes: []string{"metrics_read"},
		RecommendedRole: []string{
			"Create a service account for ddcli.",
		},
	}

	got := renderScopesText(resp, "")
	if !strings.Contains(got, "logs_modify_indexes\nmetrics_read") {
		t.Fatalf("expected permissions grouped together, got:\n%s", got)
	}
	if !strings.Contains(got, "- metrics query: RBAC metrics_read; OAuth metrics_read") {
		t.Fatalf("expected OAuth command coverage, got:\n%s", got)
	}
	if !strings.Contains(got, "- logs indexes patch-exclusions: RBAC logs_modify_indexes; OAuth unavailable") {
		t.Fatalf("expected command coverage, got:\n%s", got)
	}
}

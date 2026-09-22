package cmd

import "testing"

func TestBuildNotificationRulesListParams(t *testing.T) {
	got := buildNotificationRulesListParams(2, 25, "name:asc", "name:pagerduty", "created_by")

	if got.Page == nil || *got.Page != 2 {
		t.Fatalf("unexpected page: %#v", got.Page)
	}
	if got.PerPage == nil || *got.PerPage != 25 {
		t.Fatalf("unexpected per_page: %#v", got.PerPage)
	}
	if got.Sort == nil || *got.Sort != "name:asc" {
		t.Fatalf("unexpected sort: %#v", got.Sort)
	}
	if got.Filters == nil || *got.Filters != "name:pagerduty" {
		t.Fatalf("unexpected filters: %#v", got.Filters)
	}
	if got.Include == nil || *got.Include != "created_by" {
		t.Fatalf("unexpected include: %#v", got.Include)
	}
}

func TestBuildNotificationRulesListParamsOmitsUnsetValues(t *testing.T) {
	got := buildNotificationRulesListParams(0, 0, "", "", "")

	if got.Page != nil {
		t.Fatalf("expected nil page, got %#v", got.Page)
	}
	if got.PerPage != nil {
		t.Fatalf("expected nil per_page, got %#v", got.PerPage)
	}
	if got.Sort != nil {
		t.Fatalf("expected nil sort, got %#v", got.Sort)
	}
	if got.Filters != nil {
		t.Fatalf("expected nil filters, got %#v", got.Filters)
	}
	if got.Include != nil {
		t.Fatalf("expected nil include, got %#v", got.Include)
	}
}

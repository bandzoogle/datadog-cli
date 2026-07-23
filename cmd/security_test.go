package cmd

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func TestBuildSecuritySignalsSearchRequest(t *testing.T) {
	from := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)

	got := buildSecuritySignalsSearchRequest("team:bandzoogle", from, to, 25, "next-page")

	if got.Filter == nil || got.Filter.GetQuery() != "team:bandzoogle" {
		t.Fatalf("unexpected filter: %#v", got.Filter)
	}
	if !got.Filter.GetFrom().Equal(from) || !got.Filter.GetTo().Equal(to) {
		t.Fatalf("unexpected time range: %s to %s", got.Filter.GetFrom(), got.Filter.GetTo())
	}
	if got.Page == nil || got.Page.GetLimit() != 25 || got.Page.GetCursor() != "next-page" {
		t.Fatalf("unexpected page: %#v", got.Page)
	}
	if got.Sort == nil || *got.Sort != datadogV2.SECURITYMONITORINGSIGNALSSORT_TIMESTAMP_DESCENDING {
		t.Fatalf("unexpected sort: %#v", got.Sort)
	}
}

func TestBuildSecuritySignalsSearchRequestOmitsEmptyCursor(t *testing.T) {
	now := time.Now()
	got := buildSecuritySignalsSearchRequest("*", now.Add(-time.Hour), now, 50, "")

	if got.Page == nil {
		t.Fatal("expected page")
	}
	if got.Page.Cursor != nil {
		t.Fatalf("expected nil cursor, got %q", got.Page.GetCursor())
	}
}

package cmd

import "testing"

func TestBuildCostQueryDefaults(t *testing.T) {
	got := buildCostQuery("", "", "")
	want := "sum:all.cost{*}"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildCostQueryWithGroupBy(t *testing.T) {
	got := buildCostQuery("aws.cost.amortized", "env:prod", "service")
	want := "sum:aws.cost.amortized{env:prod} by {service}"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

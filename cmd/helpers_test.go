package cmd

import (
	"errors"
	"strings"
	"testing"
)

type testBodyError struct {
	body []byte
}

func (e testBodyError) Error() string {
	return "400 Bad Request"
}

func (e testBodyError) Body() []byte {
	return e.body
}

func TestDatadogErrorDetailExtractsErrors(t *testing.T) {
	err := testBodyError{body: []byte(`{"errors":["invalid widget type","title is required"]}`)}

	got := datadogErrorDetail(err)

	if got != ": invalid widget type; title is required" {
		t.Fatalf("unexpected error detail: %q", got)
	}
}

func TestDatadogErrorDetailIgnoresUnknownBodies(t *testing.T) {
	if got := datadogErrorDetail(errors.New("failure")); got != "" {
		t.Fatalf("expected no detail, got %q", got)
	}
}

func TestDatadogErrorDetailLimitsOutput(t *testing.T) {
	err := testBodyError{body: []byte(`{"errors":["` + strings.Repeat("x", 600) + `"]}`)}

	got := datadogErrorDetail(err)

	if len(got) != 505 || !strings.HasSuffix(got, "...") {
		t.Fatalf("expected bounded detail, got length %d", len(got))
	}
}

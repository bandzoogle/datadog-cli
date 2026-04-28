package timeutil

import (
	"testing"
	"time"
)

func TestUnixSecondsParsesRelativeTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	got, err := UnixSeconds("now-15m", now)
	if err != nil {
		t.Fatalf("parse relative time: %v", err)
	}
	want := now.Add(-15 * time.Minute).Unix()
	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

func TestUnixMillisParsesRFC3339(t *testing.T) {
	got, err := UnixMillis("2024-01-02T03:04:05Z", time.Now())
	if err != nil {
		t.Fatalf("parse RFC3339: %v", err)
	}
	if got != int64(1704164645000) {
		t.Fatalf("unexpected millis: %d", got)
	}
}

func TestParseRejectsUnknownUnit(t *testing.T) {
	_, err := Parse("now-1q", time.Now())
	if err == nil {
		t.Fatal("expected unknown unit error")
	}
}

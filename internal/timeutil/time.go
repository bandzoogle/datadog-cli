package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func UnixSeconds(value string, now time.Time) (int64, error) {
	t, err := Parse(value, now)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

func UnixMillis(value string, now time.Time) (int64, error) {
	t, err := Parse(value, now)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func Parse(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("time value cannot be empty")
	}
	if value == "now" {
		return now, nil
	}
	if strings.HasPrefix(value, "now-") {
		d, err := parseDatadogDuration(strings.TrimPrefix(value, "now-"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(-d), nil
	}
	if strings.HasPrefix(value, "now+") {
		d, err := parseDatadogDuration(strings.TrimPrefix(value, "now+"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		if len(value) > 10 {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time %q: use now, now-15m, RFC3339, Unix seconds, or Unix milliseconds", value)
}

func parseDatadogDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("relative time duration cannot be empty")
	}
	unit := value[len(value)-1]
	amount, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
	if err != nil || amount < 0 {
		return 0, fmt.Errorf("invalid relative time duration %q", value)
	}
	switch unit {
	case 's':
		return time.Duration(amount) * time.Second, nil
	case 'm':
		return time.Duration(amount) * time.Minute, nil
	case 'h':
		return time.Duration(amount) * time.Hour, nil
	case 'd':
		return time.Duration(amount) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(amount) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported relative time unit %q in %q", unit, value)
	}
}

package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func requireStringFlag(cmd *cobra.Command, name string) error {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("--%s is required", name)
	}
	return nil
}

func meta(site string, values map[string]any, resp *http.Response) map[string]any {
	out := map[string]any{
		"site": site,
	}
	for k, v := range values {
		if v != nil && v != "" {
			out[k] = v
		}
	}
	if resp != nil {
		out["http_status"] = resp.StatusCode
		if requestID := resp.Header.Get("x-datadog-request-id"); requestID != "" {
			out["request_id"] = requestID
		}
	}
	return out
}

func apiError(operation string, resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return fmt.Errorf("%s failed: %w", operation, err)
	}
	return fmt.Errorf("%s failed: %w (http status %d)", operation, err, resp.StatusCode)
}

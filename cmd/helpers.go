package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	detail := datadogErrorDetail(err)
	if resp == nil {
		return fmt.Errorf("%s failed: %w%s", operation, err, detail)
	}
	return fmt.Errorf("%s failed: %w (http status %d)%s", operation, err, resp.StatusCode, detail)
}

func datadogErrorDetail(err error) string {
	var bodyError interface{ Body() []byte }
	if !errors.As(err, &bodyError) {
		return ""
	}
	var response struct {
		Errors []string `json:"errors"`
	}
	if json.Unmarshal(bodyError.Body(), &response) != nil || len(response.Errors) == 0 {
		return ""
	}
	for i, message := range response.Errors {
		response.Errors[i] = strings.TrimSpace(message)
	}
	detail := strings.Join(response.Errors, "; ")
	if len(detail) > 500 {
		detail = detail[:500] + "..."
	}
	return ": " + detail
}

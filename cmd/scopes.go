package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

type scopeInfo struct {
	Command    string   `json:"command"`
	API        string   `json:"api"`
	Permission string   `json:"permission"`
	OAuthScope string   `json:"oauth_scope"`
	Notes      []string `json:"notes,omitempty"`
}

type scopesResponse struct {
	Summary         []string    `json:"summary"`
	Required        []scopeInfo `json:"required"`
	Unique          []string    `json:"unique"`
	RecommendedRole []string    `json:"recommended_role"`
}

var scopesCmd = &cobra.Command{
	Use:     "scopes",
	Aliases: []string{"permissions"},
	Short:   "Show Datadog permissions and OAuth scopes needed by ddcli",
	Long: `Show the Datadog permissions and OAuth scopes needed by ddcli.

Datadog API keys identify the organization. Access is controlled by the
application key owner's role permissions, scoped application key permissions,
or OAuth access token scopes.`,
	RunE: runScopes,
}

func init() {
	rootCmd.AddCommand(scopesCmd)
	scopesCmd.Flags().String("command", "", "Filter by command prefix, e.g. logs, cost, dashboards")
}

func runScopes(cmd *cobra.Command, args []string) error {
	filter, _ := cmd.Flags().GetString("command")
	required := filterScopes(requiredScopes(), filter)
	resp := scopesResponse{
		Summary: []string{
			"DD_API_KEY identifies the Datadog organization.",
			"DD_APP_KEY or DD_APPLICATION_KEY must belong to a user/service account with the listed permissions.",
			"DD_ACCESS_TOKEN must include the listed OAuth scopes when using bearer-token auth.",
			"Use the smallest subset that matches the commands you plan to run.",
		},
		Required: required,
		Unique:   uniquePermissions(required),
		RecommendedRole: []string{
			"Create a service account for ddcli.",
			"Grant only the read permissions for the command groups you need.",
			"For logs, also apply Datadog log restriction queries if the CLI should see only a subset of log data.",
		},
	}

	opts := outputOptions()
	if !opts.Raw {
		_, err := fmt.Fprint(cmd.OutOrStdout(), renderScopesText(resp, filter))
		return err
	}

	return output.WriteEnvelope(cmd.OutOrStdout(),
		map[string]any{"command": "scopes", "filter": filter},
		map[string]any{"auth_required": false, "count": len(required)},
		resp,
		opts,
	)
}

func renderScopesText(resp scopesResponse, filter string) string {
	var b strings.Builder
	if filter != "" {
		fmt.Fprintf(&b, "Datadog permissions for %q:\n\n", filter)
	} else {
		b.WriteString("Datadog permissions for ddcli:\n\n")
	}

	b.WriteString(strings.Join(resp.Unique, "\n"))
	b.WriteString("\n\n")
	b.WriteString("Grant these to the DD_APP_KEY owner, scoped application key, or OAuth token.\n")
	b.WriteString("DD_API_KEY only identifies the Datadog organization.\n\n")

	b.WriteString("Command coverage:\n")
	for _, item := range resp.Required {
		fmt.Fprintf(&b, "- %s: %s\n", item.Command, item.Permission)
	}

	b.WriteString("\nSetup notes:\n")
	for _, note := range resp.RecommendedRole {
		fmt.Fprintf(&b, "- %s\n", note)
	}
	return b.String()
}

func requiredScopes() []scopeInfo {
	return []scopeInfo{
		{
			Command:    "logs search",
			API:        "Logs Search API",
			Permission: "logs_read_data",
			OAuthScope: "logs_read_data",
			Notes: []string{
				"Actual log visibility may also be narrowed by log indexes and restriction queries.",
			},
		},
		{
			Command:    "synthetics list|get",
			API:        "Synthetics API",
			Permission: "synthetics_read",
			OAuthScope: "synthetics_read",
		},
		{
			Command:    "metrics list|metadata|query",
			API:        "Metrics API",
			Permission: "metrics_read",
			OAuthScope: "metrics_read",
		},
		{
			Command:    "hosts list|totals",
			API:        "Hosts API",
			Permission: "hosts_read",
			OAuthScope: "hosts_read",
		},
		{
			Command:    "dashboards list|get",
			API:        "Dashboards API",
			Permission: "dashboards_read",
			OAuthScope: "dashboards_read",
		},
		{
			Command:    "apm spans",
			API:        "Spans API",
			Permission: "apm_read",
			OAuthScope: "apm_read",
		},
		{
			Command:    "errors search|get",
			API:        "Error Tracking API",
			Permission: "error_tracking_read",
			OAuthScope: "error_tracking_read",
		},
		{
			Command:    "cost analyze",
			API:        "Metrics API with cloud_cost data source",
			Permission: "cloud_cost_management_read",
			OAuthScope: "cloud_cost_management_read",
			Notes: []string{
				"Cloud Cost data is queried through the Metrics API, but Datadog documents Cloud Cost visibility under Cloud Cost Management read access.",
			},
		},
		{
			Command:    "cost accounts|budgets|allocation-rules|tag-pipelines|custom-costs list",
			API:        "Cloud Cost Management API",
			Permission: "cloud_cost_management_read",
			OAuthScope: "cloud_cost_management_read",
		},
	}
}

func filterScopes(scopes []scopeInfo, filter string) []scopeInfo {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return scopes
	}
	out := make([]scopeInfo, 0, len(scopes))
	for _, scope := range scopes {
		if strings.HasPrefix(strings.ToLower(scope.Command), filter) || strings.Contains(strings.ToLower(scope.Command), filter+" ") {
			out = append(out, scope)
		}
	}
	return out
}

func uniquePermissions(scopes []scopeInfo) []string {
	seen := map[string]bool{}
	for _, scope := range scopes {
		if scope.Permission != "" {
			seen[scope.Permission] = true
		}
	}
	out := make([]string, 0, len(seen))
	for permission := range seen {
		out = append(out, permission)
	}
	sort.Strings(out)
	return out
}

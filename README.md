# Datadog CLI

`ddcli` is a Datadog CLI designed for scripts and LLM agents. It keeps the
interface narrow and discoverable while returning stable JSON on stdout.
Most commands are read-only; write commands are explicit and idempotent.

## Auth

Use a scoped API key and application key:

```sh
export DD_SITE=datadoghq.com
export DD_API_KEY=...
export DD_APP_KEY=...
```

`DD_APPLICATION_KEY` is also accepted. Flags can override env vars:

```sh
ddcli --site us3.datadoghq.com --api-key "$DD_API_KEY" --app-key "$DD_APP_KEY" dashboards list
```

For temporary OAuth-style credentials, `DD_ACCESS_TOKEN` or `--access-token` is accepted and sent as bearer auth through the Datadog Go client. A full `auth login` flow is not implemented yet because it needs OAuth client registration and secure refresh-token storage.

## Output

Every command writes JSON to stdout. Diagnostics and errors go to stderr.

Default output is wrapped:

```json
{
  "query": {
    "command": "logs search",
    "filter": "service:web error"
  },
  "meta": {
    "site": "datadoghq.com",
    "from": "now-15m",
    "to": "now",
    "limit": 50
  },
  "data": {}
}
```

Use `--pretty` for indented JSON and `--raw` to print the Datadog response without the `query` and `meta` wrapper.

## Commands

```sh
ddcli logs search --query 'service:web error' --from now-15m --to now --limit 50
ddcli logs indexes order
ddcli logs indexes list --query production
ddcli logs indexes get bandzoogle-production
ddcli logs indexes patch-exclusions patch.json --dry-run
ddcli logs indexes patch-exclusions patch.json
ddcli logs pipelines order
ddcli logs pipelines list --query openresty
ddcli logs pipelines get PIPELINE_ID
ddcli synthetics list --query checkout --limit 25
ddcli synthetics get abc-def-ghi
ddcli metrics list --query system.cpu
ddcli metrics metadata system.cpu.user
ddcli metrics query --query 'avg:system.cpu.user{*}' --from now-1h --to now
ddcli hosts list --filter 'env:prod' --limit 25
ddcli hosts totals
ddcli dashboards list --query app --limit 100
ddcli dashboards get abc-def-ghi
ddcli dashboards apply dashboard.json --dry-run
ddcli dashboards apply dashboard.json
ddcli apm spans --query 'service:api @http.status_code:500' --from now-15m --to now --limit 25
ddcli appsec blocked-rules summary --from now-7d --limit 200 --pretty
ddcli appsec custom-rules list
ddcli appsec exclusion-filters list
ddcli errors search --query 'service:api' --track trace --from now-1h --to now
ddcli errors get ISSUE_ID
ddcli security signals search --query 'team:bandzoogle' --from now-1h --to now
ddcli security signals get SIGNAL_ID
ddcli cost analyze --group-by service --from now-30d --to now-2d --limit 25
ddcli cost analyze --metric aws.cost.amortized --filter 'env:prod' --group-by region
ddcli cost accounts list --provider all
ddcli cost budgets list
ddcli cost allocation-rules list
ddcli cost tag-pipelines list
ddcli scopes
ddcli scopes --command cost
```

Time flags accept `now`, relative values like `now-15m`, RFC3339 timestamps, Unix seconds, or Unix milliseconds. Logs and spans pass time strings through to Datadog; metrics and Error Tracking convert them to the epoch formats required by their APIs.

## Log configuration audit

The `logs indexes` and `logs pipelines` commands are read-only. Index responses
include routing filters, daily quotas, Standard/Flex retention, and ordered
exclusion filters with sample rates. Pipeline responses preserve the API's
processor order and include filters, parser rules, remappers, and nested
processors.

Use the explicit order commands when routing order matters:

```sh
ddcli logs indexes order --pretty
ddcli logs indexes get bandzoogle-production --pretty
ddcli logs pipelines order --pretty
ddcli logs pipelines list --query openresty --pretty
```

These commands require `logs_read_config`. Datadog also requires an
administrator-owned application key for pipeline configuration reads.

### Preconditioned exclusion patch

`logs indexes patch-exclusions` is the only log-configuration write command. It
fetches the named index, requires every named exclusion to occur exactly once
with the exact expected query, replaces only those query strings, and sends one
index update request containing all current updateable properties. Missing,
duplicate, stale, unknown, empty, and no-op patch entries fail without writing.

Patch files are non-secret JSON:

```json
{
  "index_name": "example-index",
  "replacements": [
    {
      "name": "Example exclusion",
      "expected_query": "service:example status:info",
      "replacement_query": "service:example status:(info OR ok)"
    }
  ]
}
```

Always run `--dry-run` first. It performs the read and all precondition checks,
then prints the exact before/after queries and preserved invariants without an
update. The guarded read requires the `logs_read_config` RBAC permission.
Applying through the V1 `UpdateLogsIndex` endpoint requires
`logs_modify_indexes`, shown as **Logs Modify Indexes** in the Datadog UI.
These are role or scoped application-key permissions; Datadog does not offer
them as OAuth client scopes.

## Dashboard apply

`dashboards apply` validates a canonical Datadog dashboard JSON file and creates
or updates it. If the definition contains an `id`, that dashboard is updated.
Otherwise, the command matches by exact title: no match creates a dashboard,
one match updates it, and multiple matches fail without writing.

Use `--dry-run` to validate the JSON locally without credentials or a write:

```sh
ddcli dashboards apply dashboards/review-edge-overview.json --dry-run --pretty
ddcli dashboards apply dashboards/review-edge-overview.json --pretty
```

The application key or access token needs `dashboards_write`; title-based
matching also needs `dashboards_read`.

## Required Permissions

Use `ddcli scopes` to print the Datadog RBAC permissions and, where available,
separate OAuth scopes needed by each command group. Commands marked with no
OAuth scope require application-key authentication. This command does not
require Datadog credentials.

Datadog API keys identify the organization. Access is controlled by the application key owner's role permissions, scoped application key permissions, or OAuth access token scopes.

Unlike most commands, `ddcli scopes` prints a compact human-readable permissions list by default because it is primarily a setup reference. Use `--raw` if you need the underlying JSON data object.

## Cost Analysis

`ddcli cost analyze` queries Datadog's Cloud Cost data source through the metrics API. Its default query is `sum:all.cost{*} by {service}` over the last 30 days, ending at `now-2d` to avoid incomplete recent cost data.

For cost-cutting work, start broad and then pivot:

```sh
ddcli cost analyze --group-by service
ddcli cost analyze --group-by team
ddcli cost analyze --group-by subaccountname
ddcli cost analyze --group-by region
ddcli cost analyze --metric aws.cost.amortized --group-by instance_type
```

The Cloud Cost Management inventory commands expose setup and governance context, such as connected accounts, budgets, custom allocation rules, tag pipelines, and custom cost files.

## MCP Tradeoff

This CLI is meant to complement MCP rather than replace it everywhere. MCP is useful for integrated auth and rich tool discovery, but it also adds server and tool context to conversations. A CLI gives agents a smaller contract: `--help`, explicit commands, JSON stdout, stderr diagnostics, and replayable invocations outside Cursor.

## Binary releases

Pushes to `main` that pass CI and change Go sources or `go.mod` / `go.sum` trigger a patch semver GitHub release (for example `v0.1.1`). Builds are published for Linux and macOS on `amd64` and `arm64`.

Stable download URLs (always point at the **latest** release’s asset names):

- Linux x86_64: `https://github.com/bandzoogle/datadog-cli/releases/latest/download/ddcli_linux_amd64.tar.gz`
- Linux arm64: `https://github.com/bandzoogle/datadog-cli/releases/latest/download/ddcli_linux_arm64.tar.gz`
- macOS x86_64: `https://github.com/bandzoogle/datadog-cli/releases/latest/download/ddcli_darwin_amd64.tar.gz`
- macOS arm64: `https://github.com/bandzoogle/datadog-cli/releases/latest/download/ddcli_darwin_arm64.tar.gz`

Each archive contains a single `ddcli` binary. Run `ddcli --version` to see the release tag baked into the binary. Versioned archives (`ddcli_<tag>_linux_amd64.tar.gz`, etc.) are attached for pinning.

## Development

```sh
bin/build
```

`bin/build` runs unit tests, builds `dist/ddcli`, and smoke-tests the help output
for the main command groups. Live Datadog smoke tests require credentials and
are intentionally manual for now.

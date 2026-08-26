# Getting Started

This guide walks you through installing the LOZA CLI, configuring it, and running your first commands.

## Install

### From Source

```bash
cd cli
go build -o loza.exe ./cmd/loza
```

### Go Install

```bash
go install github.com/astraive/loza/cli/cmd/loza@latest
```

## Configure

The CLI reads configuration from `loza-cli.defaults.yaml` in the current directory or the path specified by `--config`. The most important settings are the collector URL and API key.

```yaml
collector:
  url: https://collector.loza.dev
```

Set the collector URL and API key via environment variables:

```bash
export LOZA_COLLECTOR_URL=https://collector.loza.dev
export LOZA_API_KEY=lz_sec_live_k_xxx_yyyy
```

### Authentication

The CLI authenticates with the collector using `Authorization: Bearer <key>`. Set the `LOZA_API_KEY` environment variable.

| Env Var | Description |
|---|---|
| `LOZA_API_KEY` | Primary ingest API key (works for all services) |
| `LOZA_COLLECTOR_API_KEY` | Collector-specific key (fallback) |
| `LOZA_CORTEX_API_KEY` | Cortex-specific key (fallback) |

```bash
# Production
export LOZA_API_KEY=lz_sec_live_k_xxx_yyyy
loza query "SELECT * FROM events LIMIT 10"

# Local dev (no auth)
export LOZA_COLLECTOR_URL=http://localhost:9308
loza status
```

See [Security](../../docs/security.md) for key types and RBAC roles.

## First Commands

### Check Health

```bash
loza doctor
```

This runs a series of checks against the configured collector: connectivity, readiness, schema compatibility, and sink status.

### View Collector Status

```bash
loza status
```

Shows the collector's uptime, version, configured sinks, and current event counts.

### Query Events

```bash
loza query "SELECT * FROM events WHERE service = 'payment-service' LIMIT 10"
```

Executes a SQL query against the collector's DuckDB storage.

### Tail Events

```bash
loza tail --service payment-service
```

Streams events in real time from the collector. Use `--kind`, `--service`, or `--level` to narrow by event kind, service name, or severity level.

### Emit a Test Event

```bash
loza emit sample --kind http_request --outcome success --level info --attrs status_code=200,path=/api/test
```

Sends a single test event to the collector for validation.

## Command Categories

| Category | Commands | Description |
|---|---|---|
| Setup | `init`, `dev` | Initialize workspace, start local dev environment |
| Collector | `collector run`, `collector config`, `collector version` | Run and manage the collector |
| Worker | `worker run`, `worker version` | Run the queue worker |
| Schema | `schema validate`, `schema fetch` | Validate and fetch event schemas |
| Query | `query`, `tail`, `watch` | Query and stream events |
| Status | `status`, `sinks`, `doctor` | Check system health |
| DLQ | `dlq`, `replay` | Inspect and replay dead letter queue |
| GDPR | `delete tenant`, `delete user`, `delete event` | Data deletion |
| Audit | `audit`, `export` | PII audit and data export |
| Cortex | `cortex`, `incident`, `graph`, `signatures` | Incident intelligence queries |
| Deploy | `deploy`, `dashboard` | Deployment and dashboard management |
| Debug | `debug`, `bench` | Debugging and benchmarking |

## Next Steps

- [Command Reference](command-reference.md) -- full command reference with flags and examples
- [Collector Getting Started](../../collector/docs/getting-started.md) -- set up the collector
- [Architecture](../../collector/docs/architecture.md) -- understand the system architecture

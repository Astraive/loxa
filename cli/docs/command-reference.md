# Command Reference

Full reference for all LOZA CLI commands.

## Global Flags

| Flag | Description |
|---|---|
| `--config` | Path to config file (default: `loza-cli.defaults.yaml`) |
| `--verbose` | Enable verbose logging |
| `--output` | Output format: `text`, `json`, `yaml` (default: `text`) |
| `--no-color` | Disable colored output |

---

## Setup Commands

### loza init

Initialize a new LOZA workspace. Creates a default config file and validates the environment.

```bash
loza init
```

**Flags**

| Flag | Description |
|---|---|
| `--dir` | Target directory (default: current directory) |
| `--force` | Overwrite existing config |

**Example**

```bash
loza init --dir ./my-project
```

---

### loza dev

Start a local development environment with collector, Cortex, and a DuckDB backend.

```bash
loza dev
```

**Flags**

| Flag | Description |
|---|---|
| `--port` | Collector port (default: 9308) |
| `--cortex-port` | Cortex port (default: 9312) |
| `--db` | DuckDB file path (default: `loza-local.db`) |

**Example**

```bash
loza dev --port 9308 --cortex-port 9312
```

---

## Collector Commands

### loza collector run

Run the collector binary.

```bash
loza collector run
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Collector config file path |

**Example**

```bash
loza collector run -c configs/loza.local.yaml
```

---

### loza collector version

Print the collector version.

```bash
loza collector version
```

---

### loza collector config print

Print the resolved collector configuration.

```bash
loza collector config print
```

**Example**

```bash
loza collector config print --output json
```

---

### loza collector config validate

Validate a collector config file.

```bash
loza collector config validate
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Config file to validate |

**Example**

```bash
loza collector config validate -c configs/loza.local.yaml
```

---

## Worker Commands

### loza worker run

Run the queue worker for distributed delivery.

```bash
loza worker run
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Worker config file path |

**Example**

```bash
loza worker run -c configs/loza.queue.kafka.yaml
```

---

### loza worker version

Print the worker version.

```bash
loza worker version
```

---

## Config Commands

### loza config print

Print the resolved CLI configuration.

```bash
loza config print
```

**Example**

```bash
loza config print --output yaml
```

---

### loza config validate

Validate the CLI configuration file.

```bash
loza config validate
```

---

## Schema Commands

### loza schema validate

Validate an event payload against the registered schema.

```bash
loza schema validate
```

**Flags**

| Flag | Description |
|---|---|
| `--file` | Event file to validate |
| `--stdin` | Read event from stdin |
| `--schema` | Schema version to validate against |

**Example**

```bash
cat event.json | loza schema validate --stdin
loza schema validate --file event.json --schema v2
```

---

### loza schema fetch

Fetch the current schema from the collector.

```bash
loza schema fetch
```

**Flags**

| Flag | Description |
|---|---|
| `--output` | Output file path |
| `--version` | Schema version to fetch |

**Example**

```bash
loza schema fetch --output schema.json --version v2
```

---

## Emit Commands

### loza emit

Send a single event to the collector.

```bash
loza emit
```

**Flags**

| Flag | Description |
|---|---|
| `--service` | Source service name (required) |
| `--type` | Event type (required) |
| `--attr` | Attribute in key=value format (repeatable) |
| `--file` | Read event from JSON file |
| `--stdin` | Read event from stdin |

**Example**

```bash
loza emit --service payment-service --type http_request \
  --attr status_code=500 --attr path=/api/charge --attr latency_ms=4500

echo '{"service":"test","event_type":"custom"}' | loza emit --stdin
```

---

## Query Commands

### loza query

Execute a SQL query against the collector's DuckDB storage.

```bash
loza query "SQL_STATEMENT"
```

**Flags**

| Flag | Description |
|---|---|
| `--output` | Output format: `text`, `json`, `csv` (default: `text`) |
| `--limit` | Max rows to return |

**Example**

```bash
loza query "SELECT service, count(*) FROM events GROUP BY service ORDER BY count(*) DESC"
loza query "SELECT * FROM events WHERE timestamp > now() - interval '1 hour'" --output json
```

---

### loza tail

Stream events in real time from the collector.

```bash
loza tail
```

**Flags**

| Flag | Description |
|---|---|
| `--service` | Filter by service name |
| `--type` | Filter by event type |
| `--filter` | Attribute filter in key=value format (repeatable) |
| `--since` | Start time (e.g., `5m`, `1h`) |

**Example**

```bash
loza tail --service payment-service --since 5m
loza tail --type http_request --filter status_code=500
```

---

### loza watch

Watch events and display real-time statistics.

```bash
loza watch
```

**Flags**

| Flag | Description |
|---|---|
| `--interval` | Refresh interval (default: `1s`) |
| `--service` | Filter by service |

**Example**

```bash
loza watch --interval 2s --service payment-service
```

---

## Status Commands

### loza status

Show collector status: uptime, version, sinks, event counts.

```bash
loza status
```

**Example**

```bash
loza status --output json
```

---

### loza sinks

List configured sinks and their status. Supports explicit subcommands:
- `loza sinks list`
- `loza sinks show <name>`
- `loza sinks test <name>`

```bash
loza sinks
```

**Example**

```bash
loza sinks --output json
```

```bash
loza sinks test duckdb
```

---

### loza doctor

Run health checks against the collector.

```bash
loza doctor
```

Checks: connectivity, readiness, schema compatibility, sink health, DLQ status.

**Example**

```bash
loza doctor --verbose
```

---

## DLQ Commands

### loza dlq

Inspect the Dead Letter Queue.

```bash
loza dlq
```

**Flags**

| Flag | Description |
|---|---|
| `--limit` | Max entries to show (default: 20) |
| `--output` | Output format |

**Example**

```bash
loza dlq --limit 50 --output json
```

---

### loza replay

Replay events from the Dead Letter Queue.

```bash
loza replay
```

**Flags**

| Flag | Description |
|---|---|
| `--all` | Replay all DLQ entries |
| `--id` | Replay a specific DLQ entry by ID |
| `--limit` | Max entries to replay |

**Example**

```bash
loza replay --all
loza replay --id dlq-001
```

---

## GDPR Commands

### loza delete tenant

Delete all data for a tenant.

```bash
loza delete tenant <tenant_id>
```

**Example**

```bash
loza delete tenant acme-corp
```

---

### loza delete user

Delete all data for a user.

```bash
loza delete user <user_id>
```

**Example**

```bash
loza delete user user-12345
```

---

### loza delete event

Delete a specific event.

```bash
loza delete event <event_id>
```

**Example**

```bash
loza delete event evt-abc123
```

---

## Audit Commands

### loza audit

Run a PII audit on stored events.

```bash
loza audit
```

**Flags**

| Flag | Description |
|---|---|
| `--service` | Audit a specific service |
| `--since` | Audit events since a time (e.g., `7d`) |
| `--output` | Output format |

**Example**

```bash
loza audit --service payment-service --since 30d --output json
```

---

### loza export

Export events from the collector.

```bash
loza export
```

**Flags**

| Flag | Description |
|---|---|
| `--output` | Output file path (required) |
| `--format` | Export format: `json`, `csv`, `ndjson` (default: `ndjson`) |
| `--service` | Filter by service |
| `--since` | Export events since a time |
| `--limit` | Max events to export |

**Example**

```bash
loza export --output events.json --format json --service payment-service --since 7d
```

---

## Cortex Commands

### loza cortex

Query the Cortex control plane.

```bash
loza cortex
```

**Flags**

| Flag | Description |
|---|---|
| `--url` | Cortex URL (default: from config) |

---

### loza incident

Reconstruct an incident.

```bash
loza incident <incident_id>
```

**Flags**

| Flag | Description |
|---|---|
| `--mode` | Reconstruction mode: `fast`, `deep` (default: `fast`) |
| `--output` | Output format |

**Example**

```bash
loza incident inc-001 --mode deep --output json
```

---

### loza graph

Query the service graph.

```bash
loza graph <service>
```

**Flags**

| Flag | Description |
|---|---|
| `--depth` | Graph traversal depth (default: 3) |
| `--output` | Output format |

**Example**

```bash
loza graph payment-service --depth 5 --output json
```

---

### loza signatures

Search for similar incident signatures.

```bash
loza signatures
```

**Flags**

| Flag | Description |
|---|---|
| `--symptoms` | Comma-separated symptom list |
| `--services` | Comma-separated service list |
| `--limit` | Max results (default: 10) |
| `--output` | Output format |

**Example**

```bash
loza signatures --symptoms "timeout,5xx_spike" --services "payment-service" --limit 5
```

---

## Debug Commands

### loza debug

Debug utilities for troubleshooting.

```bash
loza debug
```

**Flags**

| Flag | Description |
|---|---|
| `--pprof` | Enable pprof endpoint |
| `--dump-config` | Dump resolved config |

**Example**

```bash
loza debug --dump-config --output json
```

---

### loza bench

Run benchmarks against the collector.

```bash
loza bench
```

**Flags**

| Flag | Description |
|---|---|
| `--rate` | Events per second (default: 1000) |
| `--duration` | Test duration (default: `10s`) |
| `--connections` | Number of concurrent connections (default: 10) |

**Example**

```bash
loza bench --rate 10000 --duration 60s --connections 50
```

---

## Deploy Commands

### loza deploy

Deploy LOZA components.

```bash
loza deploy
```

**Flags**

| Flag | Description |
|---|---|
| `--target` | Deployment target: `docker`, `k8s`, `local` |
| `--component` | Component to deploy: `collector`, `cortex`, `all` |

**Example**

```bash
loza deploy --target k8s --component collector
```

---

### loza dashboard

Open or manage the LOZA dashboard.

```bash
loza dashboard
```

**Flags**

| Flag | Description |
|---|---|
| `--port` | Dashboard port (default: 3000) |
| `--open` | Open in browser |

**Example**

```bash
loza dashboard --port 8080 --open
```

---

## Maturity Command

### loza maturity

Display the maturity status of each LOZA component.

```bash
loza maturity
```

Shows the version, status, and feature completeness of the collector, cortex, CLI, and SDKs.

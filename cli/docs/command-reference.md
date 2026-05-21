# Command Reference

Full reference for all LOXA CLI commands.

## Global Flags

| Flag | Description |
|---|---|
| `--config` | Path to config file (default: `loxa-cli.defaults.yaml`) |
| `--verbose` | Enable verbose logging |
| `--output` | Output format: `text`, `json`, `yaml` (default: `text`) |
| `--no-color` | Disable colored output |

---

## Setup Commands

### loxa init

Initialize a new LOXA workspace. Creates a default config file and validates the environment.

```bash
loxa init
```

**Flags**

| Flag | Description |
|---|---|
| `--dir` | Target directory (default: current directory) |
| `--force` | Overwrite existing config |

**Example**

```bash
loxa init --dir ./my-project
```

---

### loxa dev

Start a local development environment with collector, Cortex, and a DuckDB backend.

```bash
loxa dev
```

**Flags**

| Flag | Description |
|---|---|
| `--port` | Collector port (default: 9090) |
| `--cortex-port` | Cortex port (default: 9091) |
| `--db` | DuckDB file path (default: `loxa-local.db`) |

**Example**

```bash
loxa dev --port 8080 --cortex-port 8081
```

---

## Collector Commands

### loxa collector run

Run the collector binary.

```bash
loxa collector run
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Collector config file path |

**Example**

```bash
loxa collector run -c configs/loxa.local.yaml
```

---

### loxa collector version

Print the collector version.

```bash
loxa collector version
```

---

### loxa collector config print

Print the resolved collector configuration.

```bash
loxa collector config print
```

**Example**

```bash
loxa collector config print --output json
```

---

### loxa collector config validate

Validate a collector config file.

```bash
loxa collector config validate
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Config file to validate |

**Example**

```bash
loxa collector config validate -c configs/loxa.local.yaml
```

---

## Worker Commands

### loxa worker run

Run the queue worker for distributed delivery.

```bash
loxa worker run
```

**Flags**

| Flag | Description |
|---|---|
| `-c`, `--config` | Worker config file path |

**Example**

```bash
loxa worker run -c configs/loxa.queue.kafka.yaml
```

---

### loxa worker version

Print the worker version.

```bash
loxa worker version
```

---

## Config Commands

### loxa config print

Print the resolved CLI configuration.

```bash
loxa config print
```

**Example**

```bash
loxa config print --output yaml
```

---

### loxa config validate

Validate the CLI configuration file.

```bash
loxa config validate
```

---

## Schema Commands

### loxa schema validate

Validate an event payload against the registered schema.

```bash
loxa schema validate
```

**Flags**

| Flag | Description |
|---|---|
| `--file` | Event file to validate |
| `--stdin` | Read event from stdin |
| `--schema` | Schema version to validate against |

**Example**

```bash
cat event.json | loxa schema validate --stdin
loxa schema validate --file event.json --schema v2
```

---

### loxa schema fetch

Fetch the current schema from the collector.

```bash
loxa schema fetch
```

**Flags**

| Flag | Description |
|---|---|
| `--output` | Output file path |
| `--version` | Schema version to fetch |

**Example**

```bash
loxa schema fetch --output schema.json --version v2
```

---

## Emit Commands

### loxa emit

Send a single event to the collector.

```bash
loxa emit
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
loxa emit --service payment-service --type http_request \
  --attr status_code=500 --attr path=/api/charge --attr latency_ms=4500

echo '{"service":"test","event_type":"custom"}' | loxa emit --stdin
```

---

## Query Commands

### loxa query

Execute a SQL query against the collector's DuckDB storage.

```bash
loxa query "SQL_STATEMENT"
```

**Flags**

| Flag | Description |
|---|---|
| `--output` | Output format: `text`, `json`, `csv` (default: `text`) |
| `--limit` | Max rows to return |

**Example**

```bash
loxa query "SELECT service, count(*) FROM events GROUP BY service ORDER BY count(*) DESC"
loxa query "SELECT * FROM events WHERE timestamp > now() - interval '1 hour'" --output json
```

---

### loxa tail

Stream events in real time from the collector.

```bash
loxa tail
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
loxa tail --service payment-service --since 5m
loxa tail --type http_request --filter status_code=500
```

---

### loxa watch

Watch events and display real-time statistics.

```bash
loxa watch
```

**Flags**

| Flag | Description |
|---|---|
| `--interval` | Refresh interval (default: `1s`) |
| `--service` | Filter by service |

**Example**

```bash
loxa watch --interval 2s --service payment-service
```

---

## Status Commands

### loxa status

Show collector status: uptime, version, sinks, event counts.

```bash
loxa status
```

**Example**

```bash
loxa status --output json
```

---

### loxa sinks

List configured sinks and their status.

```bash
loxa sinks
```

**Example**

```bash
loxa sinks --output json
```

---

### loxa doctor

Run health checks against the collector.

```bash
loxa doctor
```

Checks: connectivity, readiness, schema compatibility, sink health, DLQ status.

**Example**

```bash
loxa doctor --verbose
```

---

## DLQ Commands

### loxa dlq

Inspect the Dead Letter Queue.

```bash
loxa dlq
```

**Flags**

| Flag | Description |
|---|---|
| `--limit` | Max entries to show (default: 20) |
| `--output` | Output format |

**Example**

```bash
loxa dlq --limit 50 --output json
```

---

### loxa replay

Replay events from the Dead Letter Queue.

```bash
loxa replay
```

**Flags**

| Flag | Description |
|---|---|
| `--all` | Replay all DLQ entries |
| `--id` | Replay a specific DLQ entry by ID |
| `--limit` | Max entries to replay |

**Example**

```bash
loxa replay --all
loxa replay --id dlq-001
```

---

## GDPR Commands

### loxa delete tenant

Delete all data for a tenant.

```bash
loxa delete tenant <tenant_id>
```

**Example**

```bash
loxa delete tenant acme-corp
```

---

### loxa delete user

Delete all data for a user.

```bash
loxa delete user <user_id>
```

**Example**

```bash
loxa delete user user-12345
```

---

### loxa delete event

Delete a specific event.

```bash
loxa delete event <event_id>
```

**Example**

```bash
loxa delete event evt-abc123
```

---

## Audit Commands

### loxa audit

Run a PII audit on stored events.

```bash
loxa audit
```

**Flags**

| Flag | Description |
|---|---|
| `--service` | Audit a specific service |
| `--since` | Audit events since a time (e.g., `7d`) |
| `--output` | Output format |

**Example**

```bash
loxa audit --service payment-service --since 30d --output json
```

---

### loxa export

Export events from the collector.

```bash
loxa export
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
loxa export --output events.json --format json --service payment-service --since 7d
```

---

## Cortex Commands

### loxa cortex

Query the Cortex control plane.

```bash
loxa cortex
```

**Flags**

| Flag | Description |
|---|---|
| `--url` | Cortex URL (default: from config) |

---

### loxa incident

Reconstruct an incident.

```bash
loxa incident <incident_id>
```

**Flags**

| Flag | Description |
|---|---|
| `--mode` | Reconstruction mode: `fast`, `deep` (default: `fast`) |
| `--output` | Output format |

**Example**

```bash
loxa incident inc-001 --mode deep --output json
```

---

### loxa graph

Query the service graph.

```bash
loxa graph <service>
```

**Flags**

| Flag | Description |
|---|---|
| `--depth` | Graph traversal depth (default: 3) |
| `--output` | Output format |

**Example**

```bash
loxa graph payment-service --depth 5 --output json
```

---

### loxa signatures

Search for similar incident signatures.

```bash
loxa signatures
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
loxa signatures --symptoms "timeout,5xx_spike" --services "payment-service" --limit 5
```

---

## Debug Commands

### loxa debug

Debug utilities for troubleshooting.

```bash
loxa debug
```

**Flags**

| Flag | Description |
|---|---|
| `--pprof` | Enable pprof endpoint |
| `--dump-config` | Dump resolved config |

**Example**

```bash
loxa debug --dump-config --output json
```

---

### loxa bench

Run benchmarks against the collector.

```bash
loxa bench
```

**Flags**

| Flag | Description |
|---|---|
| `--rate` | Events per second (default: 1000) |
| `--duration` | Test duration (default: `10s`) |
| `--connections` | Number of concurrent connections (default: 10) |

**Example**

```bash
loxa bench --rate 10000 --duration 60s --connections 50
```

---

## Deploy Commands

### loxa deploy

Deploy LOXA components.

```bash
loxa deploy
```

**Flags**

| Flag | Description |
|---|---|
| `--target` | Deployment target: `docker`, `k8s`, `local` |
| `--component` | Component to deploy: `collector`, `cortex`, `all` |

**Example**

```bash
loxa deploy --target k8s --component collector
```

---

### loxa dashboard

Open or manage the LOXA dashboard.

```bash
loxa dashboard
```

**Flags**

| Flag | Description |
|---|---|
| `--port` | Dashboard port (default: 3000) |
| `--open` | Open in browser |

**Example**

```bash
loxa dashboard --port 8080 --open
```

---

## Maturity Command

### loxa maturity

Display the maturity status of each LOXA component.

```bash
loxa maturity
```

Shows the version, status, and feature completeness of the collector, cortex, CLI, and SDKs.

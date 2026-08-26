---
name: loza
description: Help release users install and use LOZA end to end: choose Collector versus Cortex, install SDKs and CLI, instrument canonical wide events, query with LQL, operate the CLI, and troubleshoot release behavior. Use for product usage, not repository development.
compatibility: Works from a published LOZA release, matching Docker image, released binary, CLI, or SDK package. Prefer the user's installed release and its bundled documentation over the development repository.
---

# LOZA release-user guide

LOZA is a collector-first wide-event observability stack. Thin application SDKs create canonical events; the Collector validates, redacts, deduplicates, persists, queries, tails, replays, deletes, and fans events to sinks. Cortex is the optional control plane for incident context, causal reconstruction, service graphs, similarity matching, and remediation feedback.

## Component boundary

| Need | Use | Do not use |
|---|---|---|
| Instrument an application | Go, Python, JavaScript/TypeScript, or Rust SDK | Direct database writes |
| Ingest, validate, persist, query, tail, replay, or delete events | Collector | Cortex as the primary event store |
| Incident reconstruction, graph analysis, similar incidents, feedback | Cortex | SDK-side incident logic |
| Install or operate a released stack | Release artifacts, Docker, CLI | `go run`, source paths, CI workflows |
| Develop LOZA itself | Repository docs and build commands | This release-user skill |

Use `loza-collector.md` for data-plane configuration and `loza-cortex.md` for control-plane operation. When a release README conflicts with repository source or roadmap text, the installed release wins.

## Release and installation

Choose one release and keep Collector, Cortex, CLI, and SDK versions compatible. Pin binaries/images in production; avoid `latest`.

Authoritative release and package sources:

- Releases: <https://github.com/Astraive/loza/releases>
- Repository: <https://github.com/Astraive/loza>
- Collector image: <https://hub.docker.com/r/astraive/loza>
- Cortex image: <https://hub.docker.com/r/astraive/loza-cortex>
- Go SDK: <https://pkg.go.dev/github.com/astraive/loza/sdks/go>
- Python SDK: <https://pypi.org/project/loza/>
- JavaScript SDK: <https://www.npmjs.com/package/@astraive/loza>
- Rust SDK: <https://crates.io/crates/loza>

Published SDK installs:

```bash
npm install @astraive/loza
pip install loza
cargo add loza
go get github.com/astraive/loza/sdks/go
```

CLI installers in the repository release contract:

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/Astraive/loza/main/cli/install/install.sh | bash

# Windows PowerShell
irm https://raw.githubusercontent.com/Astraive/loza/main/cli/install/install.ps1 | iex
```

For offline or reproducible installation, download the release archive and its checksum, verify the checksum, and install the matching artifact. Do not execute an unreviewed remote installer in a restricted environment.

## First-use sequence

1. Start a released Collector; configure durable storage and authentication first.
2. Verify `/health` and `/readyz`; check `/version` when available.
3. Configure an SDK with service identity, Collector URL, and a least-privilege ingest credential.
4. Emit one small canonical event and verify it with `loza query`.
5. Use `loza tail` or `loza watch` for live delivery.
6. Add Cortex only for incident intelligence or graph workflows.
7. Configure retention, redaction, rate limits, and monitoring before production traffic.

The health endpoint proves process liveness, not authentication, storage, or sink delivery. Always verify one authenticated write and one read against the intended collector/environment.

## Canonical event model

Required identity varies by SDK/configuration, but a useful event includes:

```json
{
  "event_id": "evt_01",
  "event": "checkout.request",
  "kind": "http",
  "service": "checkout",
  "environment": "production",
  "release": "2026.08.26",
  "timestamp": "2026-08-26T12:00:00Z",
  "outcome": "success",
  "duration_ms": 42,
  "trace_id": "trace-01",
  "span_id": "span-01",
  "attrs": {"order_id": "ord-01"}
}
```

Use stable event names and typed attributes. Keep secrets, access tokens, raw authorization headers, payment data, and unnecessary PII out of custom attributes. Let the Collector enforce the final redaction and schema policy. Preserve `event_id` for idempotency and trace/request identifiers for correlation.

Event lifecycle is start → enrich → checkpoint/process/group/timer/link as needed → finish or finish-error → emit. A request handler should start one event and enrich it; a lower-level function should not create duplicate events for the same operation unless it represents a separate operation. Always flush/shutdown the SDK during graceful process termination.

## SDK API inventory

The cross-language lifecycle names are intentionally parallel:

- `StartEvent` / `startEvent`: generic event creation.
- `StartHTTPEvent` and Go `StartHTTPEventFromRequest`: HTTP defaults and request/trace extraction.
- `StartJobEvent` / `startJobEvent`: background work.
- `StartQueueEvent` / `startQueueEvent`: queue processing.
- `StartCLIEvent` / `startCLIEvent`: command execution.
- `StartCronEvent` / `startCronEvent`: scheduled work.
- `StartJob` and `StartQueueJob`: named convenience wrappers where provided.
- `Enrich`: add typed fields to the event in context.
- `Checkpoint`, `Process`, `Group`, `Timer`, and `Link`: record lifecycle relationships and timing.
- `Finish`, `FinishError`, and `Emit`/`EmitEvent`: close and deliver the event according to the SDK state machine.
- `Flush` and `Shutdown`: drain the configured sink and release resources.

### Go

Package: `github.com/astraive/loza/sdks/go`. Public factories/configuration include `New`, `TryNew`, `CreateLoza`, `NewClient`, `Configure`, `Default`, `SetDefault`, `Reset`, `LoadFromFile`, `Dev`, `Production`, `Test`, and `ApplyConfig`. Lifecycle helpers include `StartEvent(ctx, params)`, `StartHTTPEvent(ctx, params)`, `StartHTTPEventFromRequest(r, params)`, `StartJobEvent(ctx, params)`, `StartQueueEvent(ctx, params)`, `StartCLIEvent(ctx, params)`, `StartCronEvent(ctx, params)`, `StartJob`, `StartQueueJob`, `StartCron`, `Enrich`, `Checkpoint`, `Process`, `Group`, `Timer`, `Link`, `Finish`, `FinishError`, `Emit`, `Flush`, and `Shutdown`. `StartHTTPEventFromRequest` takes the `*http.Request` first and derives its context; it is not passed a separate context argument.

Typical use:

```go
cfg := loza.Production().WithService("checkout").WithCollectorEndpoint("http://127.0.0.1:9308")
if err := loza.Configure(cfg); err != nil { panic(err) }
defer loza.Shutdown(context.Background())

ctx := loza.StartHTTPEvent(context.Background(), loza.Params{Event: "checkout.request"})
loza.Enrich(ctx, loza.String("order_id", orderID))
if err := doCheckout(ctx); err != nil {
    loza.FinishError(ctx, err)
    return err
}
loza.Finish(ctx, "success")
```

Use the SDK's typed attribute helpers rather than constructing untyped maps when possible. Use `loza.TestLogger`, `loza.Capture`, and `MemorySink` only in application tests.
Go attribute helpers are part of the public API: primitive constructors `String`, `Int`, `Int64`, `Uint64`, `Float64`, `Bool`, `Time`, `Duration`, `Any`, `Null`, `Err`, `Stringer`, and `Group`; canonical constructors such as `RequestID`, `TraceID`, `SpanID`, `Service`, `Version`, `DeploymentID`, `Region`, `Method`, `Path`, `Route`, `IncidentID`, `StatusCode`, `DurationMS`, and `Outcome`; domain constructors including `UserID`, `TenantID`, `WorkspaceID`, `OrganizationID`, `SessionID`, `CartID`, `OrderID`, `ProductID`, `CustomerID`, `Plan`, `Currency`, `Amount`, `PaymentProvider`, `JobName`, `QueueName`, `MessageID`, `Attempt`, and error helpers; and privacy markers `MarkSensitive`, `SensitiveString`, and `HashString`. Prefer these constructors to arbitrary keys so the Collector and cross-language queries see consistent field paths.

Framework adapters are separate optional packages. Use `loza/sdks/go/middleware/nethttp`, or the matching `gin`, `echo`, `fiber`, `chi`, or `grpc` adapter, when the release package includes it. Middleware starts/completes one canonical request/RPC event and can recover panics according to its configured policy; do not also create a duplicate manual event around the same handler.

### Python

Package: `loza`. The documented surface includes `configure`, `production`, `development`, `test`, `start_event`, `start_http_event`, `start_job_event`, `start_queue_event`, `start_cli_event`, `start_cron_event`, `enrich`, `checkpoint`, `process`, `group`, `timer`, `link`, `finish`, `finish_error`, `emit`, `flush`, and `shutdown`, plus typed identity/attribute helpers and sink builders. Configure once at process startup, use the returned/context event handle through the operation, and call shutdown in a `finally`/application teardown hook.

### JavaScript/TypeScript

Package: `@astraive/loza`. The documented surface mirrors the lifecycle API with camelCase names: `configure`, `createLoza`, `getDefault`, `startEvent`, `startHttpEvent`, `startJobEvent`, `startQueueEvent`, `startCliEvent`, `startCronEvent`, `enrich`, `checkpoint`, `process`, `group`, `timer`, `link`, `finish`, `finishError`, `emit`, `flush`, and `shutdown`. Use the async variants where the selected sink is asynchronous. Keep browser credentials scoped and origin-restricted; prefer server-side instrumentation for secrets and durable delivery.

### Rust

Crate: `loza`. Use `Config`, `production`, `development`, or `test`; create an event with `start_event` and `Params`; then use `enrich`, lifecycle primitives, `finish`/`finish_error`, `emit`, `flush`, and `shutdown`. Rust integrations expose middleware/layer adapters where enabled by the package feature set. Check the installed crate README for feature flags before enabling a framework adapter.

## Configuration precedence and deployment

The Collector and SDK configuration contracts use this order unless a release README says otherwise:

1. Explicit code/options.
2. Environment variables (`LOZA_*` and component-specific secret variables).
3. Config file, commonly `./loza.yaml`.
4. Release defaults.

Keep API keys, server secrets, database passwords, and storage encryption keys outside committed YAML. Use separate credentials for ingest, read/query, administration, Cortex-to-Collector access, and storage. Set bounded event/body sizes, request timeouts, retries, rate limits, and durable paths. See `loza-collector.md` for the full Collector key map and `loza-cortex.md` for Cortex configuration.

## CLI catalog

Global options may appear before or after the command:

```text
--verbose
--output=json|table|text
```

Command routing in the released CLI:

- `loza version`, `loza help`, `loza maturity`: version, help, and maturity metadata.
- `loza init [--validation-mode off|warn|enforce|quarantine]`: initialize local configuration.
- `loza config print|validate [--validation-mode ...]`: print/validate Collector configuration.
- `loza collector run|config print|config validate`: operate a configured Collector binary; normally a local/source workflow, not a package install.
- `loza worker run`: operate a configured worker binary; normally a local/source workflow.
- `loza emit sample [--service S] [--event E] [--kind K] [--outcome O] [--level L] [--attrs JSON] [--print] [--output FILE]`: create/send a sample event.
- `loza query [-q LQL] [--engine duckdb|clickhouse] [--raw-sql] [--format table|json|csv] [--limit N] [--param key=value]`: query through Collector LQL or explicit raw SQL where supported.
- `loza tail [--kind K] [--service S] [--level L]`: stream events.
- `loza watch [--kind K] [--service S] [--level L]`: live formatted event viewer.
- `loza status`: show Collector status.
- `loza sinks list|show NAME|test NAME`: inspect or test sink health.
- `loza dlq list|show ID|delete ID|replay ID|replay-all`: inspect and manage dead-letter items.
- `loza quarantine list|replay ID|delete ID`: manage schema-quarantined events.
- `loza replay [--source FILE]`: replay NDJSON from a file or configured Collector DLQ.
- `loza delete tenant|user|event ID [--reason TEXT]`: perform destructive event deletion; verify scope first.
- `loza audit pii [--limit N]`: scan stored events for PII findings.
- `loza schema validate|fetch|list|diff|publish`: validate/fetch/list/compare/publish schemas; `loza schema blueprint add|list|publish` manages blueprints.
- `loza export [--engine E] [-o FILE] [--format ndjson|parquet] [--table events]`: export stored data.
- `loza keys create|revoke ID|rotate ID`: administer API keys; protect these commands.
- `loza incident ID [--mode fast|deep] [--depth N] [--format json|text]`: incident-oriented Cortex output.
- `loza graph service|incident NAME_OR_ID [--depth N] [--format json|ascii]`: graph output.
- `loza signatures [--limit N]`: list incident signatures.
- `loza cortex run|ingest|reconstruct|similar|remediation|feedback|graph|replay`: Cortex operations; `cortex replay` currently returns an explicit not-implemented error in the checked-in CLI, so use `cortex ingest` or the API instead.
- `loza doctor [--metrics]`: component health checks.
- `loza bench [--url URL] [--rate N] [--duration D] [--size BYTES] [--batch-size N]`: load generation; never aim it at production without approval.
- `loza deploy up|down|status|logs|render KIND [--out DIR]`: deployment asset/local lifecycle commands; inspect the installed CLI help because deployment backends are release-specific.
- `loza dashboard ...`, `loza debug event|pipeline|cortex ...`: experimental diagnostics/integration surfaces; do not treat them as stable API.

Use `loza <command> --help` for the installed binary's exact parser behavior. Do not put credentials in command history; use environment variables or a secret manager.

## LQL quick reference

LQL is the high-level query language compiled for supported Collector targets. Start with a bounded query:

```lql
from events
where service = "checkout" and level = "error"
summarize count() by event
sort count desc
limit 100
```

Use `cargo run --bin lql` only when developing the LQL repository; release users use `loza query` or the Collector's LQL endpoint. Keep user-supplied values in typed `--param key=value` parameters where supported. Never concatenate untrusted values into raw SQL. Query only authorized collector scopes and keep limits/time windows bounded.

## Troubleshooting order

1. Confirm all component versions and image tags.
2. Check Collector `/health`, `/readyz`, and `/version`; then check Cortex `/healthz`, `/readyz`, and `/version`.
3. Run `loza doctor` and inspect the configured URLs, environment, collector scope, and key kind/permissions.
4. Verify one small authenticated event, then one bounded query.
5. Inspect readiness, metrics, sink health, spool/queue growth, and storage free space without exposing tokens or raw sensitive events.
6. Reproduce with one event and one sink before changing batching/retry/durability settings.
7. Restart after configuration changes unless the installed release explicitly documents live reload.

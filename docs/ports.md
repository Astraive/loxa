# Loxa Port Map

> All ports are configurable. Values below are canonical defaults.

## Loxa Backend Services

| Port | Service | Protocol | Config Key | Env Var |
|------|---------|----------|------------|---------|
| 9308 | Collector HTTP | HTTP | `server.port` | `COLLECTOR_SERVER_PORT` |
| 9309 | Collector gRPC | gRPC | `grpc.port` | `COLLECTOR_GRPC_PORT` |
| 9310 | Collector GraphQL | HTTP | `graphql.port` | `COLLECTOR_GRAPHQL_PORT` |
| 9311 | Collector Metrics | HTTP | `metrics.port` | `COLLECTOR_METRICS_PORT` |
| 9312 | Cortex HTTP | HTTP | `server.port` | `CORTEX_SERVER_PORT` |
| 9313 | Cortex gRPC | gRPC | `grpc.port` | `CORTEX_GRPC_PORT` |
| 9314 | Cortex WebSocket | WS | `websocket.port` | `CORTEX_WS_PORT` |
| 9315 | Schema Service | HTTP | — | `SCHEMA_SERVICE_PORT` |
| 9316 | Dev Tools | HTTP | — | Reserved |
| 9317 | Worker Admin | HTTP | — | Reserved |
| 9318 | Internal Gateway | HTTP | — | Reserved |
| 9319 | Reserved | — | — | Reserved |

## UI

| Port | Service |
|------|---------|
| 3000 | Loxana (dev) |
| 80 | Loxana (prod/nginx) |

## External Infrastructure

| Port | Service | Notes |
|------|---------|-------|
| 3100 | Loki | Log aggregation |
| 4222 | NATS | Client connections |
| 4317 | OTLP gRPC | OpenTelemetry ingestion |
| 4318 | OTLP HTTP | OpenTelemetry ingestion |
| 5432 | PostgreSQL | Cortex storage |
| 6379 | Redis | Deduplication / cache |
| 8222 | NATS monitoring | Health / monitoring |
| 9092 | Kafka | Event streaming |

## Configuration

Loxa uses a layered config system. All non-secret values can be set via YAML files, environment variables, or code. Secrets must come from environment variables.

### 1. Defaults (committed)

`loxa-collector.defaults.yaml`, `loxa-cortex.defaults.yaml`, `loxa-cli.defaults.yaml` — canonical defaults shipped with the project. Do not edit these.

### 2. User config (any file name)

Users can create config files with any name. The SDKs search for standard names as a convenience, but any path works.

**Server components (Collector, Cortex, CLI):**

```bash
# Default search: loxa.yaml in current directory
./loxa-collector run

# Custom path via flag
./loxa-collector run -c /etc/loxa/production.yaml
./cortex --config /etc/cortex/prod.yaml
```

**SDKs:**

```go
// Go — custom path
fileCfg, _ := loxa.LoadFromFile("/etc/loxa/prod.yaml")
```

```python
# Python — custom path via env var
LOXA_PY_CONFIG=/etc/loxa/prod.yaml python app.py
```

```typescript
// JS — custom path via env var
LOXA_JS_CONFIG=/etc/loxa/prod.yaml node app.js
```

```rust
// Rust — custom path via env var
LOXA_RS_CONFIG=/etc/loxa/prod.yaml cargo run
```

**Docker:**

```bash
docker run -v /host/config.yaml:/etc/loxa/config.yaml \
  -e LOXA_COLLECTOR_CONFIG=/etc/loxa/config.yaml \
  ghcr.io/astraive/loxa:latest
```

### 3. Environment variables

All config values can be overridden via env vars. Secrets must use env vars.

```bash
# Non-secret overrides
COLLECTOR_SERVER_PORT=9308
CORTEX_SERVER_PORT=9312
LOXA_ENVIRONMENT=production

# Secrets (env vars only)
LOXA_API_KEY=lx_sec_live_xxx
LOXA_STORAGE_ENCRYPTION_KEY=xxx
CORTEX_POSTGRES_PASSWORD=xxx
```

### 4. Code (SDKs)

Highest precedence — overrides everything.

```go
loxa.Configure(loxa.Production("checkout").
    WithCollectorEndpoint("http://localhost:9308").
    WithAPIKey(os.Getenv("LOXA_API_KEY")))
```

```python
loxa.configure(loxa.production("checkout")
    .with_collector_endpoint("http://localhost:9308")
    .with_api_key(os.environ["LOXA_API_KEY"]))
```

```typescript
loxa.configure(loxa.production('checkout')
    .withCollectorEndpoint('http://localhost:9308')
    .withApiKey(process.env.LOXA_API_KEY!));
```

```rust
loxa::configure(loxa::Config::production("checkout")
    .with_collector_endpoint("http://localhost:9308")
    .with_api_key(std::env::var("LOXA_API_KEY")?));
```

## loxa:// DSN

The `loxa://` DSN defaults to port 9308 for localhost:

```
loxa://localhost/my-app          → http://localhost:9308
loxa://localhost:9999/my-app     → http://localhost:9999
```

Set via env var: `LOXA_DSN=loxa://localhost/my-app`

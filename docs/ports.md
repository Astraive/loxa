# Loza Port Map

> All ports are configurable. Values below are canonical defaults.

## Loza Backend Services

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
| 3000 | Lozana (dev) |
| 80 | Lozana (prod/nginx) |

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

Loza uses a layered config system. Non-secret values can be set via YAML files, environment variables, or code. Prefer secret environment variables or a secret manager for credentials. A credentialed `LOZA_DSN` may carry percent-encoded Basic-auth userinfo, but it must not be committed or logged.

### 1. Defaults (committed)

`loza-collector.defaults.yaml`, `loza-cortex.defaults.yaml`, `loza-cli.defaults.yaml` — canonical defaults shipped with the project. Do not edit these.

### 2. User config (any file name)

Users can create config files with any name. The SDKs search for standard names as a convenience, but any path works.

**Server components (Collector, Cortex, CLI):**

```bash
# Default search: loza.yaml in current directory
./loza-collector run

# Custom path via flag
./loza-collector run -c /etc/loza/production.yaml
./cortex --config /etc/cortex/prod.yaml
```

**SDKs:**

```go
// Go — custom path
fileCfg, _ := loza.LoadFromFile("/etc/loza/prod.yaml")
```

```python
# Python — custom path via env var
LOZA_PY_CONFIG=/etc/loza/prod.yaml python app.py
```

```typescript
// JS — custom path via env var
LOZA_JS_CONFIG=/etc/loza/prod.yaml node app.js
```

```rust
// Rust — custom path via env var
LOZA_RS_CONFIG=/etc/loza/prod.yaml cargo run
```

**Docker:**

```bash
docker run -v /host/config.yaml:/etc/loza/config.yaml \
  -e LOZA_COLLECTOR_CONFIG=/etc/loza/config.yaml \
  ghcr.io/astraive/loza:latest
```

### 3. Environment variables

All config values can be overridden via env vars. Keep API keys and DSN credentials in environment variables or a secret manager.

```bash
# Non-secret overrides
COLLECTOR_SERVER_PORT=9308
CORTEX_SERVER_PORT=9312
LOZA_ENVIRONMENT=production

# Secrets
LOZA_API_KEY=lx_sec_live_xxx
# Optional Basic-auth form; percent-encode reserved password characters
LOZA_DSN='loza://kingest:s%40cret%3Avalue@collector.example.com/my-app?env=prod'
LOZA_STORAGE_ENCRYPTION_KEY=xxx
CORTEX_POSTGRES_PASSWORD=xxx
```

### 4. Code (SDKs)

Credential precedence is: explicit code API key or Basic credentials, then credentials in an explicitly supplied code DSN, then environment credentials (`LOZA_API_KEY` or userinfo in `LOZA_DSN`). `LOZA_API_KEY` remains the highest-priority token credential. `LOZA_COLLECTOR_URL` overrides only the endpoint and does not replace DSN-derived environment, service, or credentials. A DSN without userinfo does not clear separately configured credentials.

Explicit code configuration otherwise remains the highest-precedence source for the fields it sets:

```go
loza.Configure(loza.Production("checkout").
    WithCollectorEndpoint("http://localhost:9308").
    WithAPIKey(os.Getenv("LOZA_API_KEY")))
```

```python
loza.configure(loza.production("checkout")
    .with_collector_endpoint("http://localhost:9308")
    .with_api_key(os.environ["LOZA_API_KEY"]))
```

```typescript
loza.configure(loza.production('checkout')
    .withCollectorEndpoint('http://localhost:9308')
    .withApiKey(process.env.LOZA_API_KEY!));
```

```rust
loza::configure(loza::Config::production("checkout")
    .with_collector_endpoint("http://localhost:9308")
    .with_api_key(std::env::var("LOZA_API_KEY")?));
```

## loza:// DSN

The `loza://` DSN defaults to port 9308 for localhost:

```
loza://localhost/my-app          → http://localhost:9308
loza://localhost:9999/my-app     → http://localhost:9999
```

Credentialed DSNs use `loza://username:password@host/project`. The username is the Collector `key_id`; the password is its secret. SDKs send Basic auth over TLS by default and reject plaintext HTTP with credentials unless explicitly local (`tls=false`/insecure). Resolved endpoint URLs and logs never contain the password.

Set via env var: `LOZA_DSN=loza://localhost/my-app`

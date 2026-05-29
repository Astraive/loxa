# Authentication

## Overview

LOXA uses scoped API keys with the `Authorization: Bearer` header for all ingest and control-plane requests. Keys follow the `lx_` format and carry built-in RBAC roles and ABAC restrictions.

```
Authorization: Bearer lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
```

## Key Format

```
lx_{kind}_{env}_{key_id}_{secret}
```

| Component | Values | Example |
|-----------|--------|---------|
| Prefix | `lx` | `lx` |
| Kind | `pub`, `sec`, `local` | `sec` |
| Env | `live`, `test`, `dev` | `live` |
| Key ID | Single segment, no underscores | `k2M9aQp` |
| Secret | Arbitrary length | `7QmVxN8pT4zRbK1sYw` |

### Examples

```
lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw    # Backend server (production)
lx_pub_live_kabc123_xxxxx                    # Frontend/browser (production)
lx_sec_test_ktest1_testsecret                # Backend server (test)
lx_local_dev_mydevtoken                      # Local development only
```

## Key Types

| Kind | Prefix | Use Case | Permissions |
|------|--------|----------|-------------|
| `sec` | `lx_sec_` | Backend/server SDKs | Full ingest (events, logs, traces, metrics) |
| `pub` | `lx_pub_` | Frontend/browser/mobile | Limited ingest (events + heartbeat only) |
| `local` | `lx_local_` | Local development | Full ingest (blocked in production) |

### Secret Keys (`sec`)

For backend services that send events, logs, traces, and metrics. These keys have full ingest permissions and can include PII and attachments.

```
lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
```

### Public Keys (`pub`)

For frontend applications exposed in browsers. Public keys are restricted:

- Only `events:write` and `heartbeat:write` permissions
- PII is never allowed (cannot be overridden)
- Attachments are never allowed (cannot be overridden)
- Origin allowlist is required
- Lower rate limits and smaller payload limits

```
lx_pub_live_kabc123_xxxxx
```

### Local Keys (`local`)

For local development. Full ingest permissions but blocked in production environments.

```
lx_local_dev_mydevtoken
```

## SDK Configuration

Set the `LOXA_API_KEY` environment variable, then configure the SDK:

### Go

```go
import (
    "os"

    loxa "github.com/astraive/loxa/sdks/go"
)

loxa.Configure(loxa.Production("checkout-api").
    WithCollectorEndpoint("https://collector.example.com").
    WithAPIKey(os.Getenv("LOXA_API_KEY")))
```

Or create a custom instance:

```go
logger, _ := loxa.New(loxa.Config{
    Service:        "checkout-api",
    CollectorURL:   "https://collector.example.com",
    APIKey:         os.Getenv("LOXA_API_KEY"),
})
```

### Python

```python
import os, loxa

loxa.configure(
    loxa.production("checkout-api")
    .with_collector_endpoint("https://collector.example.com")
    .with_api_key(os.environ["LOXA_API_KEY"])
)
```

Or create a custom instance:

```python
logger = loxa.create_loxa(
    service="checkout-api",
    collector_endpoint="https://collector.example.com",
    api_key=os.environ["LOXA_API_KEY"],
)
```

### Rust

```rust
loxa::configure(
    loxa::Config::production("checkout-api")
        .with_collector_endpoint("https://collector.example.com")
        .with_api_key(std::env::var("LOXA_API_KEY")?)
)?;
```

Or create a custom instance:

```rust
let logger = loxa::create_loxa(
    loxa::Config::production("checkout-api")
        .with_collector_endpoint("https://collector.example.com")
        .with_api_key(std::env::var("LOXA_API_KEY")?)
);
```

### JavaScript

```typescript
import { loxa } from "loxa";

loxa.configure(
    loxa.production("checkout-api")
        .withCollectorEndpoint("https://collector.example.com")
        .withApiKey(process.env.LOXA_API_KEY!)
);
```

Or create a custom instance:

```typescript
import { createLoxa } from "loxa";

const logger = createLoxa({
    service: "checkout-api",
    collectorUrl: "https://collector.example.com",
    apiKey: process.env.LOXA_API_KEY!,
});
```

## Headers

When an API key is configured, the SDK automatically sends these headers:

| Header | Value | Description |
|--------|-------|-------------|
| `Authorization` | `Bearer lx_sec_live_k_xxx_yyyy` | The API key |
| `X-Loxa-Service` | `checkout-api` | Service name from config |
| `X-Loxa-Env` | `prod` | Environment from config |

## Collector Configuration

The collector validates API keys using the RBAC+ABAC auth system. Configure it in the collector YAML:

```yaml
auth:
  enabled: true
  server_secret: "${COLLECTOR_SERVER_SECRET}"
  cache_ttl: 60s
  negative_cache_ttl: 10s
  keys:
    - name: "prod-backend"
      key_id: "k2M9aQp"
      secret_env: "PROD_KEY_SECRET"
      kind: "sec"
      roles: ["collector_ingest_server"]
      allowed_envs: ["prod"]
      allowed_services: ["checkout-api", "payment-api"]
      max_requests_per_minute: 5000
      max_events_per_minute: 50000
      max_payload_bytes: 262144
    - name: "web-dashboard"
      key_id: "kabc123"
      secret_env: "WEB_KEY_SECRET"
      kind: "pub"
      roles: ["collector_ingest_public"]
      allowed_origins: ["https://app.example.com"]
      max_requests_per_minute: 600
      max_events_per_minute: 1000
      max_payload_bytes: 65536
```

### Disabling Auth for Local Development

```yaml
auth:
  enabled: false
```

Or with dev mode (accepts `local` keys):

```yaml
auth:
  enabled: true
  dev_mode: true
```

## Token Storage Security

The collector never stores raw API keys. Keys are stored as HMAC-SHA256 hashes:

```
hash = hmac_sha256(server_secret, token_secret)
```

- Server secret from `COLLECTOR_SERVER_SECRET` environment variable
- Constant-time comparison via `crypto/subtle.ConstantTimeCompare`
- Positive cache TTL: 60 seconds
- Negative cache TTL: 10 seconds

## Key Rotation

1. Create a new key in the dashboard (or control plane API)
2. Deploy your application with the new key
3. The collector accepts both keys during the overlap window
4. Revoke the old key after all deployments are updated

## Backward Compatibility

The collector supports `X-API-Key` as a fallback header for legacy clients. This is deprecated — use `Authorization: Bearer` for all new integrations.

```
# Deprecated (still works)
X-API-Key: lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw

# Recommended
Authorization: Bearer lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
```

## Error Responses

Authentication failures return HTTP 401 with diagnostic headers:

```
HTTP/1.1 401 Unauthorized
X-Auth-Failure-Reason: Key not found
X-Auth-Failure-Code: key_not_found
Content-Type: application/json

{"error": "unauthorized"}
```

### Failure Codes

| Code | Reason |
|------|--------|
| `missing_token` | No Authorization header present |
| `invalid_key_format` | Key doesn't match `lx_` format |
| `key_not_found` | Key ID not found in store |
| `key_revoked` | Key has been revoked |
| `key_expired` | Key has expired |
| `key_kind_mismatch` | Key kind prefix doesn't match DB record |
| `invalid_secret` | HMAC verification failed |
| `env_not_allowed` | `X-Loxa-Env` not in `allowed_envs` |
| `service_not_allowed` | `X-Loxa-Service` not in `allowed_services` |
| `origin_not_allowed` | Origin not in `allowed_origins` |
| `ip_not_allowed` | Client IP not in `allowed_ips` |
| `rate_limited` | Per-key rate limit exceeded |

## Local Development

For local development, use a `local` key with auth disabled or in dev mode:

```bash
export LOXA_API_KEY="lx_local_dev_mydevtoken"
```

```go
client, _ := loxa.New(loxa.Config{
    Service:  "test-service",
    Sink:     sink,
    APIKey:   "lx_local_dev_mydevtoken",
    Env:      "dev",
    Insecure: true,
})
```

## Full Specification

See [spec/auth/spec.md](../spec/auth/spec.md) for the complete authentication specification including validation flow, gRPC metadata, and future HMAC signed requests.

## Next Steps

- [Authorization](authorization.md) — RBAC roles, permissions, and ABAC restrictions
- [Security](security.md) — Architecture overview and redaction controls
- [Configuration](configuration.md) — Full collector and SDK configuration reference

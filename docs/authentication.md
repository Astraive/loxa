# Authentication

## Overview

LOZA uses scoped API keys with the `Authorization: Bearer` header for all ingest and control-plane requests. Keys follow the `lx_` format and carry built-in RBAC roles and ABAC restrictions.

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

Set the `LOZA_API_KEY` environment variable, then configure the SDK:

### Go

```go
import (
    "os"

    loza "github.com/astraive/loza/sdks/go"
)

loza.Configure(loza.Production("checkout-api").
    WithCollectorEndpoint("https://collector.example.com").
    WithAPIKey(os.Getenv("LOZA_API_KEY")))
```

Or create a custom instance:

```go
logger, _ := loza.New(loza.Config{
    Service:        "checkout-api",
    CollectorURL:   "https://collector.example.com",
    APIKey:         os.Getenv("LOZA_API_KEY"),
})
```

### Python

```python
import os, loza

loza.configure(
    loza.production("checkout-api")
    .with_collector_endpoint("https://collector.example.com")
    .with_api_key(os.environ["LOZA_API_KEY"])
)
```

Or create a custom instance:

```python
logger = loza.create_loza(
    service="checkout-api",
    collector_endpoint="https://collector.example.com",
    api_key=os.environ["LOZA_API_KEY"],
)
```

### Rust

```rust
loza::configure(
    loza::Config::production("checkout-api")
        .with_collector_endpoint("https://collector.example.com")
        .with_api_key(std::env::var("LOZA_API_KEY")?)
)?;
```

Or create a custom instance:

```rust
let logger = loza::create_loza(
    loza::Config::production("checkout-api")
        .with_collector_endpoint("https://collector.example.com")
        .with_api_key(std::env::var("LOZA_API_KEY")?)
);
```

### JavaScript

```typescript
import { loza } from "loza";

loza.configure(
    loza.production("checkout-api")
        .withCollectorEndpoint("https://collector.example.com")
        .withApiKey(process.env.LOZA_API_KEY!)
);
```

Or create a custom instance:

```typescript
import { createLoza } from "loza";

const logger = createLoza({
    service: "checkout-api",
    collectorUrl: "https://collector.example.com",
    apiKey: process.env.LOZA_API_KEY!,
});
```

## Headers

When an API key is configured, the SDK automatically sends these headers:

| Header | Value | Description |
|--------|-------|-------------|
| `Authorization` | `Bearer lx_sec_live_k_xxx_yyyy` | The API key |
| `X-Loza-Service` | `checkout-api` | Service name from config |
| `X-Loza-Env` | `prod` | Environment from config |

## Collector Configuration

The Collector validates configured key records using RBAC and ABAC. `server_secret` is an independent HMAC key; it is not the at-rest storage key. `secret_env` names hold only the token secret, so a configured key is presented as `lx_{kind}_live_{key_id}_${SECRET_ENV}`.

```yaml
auth:
  enabled: true
  server_secret: "${COLLECTOR_AUTH_SERVER_SECRET}"
  cache_ttl: 5m
  negative_cache_ttl: 30s
  keys:
    - name: ingest
      key_id: kingest
      secret_env: COLLECTOR_INGEST_KEY_SECRET
      kind: sec
      roles: [collector_ingest_server]
    - name: administrator
      key_id: kadmin
      secret_env: COLLECTOR_ADMIN_KEY_SECRET
      kind: sec
      roles: [project_admin]
    - name: browser
      key_id: collector_browser
      secret_env: COLLECTOR_BROWSER_KEY_SECRET
      kind: pub
      roles: [collector_ingest_public]
      allowed_origins: [https://app.example.com]
storage:
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
```

All configured key IDs must be unique. `kind` is `sec` or `pub`; public keys require a non-empty origin allowlist. Auth startup requires a non-empty resolved server secret, every configured key secret, and the storage encryption key. An explicit `auth.enabled: false` remains an operator override, not a tracked default.

## Token Storage Security

The collector never stores raw API keys. Keys are stored as HMAC-SHA256 hashes:

```
hash = hmac_sha256(server_secret, token_secret)
```

- Server secret from `COLLECTOR_AUTH_SERVER_SECRET`, resolved through `auth.server_secret`
- Token secrets from each configured `secret_env`
- The independent raw-event encryption key from `LOZA_STORAGE_ENCRYPTION_KEY`
- Constant-time comparison via `crypto/subtle.ConstantTimeCompare`

## Key Rotation

1. Create a new key in the dashboard (or control plane API)
2. Deploy your application with the new key
3. The collector accepts both keys during the overlap window
4. Revoke the old key after all deployments are updated

## Legacy Bootstrap Token

For a single-key bootstrap only, `auth.value` or `auth.value_env` may supply a complete valid LOZA token. It is parsed and assigned the `collector_ingest_server` role. New deployments should use `auth.keys`; raw, non-LOZA legacy values are rejected when authentication is active.

## Error Responses

Protected routes return one non-sensitive unauthenticated wire contract. Detailed authentication failures are logged server-side and are never echoed to a client.

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
X-Auth-Failure-Code: unauthorized
X-Auth-Failure-Reason: authentication required

{"error":"unauthorized"}
```

## Local Development

For local development, either deliberately disable auth or set `allow_local_dev_keys: true` alongside enabled auth. Local keys are rejected by default.

```bash
export LOZA_API_KEY="lx_local_dev_mydevtoken"
```

```go
client, _ := loza.New(loza.Config{
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

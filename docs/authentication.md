# Authentication

## Overview

LOZA supports scoped API keys and opaque bearer tokens with the `Authorization: Bearer` header, plus credentialed DSNs with HTTP Basic authentication. API keys and tokens resolve an RBAC role server-side; credentialed DSNs supply the Collector key ID and its secret.

Bearer example:

```http
Authorization: Bearer lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
```

Credentialed DSN example:

```text
loza://kingest:s%40cret%3Avalue@collector.example.com/payments?env=prod
```

In this PostgreSQL-style form, `username` is the Collector configured `key_id` and `password` is that key's secret. SDKs percent-decode both values; empty values are invalid, username `:`/whitespace is invalid, and URL-reserved password characters must be percent-encoded. SDKs send these credentials as `Authorization: Basic base64(username:password)` over TLS by default. Plain HTTP with credentials is rejected during SDK configuration unless explicitly local (`tls=false`/insecure).

Credentialed DSN passwords are never included in resolved endpoint URLs or normal DSN/debug output, and must never be logged. Use a secret environment variable or secret manager for DSNs and do not commit them.

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

## RBAC roles and credential modes

| Role | Log permissions |
| --- | --- |
| `user` | `logs:read` |
| `client` | `logs:read`, `logs:write` |
| `admin` | `logs:read`, `logs:write`, `logs:edit`, `logs:delete` |
| `superadmin` | all `admin` log permissions plus `project:admin` |

Credentials use `mode: private` or `mode: public`. Private credentials can
use any recognized role. Public credentials require an origin allowlist and
are limited to the least-privileged `client` role; never assign `admin` or
`superadmin` to a browser-exposed credential.

An opaque token is configured by environment variable and sent verbatim as a
Bearer value. LOZA derives its lookup ID and its stored HMAC from the token,
so the raw value is neither persisted as an identifier nor emitted in logs.

## SDK Configuration

Set `LOZA_API_KEY` for the existing Bearer-token flow, or set `LOZA_DSN` to a credentialed DSN for Basic auth. Explicit SDK credentials can be supplied in code as shown below.

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
import { loza } from "@astraive/loza";

loza.configure(
    loza.production("checkout-api")
        .withCollectorEndpoint("https://collector.example.com")
        .withApiKey(process.env.LOZA_API_KEY!)
);
```

Or create a custom instance:

```typescript
import { createLoza } from "@astraive/loza";

const logger = createLoza({
    service: "checkout-api",
    collectorUrl: "https://collector.example.com",
    apiKey: process.env.LOZA_API_KEY!,
});
```

## Credential Precedence

Credential sources are applied in this order:

1. Explicit code API key or Basic credentials.
2. Credentials in an explicitly supplied code DSN.
3. Environment credentials (`LOZA_API_KEY` and userinfo in `LOZA_DSN`).

`LOZA_API_KEY` remains the highest-priority token credential. `LOZA_COLLECTOR_URL` changes only the endpoint; it does not override DSN-derived environment, service, or credentials. A DSN without userinfo does not clear credentials configured separately.

## Headers

When an API key is configured, the SDK sends Bearer authentication. When a credentialed DSN is configured, it sends Basic authentication from the DSN userinfo:

| Header | Value | Description |
|--------|-------|-------------|
| `Authorization` | `Bearer lx_sec_live_k_xxx_yyyy` | Existing API-key authentication |
| `Authorization` | `Basic <base64(key_id:secret)>` | Credentialed DSN authentication |
| `X-Loza-Service` | `checkout-api` | Service name from config |
| `X-Loza-Env` | `prod` | Environment from config |

Basic credentials are sent over TLS by default. Never put the password in query parameters, paths, resolved endpoint URLs, logs, or unredacted DSN/debug output.

## Collector Configuration

The Collector validates configured key records using RBAC and ABAC.
`server_secret` is an independent HMAC key; it is not the at-rest storage key.
`secret_env` holds only the token secret, so a configured key is presented as
`lx_{kind}_live_{key_id}_${SECRET_ENV}`.

```yaml
auth:
  enabled: true
  server_secret: "${COLLECTOR_AUTH_SERVER_SECRET}"
  cache_ttl: 5m
  negative_cache_ttl: 30s
  default_collector: payments # legacy root routes only
  collectors:
    - slug: payments
  keys:
    - name: payments-operator
      key_id: kpaymentsoperator
      secret_env: PAYMENTS_OPERATOR_KEY_SECRET
      kind: sec
      collector: payments
      permissions: [events:read, events:write, events:delete]
      allowed_envs: [prod, staging]
    - name: payments-admin
      key_id: kpaymentsadmin
      secret_env: PAYMENTS_ADMIN_KEY_SECRET
      kind: sec
      collector: payments
      permissions: [project:admin]
      allowed_envs: [prod]
    - name: payments-browser
      key_id: kpaymentsbrowser
      secret_env: PAYMENTS_BROWSER_KEY_SECRET
      kind: pub
      collector: payments
      permissions: [events:write]
      allowed_envs: [prod]
      allowed_origins: [https://app.example.com]
storage:
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY
```

All configured key IDs must be unique. `kind` is `sec` or `pub`; public keys
require a non-empty origin allowlist. A scoped API key MUST supply `collector`,
`permissions`, and non-empty `allowed_envs`; the collector must be listed in
`auth.collectors`. Its permissions are evaluated only against the canonical
`/collectors/{collector}/...` resource and default to deny. Unscoped keys are
supported solely for legacy root routes during migration. Auth startup requires
a non-empty resolved server secret, every configured key secret, and the
storage encryption key. An explicit `auth.enabled: false` remains an operator
override, not a tracked default.

## Multi-Collector Access

Each DSN path identifies one Collector resource. SDKs send events to the
canonical scoped resource path:

```text
loza://kingest:s%40cret@collector.example.com/payments?env=prod
                                  └──────── collector name
```

The resolved ingest URL is
`https://collector.example.com/collectors/payments/events`; userinfo is never
included in that URL.

API keys and Collector grants both bind an authenticated credential to a named
collector, explicit environments, and permissions. The Collector defaults to
deny when no binding matches the route collector, `X-Loza-Env`, and required
permission. `project:admin` manages the named collector; `events:read`,
`events:write`, and `events:delete` remain distinct.

```yaml
auth:
  default_collector: payments # temporary legacy /events compatibility only
  collectors:
    - slug: payments
    - slug: analytics
  grants:
    - name: payments-ingest
      collector: payments
      username: payments-writer
      password_env: PAYMENTS_WRITER_PASSWORD
      permissions: [events:write]
      allowed_envs: [prod, staging]
    - name: payments-browser
      collector: payments
      public_id_env: PAYMENTS_BROWSER_ACCESS_ID
      permissions: [events:write]
      allowed_envs: [prod]
      allowed_origins: [https://app.example.com]
```

Private grants require `username:password` and a password sourced from a
secret environment variable. A public DSN uses an opaque, high-entropy
`lx_pub_...` access ID as its username and no password:

```text
loza://lx_pub_<random>@collector.example.com/payments?env=prod
```

Despite appearing in the username position, a public access ID is a revocable
bearer capability. Store it in a secret manager, redact it in logs and DSN
representations, constrain it by collector/environment/origin/IP/rate limit,
and rotate it like any other credential. A human-readable username without a
password is forgeable and is not a valid authorization identity.

Unscoped legacy routes such as `/events` work only when `default_collector` is
configured. New clients and integrations MUST use `/collectors/{collector}/...`
and MUST NOT rely on event payload fields to choose collector or environment.

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

1. Create a new key in the dashboard (or control plane API).
2. Store the new secret in the application secret environment/secret manager; if using a DSN, percent-encode reserved password characters.
3. Deploy your application with the new key or DSN.
4. The collector accepts both keys during the overlap window.
5. Revoke the old key after all deployments are updated.

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

# LOZA Authentication Specification

## Overview

LOZA uses scoped ingest API keys with RBAC+ABAC for authentication and authorization. The collector (data plane) validates keys and enforces permissions. Key management (create/revoke/rotate) lives in the control plane (cortex/API).

## Key Format

```
lz_{kind}_{env}_{key_id}_{secret}
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
lz_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw    # Backend server (production)
lz_pub_live_kabc123_xxxxx                    # Frontend/browser (production)
lz_sec_test_ktest1_testsecret                # Backend server (test)
lz_local_dev_mydevtoken                      # Local development only
```

### Key Kinds

| Kind | Prefix | Use Case | Scopes |
|------|--------|----------|--------|
| `sec` | `lz_sec_` | Backend/server SDKs | Full ingest |
| `pub` | `lz_pub_` | Frontend/browser/mobile | Limited ingest |
| `local` | `lz_local_` | Local dev only | Full ingest (blocked in prod) |

## Wire Protocol

### HTTP

```
POST /events
Authorization: Bearer lz_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
X-Loza-Service: checkout-api
X-Loza-Env: prod
Content-Type: application/json
```

### gRPC

Metadata:
```
authorization: Bearer lz_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw
x-loza-service: checkout-api
x-loza-env: prod
```

## RBAC Roles

| Role | Permissions | Use Case |
|------|-------------|----------|
| `collector_ingest_public` | events:write, heartbeat:write | Frontend/browser SDKs |
| `collector_ingest_server` | events:write, logs:write, traces:write, metrics:write, heartbeat:write | Backend SDKs |
| `collector_ingest_enterprise` | All above + profiles:write, attachments:write | Enterprise (requires mTLS/HMAC) |
| `project_readonly` | events:read, logs:read, traces:read, metrics:read, schema:read, pii_audit:read | Dashboards, support |
| `project_operator` | project_readonly + schema:write | DevOps, infra automation |
| `project_admin` | events:read, events:delete, logs:read, traces:read, metrics:read, schema:read, schema:write, pii_audit:read, project:admin | Admin (no ingest) |

Note: `project_admin` does NOT include ingest permissions. Ingest and admin keys are separate concerns.

## Permissions

```
events:write      logs:write       traces:write     metrics:write
events:read       logs:read        traces:read      metrics:read
events:delete     heartbeat:write  schema:write     schema:read
pii_audit:read    project:admin
```

## ABAC Restrictions

Each API key can have attribute-based restrictions:

| Restriction | Description | Default |
|-------------|-------------|---------|
| `allowed_envs` | Permitted environments | All |
| `allowed_services` | Permitted service names | All |
| `allowed_origins` | Permitted Origins (public keys) | Required for pub |
| `allowed_ips` | Permitted IP addresses/CIDRs | All |
| `max_payload_bytes` | Max request body size | 256KB |
| `max_requests_per_minute` | Request rate limit | 1000 |
| `max_events_per_minute` | Event rate limit | 10000 |
| `sampling_rate` | Event sampling rate | 1.0 |
| `allow_pii` | Allow PII in events | false for pub |
| `allow_attachments` | Allow attachments | false for pub |

## Validation Flow

```
1.  Parse Authorization header → extract key
2.  ParseKey(raw) → kind, env, key_id, secret
3.  Cache lookup by key_id (60s positive, 10s negative)
4.  If cache miss: KeyStore.FindByKeyID()
5.  Check revoked / expired
6.  Verify prefix matches DB record
7.  HMAC-SHA256(incoming secret) == stored hash (constant-time)
8.  Build AuthContext (org, project, roles, permissions)
9.  Public keys: force allow_pii=false, allow_attachments=false
10. Check X-Loza-Env against allowed_envs
11. Check X-Loza-Service against allowed_services
12. Check Origin against allowed_origins (public keys)
13. Check remote IP against allowed_ips
14. Check Content-Length against max_payload_bytes
15. Wrap body with http.MaxBytesReader
16. Apply per-key rate limit (requests/min + events/min)
17. Attach AuthContext to request context
18. Call next handler
```

## Default-Deny Rules

| Condition | Decision |
|-----------|----------|
| Unknown role | Deny |
| Unknown permission | Deny |
| Missing env when allowed_envs configured | Deny |
| Missing service when allowed_services configured | Deny |
| Missing origin for public key | Deny |
| Missing auth on protected route | Deny |
| Missing IP when allowed_ips configured | Deny |
| Key not found | Deny |
| Key revoked | Deny |
| Key expired | Deny |
| Payload exceeds max_payload_bytes | Deny (413) |
| Rate limit exceeded | Deny (429) |

## Public Key Strict Defaults

Public keys (`lz_pub_*`) are assumed exposed:

- Only `events:write` + `heartbeat:write` permissions
- No logs, traces, metrics, attachments, PII
- `allow_pii: false` (cannot be overridden)
- `allow_attachments: false` (cannot be overridden)
- Origin allowlist required
- Lower rate limits
- Smaller payload limits

## Failure Response

```
HTTP/401 Unauthorized
X-Auth-Failure-Reason: <human-readable reason>
X-Auth-Failure-Code: <machine-readable code>
```

| Code | Reason |
|------|--------|
| `missing_token` | No Authorization header |
| `invalid_key_format` | Key doesn't match lz_ format |
| `key_not_found` | Key ID not in store |
| `key_revoked` | Key has been revoked |
| `key_expired` | Key has expired |
| `key_kind_mismatch` | Key kind prefix mismatch |
| `invalid_secret` | HMAC verification failed |
| `env_not_allowed` | X-Loza-Env not in allowed_envs |
| `service_not_allowed` | X-Loza-Service not in allowed_services |
| `origin_not_allowed` | Origin not in allowed_origins |
| `ip_not_allowed` | IP not in allowed_ips |
| `rate_limited` | Per-key rate limit exceeded |

## Token Storage

- Never store raw tokens
- Store HMAC-SHA256 hash: `hmac_sha256(server_secret, token_secret)`
- Constant-time comparison via `crypto/subtle.ConstantTimeCompare`
- Server secret from env: `COLLECTOR_SERVER_SECRET`

## SDK Integration

### Go

```go
client := loza.New(loza.Config{
    Endpoint: "https://collector.loza.dev",
    APIKey:   os.Getenv("LOZA_API_KEY"),
    Service:  "checkout-service",
    Env:      "prod",
})
```

Env vars: `LOZA_API_KEY`, `LOZA_COLLECTOR_URL`

### Headers Set Automatically

```
Authorization: Bearer lz_sec_live_k_xxx_yyyy
X-Loza-Service: checkout-service
X-Loza-Env: prod
```

## Local Development

```go
client := loza.New(loza.Config{
    Endpoint: "http://localhost:9308",
    APIKey:   "lz_local_dev_mydevtoken",
    Service:  "test-service",
    Env:      "dev",
    Insecure: true,
})
```

Collector config:
```yaml
auth:
  enabled: false  # or enabled: true with dev_mode: true
```

## Key Management

Key management (create/revoke/rotate) lives in the control plane (cortex/API), NOT in the collector. The collector only validates keys.

### Key Lifecycle

1. User creates project in dashboard
2. Backend generates ingest token
3. User adds token to env (`LOZA_API_KEY`)
4. SDK sends batches to collector over HTTPS
5. Collector validates token, enriches, redacts, stores

### Key Rotation

1. Create new key in dashboard
2. Deploy app with new key
3. Collector accepts both keys during overlap window
4. Revoke old key

## Route-Level Permissions

Each route has its own permission requirement:

| Route | Permission |
|-------|------------|
| `POST /events` | events:write |
| `GET /events` | events:read |
| `POST /logs` | logs:write |
| `POST /traces` | traces:write |
| `GET /tail` | events:read |
| `POST /schema/publish` | schema:write |
| `DELETE /events` | events:delete |
| `POST /audit/pii` | pii_audit:read |
| `GET /dlq` | events:read |
| `POST /dlq/replay` | events:write |
| `GET /healthz` | (public) |
| `GET /readyz` | (public) |
| `GET /metrics` | events:read |

## Audit Events

| Event | Description |
|-------|-------------|
| `key.authenticated` | Successful auth |
| `key.failed` | Invalid/revoked/expired key |
| `key.rate_limited` | Rate limit exceeded |
| `key.permission_denied` | Missing required permission |
| `key.env_denied` | Environment not allowed |
| `key.origin_denied` | Origin not allowed |
| `key.payload_too_large` | Payload exceeds limit |

## Future: HMAC Signed Requests (post-v0.0.2)

For enterprise high-security mode:

```
Authorization: Loza-HMAC key_id="k_xxx", signature="...", timestamp="..."
```

Signature base: `METHOD\nPATH\nTIMESTAMP\nSHA256(body)`

Protects against replay and tampering.

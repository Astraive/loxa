# Authorization

## Overview

LOXA uses Role-Based Access Control (RBAC) with Attribute-Based Access Control (ABAC) restrictions. Every API key carries a role that determines which permissions it has, plus optional attribute restrictions that further limit what it can do.

## RBAC Roles

### Roles Matrix

| Role | events:write | events:read | events:delete | logs:write | logs:read | traces:write | traces:read | metrics:write | metrics:read | heartbeat:write | schema:write | schema:read | pii_audit:read | project:admin |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `collector_ingest_public` | x | | | | | | | | | x | | | | |
| `collector_ingest_server` | x | | | x | | x | | x | | x | | | | |
| `collector_ingest_enterprise` | x | | | x | | x | | x | | x | | | | |
| `project_readonly` | | x | | | x | | x | | x | | | x | x | |
| `project_operator` | | x | | | x | | x | | x | | x | x | x | |
| `project_admin` | | x | x | | x | | x | | x | | x | x | x | x |

### Role Definitions

#### `collector_ingest_public`

For frontend/browser/mobile SDKs. Minimal permissions — events and heartbeats only.

- **Permissions**: `events:write`, `heartbeat:write`
- **Key kind**: `pub`
- **Forced restrictions**: PII never allowed, attachments never allowed, origin allowlist required

#### `collector_ingest_server`

For backend/server SDKs. Full ingest across all signal types.

- **Permissions**: `events:write`, `logs:write`, `traces:write`, `metrics:write`, `heartbeat:write`
- **Key kind**: `sec`

#### `collector_ingest_enterprise`

For enterprise customers requiring full ingest with additional capabilities.

- **Permissions**: Same as `collector_ingest_server` + `profiles:write`, `attachments:write`
- **Key kind**: `sec`
- **Requires**: mTLS or HMAC authentication

#### `project_readonly`

For dashboards, support tools, and read-only access to stored data.

- **Permissions**: `events:read`, `logs:read`, `traces:read`, `metrics:read`, `schema:read`, `pii_audit:read`
- **Key kind**: N/A (control plane keys)

#### `project_operator`

For DevOps and infrastructure automation. Read access plus schema management.

- **Permissions**: `project_readonly` + `schema:write`
- **Key kind**: N/A (control plane keys)

#### `project_admin`

Full administrative access. **Does NOT include ingest permissions** — ingest and admin are separate concerns.

- **Permissions**: `events:read`, `events:delete`, `logs:read`, `traces:read`, `metrics:read`, `schema:read`, `schema:write`, `pii_audit:read`, `project:admin`
- **Key kind**: N/A (control plane keys)

## Permissions Reference

| Permission | Description | Used By |
|------------|-------------|---------|
| `events:write` | Ingest events | Ingest SDKs |
| `events:read` | Query/read events | CLI, dashboards |
| `events:delete` | Delete events | Admin |
| `logs:write` | Ingest logs | Ingest SDKs |
| `logs:read` | Query/read logs | CLI, dashboards |
| `traces:write` | Ingest traces | Ingest SDKs |
| `traces:read` | Query/read traces | CLI, dashboards |
| `metrics:write` | Ingest metrics | Ingest SDKs |
| `metrics:read` | Query/read metrics | CLI, dashboards |
| `heartbeat:write` | Send heartbeats | All SDKs |
| `schema:write` | Publish/update schemas | DevOps, CI/CD |
| `schema:read` | Read schemas | Dashboards |
| `pii_audit:read` | Audit PII fields | Compliance tools |
| `project:admin` | Project-level admin | Admin panel |

## ABAC Restrictions

Each API key can have attribute-based restrictions that further limit access beyond RBAC roles.

### Restriction Reference

| Restriction | Type | Description | Default |
|-------------|------|-------------|---------|
| `allowed_envs` | `string[]` | Permitted environments (`X-Loxa-Env` header) | All |
| `allowed_services` | `string[]` | Permitted service names (`X-Loxa-Service` header) | All |
| `allowed_origins` | `string[]` | Permitted HTTP Origins | Required for `pub` keys |
| `allowed_ips` | `string[]` | Permitted IP addresses/CIDRs (e.g., `10.0.0.0/8`) | All |
| `max_payload_bytes` | `int` | Maximum request body size | 262144 (256KB) |
| `max_requests_per_minute` | `int` | Request rate limit per key | 1000 |
| `max_events_per_minute` | `int` | Event rate limit per key | 10000 |
| `sampling_rate` | `float` | Event sampling rate (0.0–1.0) | 1.0 |
| `allow_pii` | `bool` | Allow PII in event payloads | `false` for `pub` |
| `allow_attachments` | `bool` | Allow file attachments | `false` for `pub` |

### Configuration Example

```yaml
auth:
  keys:
    - name: "prod-backend"
      key_id: "k2M9aQp"
      secret_env: "PROD_KEY_SECRET"
      kind: "sec"
      roles: ["collector_ingest_server"]
      allowed_envs: ["prod", "staging"]
      allowed_services: ["checkout-api", "payment-api"]
      allowed_ips: ["10.0.0.0/8", "172.16.0.0/12"]
      max_requests_per_minute: 5000
      max_events_per_minute: 50000
      max_payload_bytes: 524288
      allow_pii: true
```

### Public Key Restrictions

Public keys (`lx_pub_*`) have forced restrictions that cannot be overridden:

- `allow_pii: false` — always
- `allow_attachments: false` — always
- `allowed_origins: [...]` — required, must be explicitly configured
- Lower default rate limits
- Smaller default payload limits

## Route-Level Permissions

Each collector endpoint requires a specific permission:

| Route | Method | Permission | Description |
|-------|--------|------------|-------------|
| `/v1/events` | `POST` | `events:write` | Ingest events |
| `/v1/events` | `GET` | `events:read` | Query events |
| `/v1/events` | `DELETE` | `events:delete` | Delete events |
| `/v1/logs` | `POST` | `logs:write` | Ingest logs |
| `/v1/traces` | `POST` | `traces:write` | Ingest traces |
| `/v1/tail` | `GET` | `events:read` | Tail live events |
| `/v1/schema/publish` | `POST` | `schema:write` | Publish schemas |
| `/v1/audit/pii` | `POST` | `pii_audit:read` | Audit PII fields |
| `/v1/dlq` | `GET` | `events:read` | List dead-letter queue |
| `/v1/dlq/replay` | `POST` | `events:write` | Replay from DLQ |
| `/healthz` | `GET` | *(public)* | Health check |
| `/readyz` | `GET` | *(public)* | Readiness check |
| `/metrics` | `GET` | `events:read` | Prometheus metrics |

## Default-Deny Rules

All authorization decisions follow default-deny. If any condition is not explicitly allowed, access is denied.

| Condition | Decision |
|-----------|----------|
| Unknown role | Deny |
| Unknown permission | Deny |
| Missing env when `allowed_envs` configured | Deny |
| Missing service when `allowed_services` configured | Deny |
| Missing origin for public key | Deny |
| Missing auth on protected route | Deny |
| Missing IP when `allowed_ips` configured | Deny |
| Key not found | Deny |
| Key revoked | Deny |
| Key expired | Deny |
| Payload exceeds `max_payload_bytes` | Deny (413) |
| Rate limit exceeded | Deny (429) |

## Validation Flow

The collector validates every request through a 17-step pipeline:

```
 1. Parse Authorization header → extract key
 2. ParseKey(raw) → kind, env, key_id, secret
 3. Local dev keys → auto-authorize as collector_ingest_server
 4. Cache lookup by key_id (60s positive, 10s negative)
 5. Cache miss → KeyStore.FindByKeyID()
 6. Check revoked / expired
 7. Verify kind prefix matches DB record
 8. HMAC-SHA256(incoming secret) == stored hash (constant-time)
 9. Build AuthContext (org, project, roles, permissions)
10. Public keys → force allow_pii=false, allow_attachments=false
11. Check X-Loxa-Env against allowed_envs
12. Check X-Loxa-Service against allowed_services
13. Check Origin against allowed_origins
14. Check remote IP against allowed_ips
15. Check Content-Length against max_payload_bytes
16. Apply per-key rate limit (requests/min + events/min)
17. Attach AuthContext to request context → call next handler
```

## Audit Events

All authorization-sensitive actions emit audit events for compliance and debugging:

| Event | Description |
|-------|-------------|
| `key.authenticated` | Successful authentication |
| `key.failed` | Invalid, revoked, or expired key |
| `key.rate_limited` | Rate limit exceeded |
| `key.permission_denied` | Missing required permission for route |
| `key.env_denied` | Environment not in `allowed_envs` |
| `key.origin_denied` | Origin not in `allowed_origins` |
| `key.ip_denied` | IP not in `allowed_ips` |
| `key.payload_too_large` | Payload exceeds `max_payload_bytes` |

## gRPC Authorization

The same RBAC+ABAC model applies to gRPC endpoints. Keys are passed via gRPC metadata:

```go
metadata := metadata.Pairs(
    "authorization", "Bearer lx_sec_live_k2M9aQp_7QmVxN8pT4zRbK1sYw",
    "x-loxa-service", "checkout-api",
    "x-loxa-env", "prod",
)
ctx := metadata.NewOutgoingContext(context.Background(), metadata)
```

Both unary and stream interceptors enforce the same permission checks as HTTP routes.

## Examples

### Frontend SDK (Public Key)

```yaml
# Collector config
- name: "web-app"
  key_id: "kabc123"
  secret_env: "WEB_KEY_SECRET"
  kind: "pub"
  roles: ["collector_ingest_public"]
  allowed_origins: ["https://app.example.com"]
  max_requests_per_minute: 600
  max_events_per_minute: 1000
```

This key can only:
- Write events and heartbeats
- From `https://app.example.com`
- At 600 requests/min and 1000 events/min
- Without PII or attachments

### Backend SDK (Server Key)

```yaml
# Collector config
- name: "checkout-service"
  key_id: "kdef456"
  secret_env: "CHECKOUT_KEY_SECRET"
  kind: "sec"
  roles: ["collector_ingest_server"]
  allowed_envs: ["prod", "staging"]
  allowed_services: ["checkout-api"]
  allow_pii: true
  max_requests_per_minute: 5000
  max_events_per_minute: 50000
```

This key can:
- Write events, logs, traces, metrics, and heartbeats
- Only in `prod` or `staging` environments
- Only for the `checkout-api` service
- Include PII in payloads

### Read-Only Dashboard Key

```yaml
# Collector config (control plane)
- name: "grafana-dashboard"
  key_id: "kghi789"
  secret_env: "GRAFANA_KEY_SECRET"
  kind: "sec"
  roles: ["project_readonly"]
  allowed_ips: ["10.0.0.0/8"]
```

This key can:
- Read events, logs, traces, metrics, schemas, and PII audits
- Only from the internal network (`10.0.0.0/8`)
- Cannot write or delete anything

## Full Specification

See [spec/auth/spec.md](../spec/auth/spec.md) for the complete authorization specification.

## Next Steps

- [Authentication](authentication.md) — Key types, format, and SDK configuration
- [Security](security.md) — Architecture overview and redaction controls

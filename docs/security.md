# Security

## Architecture

```
loxa-go SDK
  |  HTTPS
  |  Authorization: Bearer lx_sec_live_k_xxx_yyyy
  |  X-Loxa-Service: checkout-api
  |  X-Loxa-Env: prod
  v
loxa-collector (data plane)
  |  validates API key (with cache)
  |  attaches org/project/key context
  |  enforces per-route RBAC permission
  |  enforces ABAC restrictions
  |  rate limits per key
  |  redacts/enriches
  v
loxa-cortex / sinks
```

## Key Types

| Type | Format | Use Case |
|------|--------|----------|
| Secret | `lx_sec_live_k_xxx_yyy` | Backend/server SDKs |
| Public | `lx_pub_live_k_xxx_yyy` | Frontend/browser/mobile |
| Local | `lx_local_dev_yyy` | Local development |

## RBAC Roles

| Role | Use Case | Permissions |
|------|----------|-------------|
| `collector_ingest_public` | Frontend | events:write, heartbeat:write |
| `collector_ingest_server` | Backend | events:write, logs:write, traces:write, metrics:write, heartbeat:write |
| `collector_ingest_enterprise` | Enterprise | All above + profiles:write, attachments:write |
| `project_readonly` | Dashboard | All read permissions |
| `project_operator` | DevOps | Read + schema:write |
| `project_admin` | Admin | Read + delete + project:admin (no ingest) |

## SDK Configuration

```go
// Production
client := loxa.New(loxa.Config{
    Endpoint: "https://collector.loxa.dev",
    APIKey:   os.Getenv("LOXA_API_KEY"),
    Service:  "checkout-service",
    Env:      "prod",
})

// Local dev
client := loxa.New(loxa.Config{
    Endpoint: "http://localhost:9090",
    APIKey:   "lx_local_dev_mydevtoken",
    Service:  "test-service",
    Env:      "dev",
    Insecure: true,
})
```

## Collector Configuration

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
      allowed_services: ["checkout-api"]
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

## Default-Deny

All auth decisions follow default-deny. Unknown roles, missing permissions, missing env/service/origin/IP all result in denial.

## Token Storage

- Never store raw tokens
- HMAC-SHA256: `hmac_sha256(server_secret, token_secret)`
- Constant-time comparison

## Key Rotation

1. Create new key
2. Deploy with new key
3. Both keys accepted during overlap
4. Revoke old key

## Audit Events

All auth-sensitive actions emit audit events: `key.authenticated`, `key.failed`, `key.rate_limited`, `key.permission_denied`, `key.env_denied`, `key.origin_denied`, `key.payload_too_large`.

## Full Spec

See [spec/auth/spec.md](../spec/auth/spec.md) for the complete authentication specification.

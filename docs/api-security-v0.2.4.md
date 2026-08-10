# API Security Audit Report -- Loza v0.2.4

**Date**: 2026-05-28
**Scope**: All API endpoints across collector, cortex, CLI, LQL, SDKs, Lozana, eventbus, spec
**Methodology**: Manual code review of all source files, OWASP API Security Top 10 2023 mapping
**Auditor**: API Security Specialist Agent

---

## Executive Summary

Loza v0.2.4 demonstrates strong security engineering with defense-in-depth across authentication, authorization, input validation, and rate limiting. The codebase shows clear evidence of iterative security hardening from v0.2.0 through v0.2.3. This audit identified **1 CRITICAL**, **3 HIGH**, **6 MEDIUM**, and **5 LOW** findings.

---

## API Security Findings

| # | Severity | Category | Endpoint/Component | Finding | OWASP Ref |
|---|----------|----------|-------------------|---------|-----------|
| 1 | CRITICAL | Injection | POST /schema/blueprint | Blueprint DuckDB type injection via parameterized types | API8:2023 |
| 2 | HIGH | Auth | GET /status | Status endpoint uses legacy auth, bypasses key store permissions | API1:2023 |
| 3 | HIGH | GraphQL | Collector /graphql | No query depth limiting on collector GraphQL endpoint | API4:2023 |
| 4 | HIGH | CORS/WebSocket | WebSocket origins | IPv6 localhost not covered in origin allowlist | API7:2023 |
| 5 | MEDIUM | Headers | All HTTP responses | Missing Strict-Transport-Security header | API8:2023 |
| 6 | MEDIUM | Headers | All HTTP responses | Deprecated X-XSS-Protection header may cause issues | API8:2023 |
| 7 | MEDIUM | Info Leak | Cortex /readyz | Error details leaked in readiness check response | API3:2023 |
| 8 | MEDIUM | DuckDB | Collector DuckDB sink | Connection pool may use different connection for SET safety guard | API8:2023 |
| 9 | MEDIUM | GraphQL | Collector /graphql | No request body size limit on GraphQL endpoint | API4:2023 |
| 10 | MEDIUM | Info Leak | Various handlers | err.Error() returned directly to clients in some handlers | API3:2023 |
| 11 | LOW | Version | GET /version | Exact version exposed publicly | API3:2023 |
| 12 | LOW | Replay | POST /replay | 10MB body limit exceeds standard 1MB | API4:2023 |
| 13 | LOW | Lozana | Frontend build | API key embedded in Vite build output | API2:2023 |
| 14 | LOW | PRAGMA | POST /query, /lql/query | PRAGMA table_info/database_list allowed, leaks schema | API3:2023 |
| 15 | LOW | Audit | Auth middleware | AuditLogger defined but not wired into middleware | API4:2023 |

---

## Detailed Findings

### Finding 1: CRITICAL -- Blueprint DuckDB Type Injection

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/schema_audit_handlers.go`, lines 419-436

**Description**: The `applyBlueprint` function validates the base DuckDB type against an allowlist, but for parameterized types (e.g., `VARCHAR(256)`), only the part before the parenthesis is checked. The full user-provided type string is interpolated directly into SQL.

**Attack vector**: A malicious blueprint column with `duckdb_type: "VARCHAR(256); DROP TABLE events--"` would pass the base type check (`VARCHAR` is allowed) but inject arbitrary SQL.

```go
// Vulnerable code
baseType := strings.ToUpper(typ)
if idx := strings.Index(baseType, "("); idx > 0 {
    baseType = baseType[:idx]  // "VARCHAR" -- passes allowlist
}
// typ is still "VARCHAR(256); DROP TABLE events--"
query := fmt.Sprintf("ALTER TABLE events ADD COLUMN IF NOT EXISTS %s %s", colIdent, typ)
```

**Impact**: An authenticated user with `schema:write` permission could execute arbitrary SQL against the DuckDB database, potentially deleting data or exfiltrating information.

**Remediation**: After the base type allowlist check, validate that `typ` only contains safe characters (alphanumeric, parentheses, commas, spaces, brackets for array types):

```go
var safeTypePattern = regexp.MustCompile(`^[A-Za-z0-9_(),\[\] ]+$`)
if !safeTypePattern.MatchString(typ) {
    return fmt.Errorf("invalid characters in DuckDB type %q for column %q", colDef.DuckDBType, colName)
}
```

---

### Finding 2: HIGH -- Status Endpoint Auth Bypass

**File**: `E:/astraive/loza/loza/collector/server/http/server.go`, line 92
**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/public_handlers.go`, line 12

**Description**: The `/status` endpoint is registered as a public route (no `protector` wrapper), but the handler calls `s.isAuthorized(r)` which uses the legacy single-key auth path. When auth is enabled with the new key store (multiple keys), `s.cfg.apiKey` may be empty, causing `authorizeAPIKey` to return `true` unconditionally.

```go
// server.go line 92 - registered without protector
mux.HandleFunc("GET /status", handlers.HandleStatus)

// auth.go lines 38-41 - legacy auth returns true when apiKey is empty
func (s *collectorState) authorizeAPIKey(r *http.Request) bool {
    if s.cfg.apiKey == "" {
        return true  // bypass!
    }
```

**Impact**: Internal metrics (accepted/rejected/invalid counts, queue depth, inflight counts, rate limits) are exposed to unauthenticated users when using multi-key auth configuration.

**Remediation**: Register `/status` with the protector when auth is enabled, or remove the `isAuthorized` check and rely on the middleware chain.

---

### Finding 3: HIGH -- Collector GraphQL Missing Depth Limit

**File**: `E:/astraive/loza/loza/collector/internal/server/graphql.go`

**Description**: The collector's GraphQL endpoint blocks introspection queries but has no query depth limiting. The cortex GraphQL (`E:/astraive/loza/loza/cortex/internal/api/graphql_server.go`, line 24) properly implements `maxGraphQLDepth = 10`, but the collector has no equivalent.

**Impact**: While the collector GraphQL schema is simple (health, ready, metrics), the lack of depth limiting is a defense-in-depth gap. If more complex queries are added in the future, this becomes a resource exhaustion vector.

**Remediation**: Add depth checking before executing queries:

```go
if queryDepth(req.Query) > 10 {
    writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query_exceeds_max_depth"})
    return
}
```

---

### Finding 4: HIGH -- WebSocket IPv6 Origin Bypass

**File**: `E:/astraive/loza/loza/collector/internal/server/websocket.go`, lines 51-53, 69-71
**File**: `E:/astraive/loza/loza/cortex/internal/api/websocket.go`, lines 32-34

**Description**: The WebSocket origin allowlist checks for `http://localhost`, `http://127.0.0.1`, and their port variants, but does not cover IPv6 localhost (`http://[::1]` or `http://[::1]:PORT`). Modern browsers and tools may use IPv6 localhost.

```go
// Current check misses IPv6
if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") ||
    origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") {
    return true
}
```

**Impact**: A malicious page running on `http://[::1]:PORT` could bypass the WebSocket origin check, potentially connecting to the collector or cortex WebSocket endpoints.

**Remediation**: Add IPv6 localhost checks:

```go
origin == "http://[::1]" || strings.HasPrefix(origin, "http://[::1]:")
```

---

### Finding 5: MEDIUM -- Missing HSTS Header

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/security_headers.go`
**File**: `E:/astraive/loza/loza/cortex/internal/middleware/security.go`

**Description**: Neither the collector nor cortex set the `Strict-Transport-Security` header. For HTTPS deployments, this leaves clients vulnerable to SSL stripping attacks.

**Remediation**: Add HSTS header when TLS is enabled:

```go
w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
```

---

### Finding 6: MEDIUM -- Deprecated X-XSS-Protection Header

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/security_headers.go`, line 12
**File**: `E:/astraive/loza/loza/cortex/internal/middleware/security.go`, line 9

**Description**: The `X-XSS-Protection: 1; mode=block` header is deprecated and can introduce vulnerabilities in older versions of Internet Explorer. Modern best practice is to omit it or set to `0` and rely on CSP.

**Remediation**: Either remove the header or change to `0`:

```go
w.Header().Set("X-XSS-Protection", "0")
```

---

### Finding 7: MEDIUM -- ReadyZ Leaks Error Details

**File**: `E:/astraive/loza/loza/cortex/internal/api/server.go`, lines 228-235

**Description**: The cortex `/readyz` endpoint returns `checks["storage"] = err.Error()` which could leak database connection strings, hostnames, or internal error details.

```go
if _, err := s.incidents.List(ctx, 1); err != nil {
    ready = false
    checks["storage"] = err.Error()  // leaks internal details
}
```

**Remediation**: Return a generic error message:

```go
checks["storage"] = "connection failed"
```

---

### Finding 8: MEDIUM -- DuckDB Connection Pool Safety Guard Gap

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/control_handlers.go`, lines 209-214
**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/lql_handler.go`, lines 79-84

**Description**: The `SET enable_external_access=false` command is connection-scoped in DuckDB. If the `sql.DB` connection pool assigns a different connection for the subsequent query, the safety guard may not apply. This is a known DuckDB gotcha.

**Current mitigation**: The code uses the same `db` variable for both the SET and the query, which typically reuses the same connection within a single goroutine. However, under high concurrency, the pool may behave unexpectedly.

**Remediation**: Use `db.Conn(ctx)` to acquire a dedicated connection for the safety guard + query pair, or execute SET and query in a single transaction.

---

### Finding 9: MEDIUM -- Collector GraphQL No Body Size Limit

**File**: `E:/astraive/loza/loza/collector/internal/server/graphql.go`, line 121

**Description**: The collector GraphQL handler reads the request body without a size limit:

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

The cortex GraphQL properly limits to 1MB (`io.LimitReader(r.Body, 1<<20)`).

**Remediation**: Add body size limiting:

```go
if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
```

---

### Finding 10: MEDIUM -- Error Messages Leak Implementation Details

**Files**: Multiple handlers across collector and cortex

**Description**: Several handlers return `err.Error()` directly to clients, which could leak file paths, database details, or internal structure.

Examples:
- `lql_handler.go:89`: `"message": err.Error()` (query failure)
- `control_handlers.go:170`: `"message": err.Error()` (query request parse)
- `schema_audit_handlers.go:94`: `"message": err.Error()` (schema registry save)

**Remediation**: Log detailed errors server-side and return generic messages to clients:

```go
log.Error().Err(err).Msg("query failed")
writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "query_failed"})
```

---

### Finding 11: LOW -- Public Version Exposure

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/public_handlers.go`, line 21
**File**: `E:/astraive/loza/loza/cortex/internal/api/server.go`, line 254

**Description**: The `/version` endpoint is public and returns exact version strings. This aids attackers in targeting known vulnerabilities for specific versions.

**Impact**: Low -- version disclosure is common practice and the security policy already documents supported versions.

---

### Finding 12: LOW -- Replay Endpoint Large Body Limit

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/control_handlers.go`, line 273

**Description**: The `/replay` endpoint reads up to 10MB (`10<<20`), which is 10x larger than the standard 1MB limit used by other handlers.

**Impact**: Low -- this is intentional for replay functionality but increases the attack surface for memory exhaustion.

---

### Finding 13: LOW -- Lozana API Key in Build Output

**File**: `E:/astraive/loza/lozana/src/lib/api/client.ts`, line 4

**Description**: The Lozana frontend reads `VITE_LOZA_API_KEY` from environment variables. Vite embeds these at build time into the JavaScript bundle, making the API key visible to anyone who inspects the built assets.

```typescript
const API_KEY = import.meta.env.VITE_LOZA_API_KEY || "";
```

**Impact**: Low -- this is a client-side SPA where the API key is inherently exposed. The key should be scoped to read-only permissions.

---

### Finding 14: LOW -- PRAGMA Commands Allowed in Query Endpoints

**File**: `E:/astraive/loza/loza/collector/cmd/loza-collector/control_handlers.go`, line 483

**Description**: The `isReadOnlyQuery` function allows `PRAGMA table_info` and `PRAGMA database_list`, which can leak database schema and structure information.

```go
for _, prefix := range []string{"select", "with", "show", "describe", "pragma table_info", "pragma database_list"} {
```

**Impact**: Low -- this is intentional for the Lozana dashboard but expands the information disclosure surface.

---

### Finding 15: LOW -- Audit Logger Not Wired Into Auth Middleware

**File**: `E:/astraive/loza/loza/collector/internal/auth/audit.go`
**File**: `E:/astraive/loza/loza/collector/internal/auth/middleware.go`

**Description**: The `AuditLogger` interface and `SlogAuditLogger` implementation are defined but not wired into the auth middleware. Auth failures are logged via `logAuthFailure` (simple JSON log), but the structured audit events (`AuditKeyAuthenticated`, `AuditKeyFailed`, etc.) are never emitted.

**Remediation**: Wire the `AuditLogger` into the `Middleware` function and emit audit events for all authentication and authorization decisions.

---

## Endpoint Risk Table

| Endpoint | Method | Auth | Rate Limit | Input Validation | Risk Level |
|----------|--------|------|------------|-----------------|------------|
| /events | POST | Key Store + RBAC | Per-key RPM + EPM | Schema + JSON validation | Low |
| /events/batch | POST | Key Store + RBAC | Per-key RPM + EPM | Schema + JSON validation | Low |
| /ingest | POST | Key Store + RBAC | Per-key RPM + EPM | Schema + JSON validation | Low |
| /otlp/logs | POST | Key Store + RBAC | Per-key RPM | Protobuf/JSON parse | Low |
| /query | POST | Key Store + RBAC | Per-key RPM | isReadOnlyQuery + isSafeQuery + SET guard | Medium |
| /lql/query | POST | Key Store + RBAC | Per-key RPM | isReadOnlyQuery + isSafeQuery + SET guard | Medium |
| /status | GET | Legacy auth | None | N/A | **HIGH** |
| /version | GET | None | None | N/A | Low |
| /health | GET | None | None | N/A | Low |
| /ready | GET | None | None | N/A | Low |
| /graphql | POST | Key Store + RBAC | Per-key RPM | Introspection block | Medium |
| /ws/tail | WS | Key Store + RBAC | Per-key RPM | Origin + ReadLimit | Low |
| /keys | POST | Key Store + RBAC (project:admin) | Per-key RPM | Kind/Env validation | Low |
| /keys/{id}/revoke | POST | Key Store + RBAC (project:admin) | Per-key RPM | KeyID from path | Low |
| /keys/{id}/rotate | POST | Key Store + RBAC (project:admin) | Per-key RPM | KeyID from path | Low |
| /schema/blueprint | POST | Key Store + RBAC (schema:write) | Per-key RPM | **Type injection** | **CRITICAL** |
| /schema/publish | POST | Key Store + RBAC (schema:write) | Per-key RPM | Version/field validation | Low |
| /events/by-tenant/{id} | DELETE | Key Store + RBAC (events:delete) | Per-key RPM | Parameterized query | Low |
| /events/by-user/{id} | DELETE | Key Store + RBAC (events:delete) | Per-key RPM | Parameterized + LIKE escape | Low |
| /dlq/* | Various | Key Store + RBAC | Per-key RPM | Path value validation | Low |
| /replay | POST | Key Store + RBAC (events:write) | Per-key RPM | JSON parse | Low |
| /tail | GET | Key Store + RBAC (events:read) | Per-key RPM | Filter validation | Low |
| /retention/apply | POST | Key Store + RBAC (project:admin) | Per-key RPM | N/A | Low |
| /debug/pprof/* | GET | localhost-only | None | N/A | Low |
| Cortex /events | POST | API Key + role:writer | Per-key + Per-IP | JSON decode | Low |
| Cortex /graphql | POST | API Key + role:reader | Per-key + Per-IP | Depth + introspection | Low |
| Cortex /ws | WS | API Key + role:reader | Per-key + Per-IP | Origin + ReadLimit + action auth | Low |
| Cortex /graph/* | GET | API Key + role:reader | Per-key + Per-IP | Depth cap (100) | Low |
| gRPC services | Various | API Key interceptor | Per-key RPM | Proto validation | Low |

---

## Positive Findings

The following security controls are well-implemented and deserve recognition:

1. **HMAC-SHA256 API Key Verification** (`auth/keys.go`): Constant-time comparison via `subtle.ConstantTimeCompare` prevents timing attacks.

2. **AES-256-GCM Encryption at Rest** (`crypto_helpers.go`): Proper nonce generation using `crypto/rand`, authenticated encryption with `cipher.NewGCM`.

3. **TLS 1.2+ Enforcement** (`http_tls.go`): `MinVersion: tls.VersionTLS12` is set on all TLS configurations.

4. **RBAC with 6 Roles** (`auth/roles.go`): Well-designed role hierarchy with default-deny for unknown roles. Separate ingest and admin key concerns.

5. **Per-Key Rate Limiting** (`auth/ratelimit.go`): Uses `ReserveN` for atomic batch token consumption, preventing partial consumption on rejection.

6. **WebSocket Origin Allowlists** (`server/websocket.go`): Exact host matching prevents domain suffix attacks (fixed in v0.2.3).

7. **WebSocket Read Limits**: 1MB `SetReadLimit` on all WebSocket handlers prevents memory exhaustion from oversized frames.

8. **X-Forwarded-For Trust** (`auth/middleware.go`): Only trusts XFF from configured trusted proxy CIDRs, preventing IP spoofing.

9. **Key Cache Invalidation** (`auth/cache.go`): `Invalidate()` properly removes revoked keys from cache, preventing use of stale cached credentials.

10. **Key Rotation Order** (`admin_handlers.go`): New key is stored BEFORE old key is revoked, preventing zero-key lockout.

11. **GraphQL Introspection Blocking** (both collector and cortex): Blocks `__schema`, `__type`, `__typename` with comment stripping to prevent bypass.

12. **SQL Injection Prevention** (`control_handlers.go`):
    - `isReadOnlyQuery()` blocks non-SELECT statements and multi-statement queries
    - `isSafeQuery()` blocks dangerous DuckDB functions (read_csv, read_json, etc.)
    - `SET enable_external_access=false` blocks filesystem/network access
    - `escapeLIKE()` prevents wildcard injection in LIKE clauses
    - Parameterized queries used for deletion handlers

13. **Body Size Limits**: 10MB default on collector, configurable via `server.max_body_bytes`. Per-key `MaxPayloadBytes` enforced via `http.MaxBytesReader`.

14. **pprof Localhost Binding** (`server/http/pprof.go`): Debug endpoints properly restricted to loopback addresses.

15. **Blueprint Type Allowlist** (`schema_audit_handlers.go`): Column types validated against explicit allowlist of safe DuckDB types.

16. **PII Redaction Pipeline** (`event_governance.go`): Configurable blocklist + secret scanning regex for automatic PII redaction.

17. **Audit Logging Infrastructure** (`auth/audit.go`): Structured audit event types defined for all auth decisions (though not yet wired in).

18. **mTLS Subject Allowlists** (`auth.go`): Configurable CN, DNS, and email allowlists for certificate-based authentication.

19. **Security Headers** (both collector and cortex): X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy on all responses.

20. **Cortex Role-Based Endpoint Protection** (`api/server.go`): Writer role required for ingest, reader role for queries. Role hierarchy enforced.

---

## OWASP API Security Top 10 2023 Mapping

| OWASP Category | Status | Findings |
|----------------|--------|----------|
| API1:2023 - Broken Object Level Authorization | Partial | #2 (status endpoint auth bypass) |
| API2:2023 - Broken Authentication | Good | #13 (Lozana key in build -- inherent to SPA) |
| API3:2023 - Broken Object Property Level Authorization | Good | #7, #10, #11, #14 (info leakage) |
| API4:2023 - Unrestricted Resource Consumption | Partial | #3, #9, #12 (missing limits) |
| API5:2023 - Broken Function Level Authorization | Good | RBAC properly enforced on all admin endpoints |
| API6:2023 - Unrestricted Access to Sensitive Business Flows | Good | Rate limiting, deduplication, schema validation |
| API7:2023 - Server Side Request Forgery | Good | External access disabled, no user-controlled URLs |
| API8:2023 - Security Misconfiguration | Partial | #1, #4, #5, #6, #8 (injection, headers, config) |
| API9:2023 - Improper Inventory Management | Good | All endpoints documented and protected |
| API10:2023 - Unsafe Consumption of APIs | Good | Input validation on all consumed data |

---

## Remediation Priority

### Immediate (P0 -- before v0.2.4 release)

1. **Finding 1 (CRITICAL)**: Sanitize blueprint DuckDB type string to prevent SQL injection via parameterized types.

### High Priority (P1 -- within 1 week)

2. **Finding 2 (HIGH)**: Fix `/status` endpoint auth to use middleware chain instead of legacy `isAuthorized`.
3. **Finding 3 (HIGH)**: Add query depth limiting to collector GraphQL endpoint.
4. **Finding 4 (HIGH)**: Add IPv6 localhost to WebSocket origin allowlists.

### Medium Priority (P2 -- within 30 days)

5. **Findings 5-10**: Add HSTS header, fix XSS-Protection header, sanitize error messages, address DuckDB connection pool safety, add GraphQL body limit.

### Low Priority (P3 -- next release cycle)

6. **Findings 11-15**: Version exposure, replay limits, Lozana key handling, PRAGMA controls, audit logger wiring.

---

## Component Coverage

| Component | Files Reviewed | Endpoints Found | Findings |
|-----------|---------------|-----------------|----------|
| Collector HTTP | 15 files | 25 endpoints | 10 |
| Collector gRPC | 2 files | 5 services | 0 |
| Collector GraphQL | 1 file | 1 endpoint | 2 |
| Collector WebSocket | 1 file | 1 endpoint | 1 |
| Collector Auth | 6 files | Middleware chain | 2 |
| Collector Sinks | 7 files | Internal | 1 |
| Cortex HTTP | 4 files | 12 endpoints | 2 |
| Cortex GraphQL | 1 file | 1 endpoint | 0 |
| Cortex WebSocket | 1 file | 1 endpoint | 1 |
| Cortex Auth | 3 files | Middleware chain | 0 |
| Lozana Frontend | 2 files | API client | 1 |
| SDKs | 2 files | Client libraries | 0 |
| Config/Deploy | 2 files | .env templates | 0 |

---

## Testing Recommendations

1. **Blueprint injection test**: Verify that `duckdb_type: "VARCHAR(256); DROP TABLE events--"` is rejected.
2. **Status endpoint auth test**: Verify `/status` requires authentication when multi-key auth is configured.
3. **GraphQL depth test**: Send deeply nested queries to collector GraphQL and verify rejection.
4. **IPv6 WebSocket test**: Verify WebSocket connection from `http://[::1]:PORT` origin.
5. **DuckDB connection pool test**: Verify SET guard applies correctly under concurrent load.
6. **Error message test**: Verify no internal paths or DB details in error responses.

---

*Report generated by API Security Specialist Agent*
*Loza v0.2.4 -- 2026-05-28*

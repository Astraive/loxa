# Changelog

All notable changes to the LOZA project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/0.2.0/).

## [0.3.1] - 2026-08-24

### Security

- Closed collector and Cortex authorization gaps, hardened encrypted spool
  recovery, and redacted internal errors from public API responses.

### Fixed

- Corrected collector shutdown ordering, spool accounting, scoped live sync,
  LQL parameter binding, and cross-backend query behavior.
- Restored deterministic SDK delivery, batching, generated contracts, and
  release-version consistency across Go, Python, Rust, and JavaScript.
- Restored the Lozana lint gate and corrected streaming, query-cache, trace,
  dashboard, and API-client behavior.

## [0.3.0] - 2026-08-14

### Security

- **Collector-scoped API keys**: Bearer API keys can now bind directly to one
  configured collector with explicit `events:read`, `events:write`,
  `events:delete`, or `project:admin` permissions and required environments.
  Canonical collector routes reject mismatched collector, environment, and
  permission scopes by default.

### Changed

- **Authorization configuration**: Documented scoped API-key configuration and
  canonical `/collectors/{collector}/...` usage. Legacy unscoped API keys
  remain restricted to explicit root-route migration paths.

## [0.2.6] - 2026-05-30

### Security

#### Critical Fixes
- **Auth bypass on empty API key**: Collector now rejects requests when `apiKey` is empty, even if `auth_enabled=true`
- **JWT algorithm confusion**: JWT verification now restricts accepted algorithms to match the configured key type (HMAC vs RSA vs ECDSA)
- **SQL injection in worker ensureSchema**: DuckDB table/column names are now quoted via `quoteSQLIdent()` with identifier validation
- **SQL injection in blueprint handlers**: Blueprint type validation now blocks semicolons, SQL comments, and uses configured `duckDBTable` instead of hardcoded "events"
- **Query injection bypass**: `/lql/query` now uses a dedicated DuckDB connection for `SET enable_external_access=false`; `isSafeQuery` blocks function aliases, extensions, and file/network primitives
- **WebSocket CORS bypass**: Tail WebSocket now validates `Origin` header against allowed origins instead of accepting all
- **SQL injection in deletion handler**: `deleteEventsByUser` LIKE pattern now uses proper escaping
- **SQL injection in frontend LQL**: Numeric values regex-validated before interpolation; `limit` validated as integer; `escapeIdent` escapes double quotes; `escapeSQLString` escapes LIKE wildcards
- **API key in localStorage**: API key moved from persistent `localStorage` to `sessionStorage` with automatic legacy cleanup
- **Weak encryption KDF**: At-rest encryption key derivation moved from raw SHA-256 to HKDF with proper salt
- **Cortex secrets in plaintext**: Production K8s manifests now require secret-backed credentials; SSL mode defaults to "require"
- **Redis without auth**: Redis endpoints now require authentication
- **PostgreSQL empty password**: All config files now require passwords from secrets
- **Path traversal (Python SDK)**: `DiskOfflineBuffer`, `FileSink`, `RotatingFileSink` now reject path traversal attempts
- **SSRF (Python SDK)**: Collector endpoint validation blocks private/internal metadata targets

#### Runtime Safety
- **Goroutine leak in rate limiter**: `KeyRateLimiter` now has `Close()` method called during collector shutdown
- **Memory leak in JWT cache**: Unbounded `jwtKeyCache` replaced with bounded cache
- **FFI null pointer (Rust)**: `CStr::from_ptr` now performs null checks before dereference
- **Parser silent corruption (LQL)**: Out-of-bounds token access returns proper error instead of `NullLit` sentinel
- **Negative limit overflow (LQL)**: `limit -5` now rejected with error instead of wrapping to huge usize
- **Integer overflow (LQL)**: Duration math uses `saturating_mul` to prevent silent wraparound
- **WASM validate error handling**: Validation errors now return `Err()` instead of `Ok(json)`, so JS `catch` blocks see failures

#### Hardened Patterns
- **LIKE pattern escaping**: LQL compilers now escape single quotes in LIKE patterns, preventing SQL injection
- **strip_quotes panic**: Single-char strings like `"` handled safely without panic
- **unreachable!() in match arms**: Replaced with proper error handling to prevent runtime panics
- **Dedup Redis key length**: Event IDs hashed before use as Redis keys to prevent memory exhaustion
- **Spool scanner buffer**: Bounded to prevent 2GB memory allocation from malformed input
- **Config secrets redaction**: `config print` now redacts API keys, encryption keys, and passwords

### Changed

- **K8s manifests**: Auth enabled by default in collector and cortex ConfigMaps
- **Rate limiting**: Enabled by default in cortex deployment
- **NetworkPolicy**: Tightened defaults to restrict ingress/egress
- **Docker healthcheck**: Now uses `wget` (installed in image) for reliable health checks
- **Rust SDK locks**: `RwLock` and `Mutex` now recover from poisoning instead of panicking
- **Python SDK config**: `with_*` methods return copies instead of mutating originals
- **Lozana sidebar**: Shows actual collector health status instead of hardcoded "System Online"
- **Lozana QueryClient**: Created per component mount instead of module-scope singleton
- **Lozana API requests**: All fetch calls now have 30s timeout with AbortController
- **Lozana code dedup**: Consolidated duplicate `cn()`, `getApiKey()`, and `Panel` type definitions

### Fixed

- **Pipeline pending race (Go SDK)**: `DropOldest` mode no longer decrements pending for un-enqueued items
- **HTTP response body leak (Go SDK)**: `HttpBatchSink` now drains response body on success
- **FileSink leak (Python SDK)**: Files properly closed on shutdown
- **DiskOfflineBuffer leak (Python SDK)**: File handles explicitly closed
- **flush() timeout (Python SDK)**: Timeout parameter now actually respected
- **Starlette middleware (Python SDK)**: Uses async HTTP to avoid blocking ASGI worker
- **Thread-unsafe stats (Python SDK)**: `DeliveryStats` now uses threading lock
- **Settings URL validation (Lozana)**: Collector URL validated as http/https before saving
- **ErrorBoundary logging (Lozana)**: `componentDidCatch` now logs errors with component stack

## [0.2.5] - 2026-05-28

### Fixed
- Security fixes for WebSocket origin validation, DuckDB connection pool safety, key cache invalidation
- Version loaded from YAML metadata files instead of hardcoded constants
- Bump all versions to 0.2.5

## [0.2.4] - 2026-05-28

### Fixed
- Fix Cortex Dockerfile base image (trixie-slim -> bookworm-slim)
- Update Cortex SECURITY.md image tag references to 0.2.5
- Update SDK READMEs to v0.2.5 (Go, Python, JavaScript, Rust)
- Add missing v0.2.4 CHANGELOG entries across all components
- Bump all versions to 0.2.4

## [0.2.0] - 2026-05-20

### Collector
- HTTP ingest server with gzip, schema validation, deduplication
- Sink fanout: DuckDB, Kafka, ClickHouse, Postgres, Loki, OTLP, S3, GCS
- Delivery modes: direct, spool, queue
- PII redaction pipeline with 14-key safety net
- Queue worker for distributed delivery
- Load generator tool

### Cortex
- Persistent Context Engine (PCE) with 4 phases complete
- Incident reconstruction with causal chains
- Service graph topology and correlation
- Remediation learning and feedback loop
- HTTP/gRPC/WebSocket/GraphQL APIs

### SDKs (Go, Python, Rust, JavaScript)
- Event lifecycle: StartEvent, Enrich, Finish, Emit
- 14-key safety-net redactor
- Full sampler suite (random, errors, status codes, routes, users, tenants, feature flags)
- 8 schema encoders (Default, Flat, Nested, EC, OTel, OTelLog, Datadog, Custom)
- HTTP batch sink with gzip and retry
- Middleware for major frameworks
- CortexClient for direct cortex access

### CLI
- 20+ commands: init, dev, collector, query, tail, schema, emit, bench, deploy, cortex, incident, graph
- Configuration management with YAML/env/code precedence
- Load generator via `loza bench`

### Spec
- v1 event schema with JSON Schema, OpenAPI, Protobuf definitions
- Conformance runner: 12 groups x 4 SDKs = 48 checks
- Comprehensive verifier: 105 subchecks across 12 categories
- Ingest envelope and collector response contracts
- Schema versioning and compatibility policy

# Changelog

All notable changes to this project are documented in this file.

## [0.2.6] - 2026-05-30

### Security
- Fix empty API key auth bypass (reject when key is empty)
- Fix JWT algorithm confusion (restrict to matching key type)
- Fix SQL injection in worker ensureSchema (quote identifiers, validate types)
- Fix SQL injection in blueprint handlers (block semicolons/comments, use configured table)
- Fix query injection bypass (dedicated DuckDB connection, expanded blocklist)
- Fix WebSocket CORS bypass (validate Origin header)
- Fix SQL injection in deletion handler (escape LIKE patterns)
- Move encryption KDF from SHA-256 to HKDF
- Add Close() to rate limiter, fix goroutine leak
- Bound JWT key cache to prevent memory leak
- Redact secrets in config print output
- Bound spool scanner buffer, hash Redis dedup keys
- Tighten K8s auth/rate-limit/NetworkPolicy defaults
- Fix Docker healthcheck (install wget)

### Changed
- Auth enabled by default in K8s ConfigMap
- Rate limiting enabled by default in cortex ConfigMap
- SSL mode defaults to "require" for PostgreSQL

## [0.2.5] - 2026-05-28

### Fixed
- Security fixes for WebSocket origin validation, DuckDB connection pool safety, key cache invalidation
- Version loaded from YAML metadata instead of hardcoded constants
- Bump version to 0.2.5

## [0.2.4] - 2026-05-28

### Fixed
- Version bumped to 0.2.4

## [0.2.3] - 2026-05-27

### Fixed
- Create shared APP_VERSION constant in Loxana to eliminate duplicate version strings
- Remove dead config files (routes.ts, nav.ts, features.ts) from Loxana
- Remove stale template assets (next.svg, vercel.svg) from Loxana public/
- Remove unnecessary "use client" directives from all Loxana .tsx files (Vite, not Next.js)
- Add root README.md with architecture overview and quickstart
- Add TESTING.md with unit, race, frontend, and security check commands
- Fix stale version strings in SDK READMEs (Go, Python, Cortex)
- Bump all versions to 0.2.3

### Security
- All Phase 1-7 security fixes from v0.2.1 and v0.2.2 remain in effect

## [0.2.2] - 2026-05-27

### Security
- Fix randomToken() CSPRNG failure fallback (now fails request instead of returning "local")
- Fix SQL query leak in LQL error/success responses
- Fix key rotation to actually store the new key in the key store
- Fix LIKE wildcard injection in Loxana LQL compiler (escapeSQLString now escapes % and _)
- Add query(), printf(), format() to isSafeQuery blocklist
- Fix isSafeQuery/isReadOnlyQuery pragma contradiction
- Fix ReadTimeout copy-paste bug in HTTP server
- Fix autoMigrateBlueprintColumns to quote table names
- Fix deletion handler to read request body before executing DELETE
- Add WebSocket origin validation
- Add memoryKeyStore mutex for concurrent access safety
- Fix DLQ/quarantine mutex hold during sleep
- Add dedupe map size cap (100K entries)
- Exclude health/ready/version/metrics endpoints from Cortex auth
- Fix Loxana escapeSQLString to escape LIKE wildcards
- Fix handleKeyCreate to actually store generated keys (with SecretHash)
- Fix handleKeyRotate to set SecretHash on new key (HMAC verification now works)
- Add serverSecret field to collectorState for key lifecycle operations
- Add Cortex WebSocket origin validation (same as collector)
- Add graph depth cap (100) to Cortex WebSocket endpoints
- Add security headers middleware to Collector (X-Content-Type-Options, X-Frame-Options, CSP, etc.)
- Add security headers middleware to Cortex
- Block GraphQL introspection queries in Cortex
- Add maxGraphDepth=100 cap on HTTP graph endpoints
- Add defaultMaxBodyBytes=10MB for Cortex request bodies
- Fix local dev key bypass to require AllowLocalDevKeys config flag
- Fix .env.example to use correct COLLECTOR_API_KEY env var name
- Fix SDK READMEs to show v0.2.2 (was v0.0.1)
- Fix Loxana sidebar and settings version strings to v0.2.2

### Fixed
- Bump all versions to 0.2.2
- Fix 11 files still at 0.2.0 (Helm charts, CLI, quickstarts, SDK manifests)
- Fix Python SDK README testkit section (wrong casing, wrong arg style)
- Add React error boundary to Loxana
- Add 404 catch-all route to Loxana
- Remove dead files from Vite migration
- Fix Loxana Settings page stale version
- Fix nav config references to non-existent routes
- Add collector .env.example
- Expand cortex .env.example

## [0.2.1] - 2026-05-27

### Security

- **CRITICAL**: Remove raw SQL passthrough from `/query` and `/lql/query` endpoints. Added blocklist for dangerous DuckDB functions (read_csv, read_json, read_blob, etc.) and `SET enable_external_access=false`.
- **CRITICAL**: Remove hardcoded `"loxa-default-secret"` fallback for HMAC/AES. Collector now fails to start when `auth.enabled=true` and `storageEncryptionKey` is not configured.
- **HIGH**: Fix LIKE wildcard injection in `deleteEventsByUser`. Added `escapeLIKE` function to escape `%`, `_`, and `\` characters.
- **HIGH**: Fix DDL injection in blueprint handler. Added allowlist of valid DuckDB column types.
- **HIGH**: Fix key revocation persistence and rotation old-key invalidation in admin handlers.
- **HIGH**: Fix Loxana LQL compiler SQL injection. Added `escapeSQLString` for string values in `wasm.ts`.
- **HIGH**: Improve mTLS certificate validation. Now validates certificate CN is non-empty.
- **HIGH**: Add rate limiter cleanup goroutine to prevent unbounded memory growth.
- **HIGH**: Pre-compile PII redaction wildcard regex patterns (cache instead of recompile per call).
- **MEDIUM**: Add startup warning when auth is disabled on non-loopback addresses.
- **MEDIUM**: Bind pprof endpoints to localhost by default for security.
- **MEDIUM**: Add security comments to default config files.

### Changed

- Bump version to 0.2.1 across all components.
- Collector, Cortex, Loxana, and all SDKs now report version 0.2.1.

### Fixed

- All 4 quickstart examples corrected to use actual SDK APIs.
- Python SDK README updated to document correct Params-based API.
- Docker build fixed to handle local replace directives.
- CLI defaults.yaml updated for monorepo layout.
- Loxana README replaced with accurate Vite+React documentation.
- Cortex README build command corrected.
- Collector defaults routes corrected (`/ingest` → `/events`, `/healthz` → `/health`).
- Duplicate route panic in `BuildMux` when `ingestPath` equals `/events`.
- All auth-enabled tests updated with required `storageEncryptionKey`.
- Added env var overrides for fanout sink secrets and NATS credentials.
- Sanitized Cortex HTTP error responses (no longer leak `err.Error()`).
- Fixed FFI `shape_hash_ffi` type mismatch in cortex-match crate.

### Infrastructure

- Added `USER` directive to all Go Dockerfiles for non-root execution.
- Added env var overrides for fanout sink secrets (ClickHouse, S3, NATS, Postgres).
- Replaced hardcoded Docker Compose credentials with env var references.
- Added `permissions: contents: read` to CI workflows missing permissions blocks.

## [0.0.2] - 2026-05-20

### Horizon 1 — Foundation Gate
- Updated README "Known Limitations" to reflect that spool, retry, DLQ, and dedupe are available in the current release.
- Moved remaining hardcoded collector values to config fields:
  - `duckdb.column_types` for schema column type mapping.
  - `reliability.spool_file` for WAL file name.
  - `reliability.delivery_queue_size` for spool delivery queue.
- Reference config `build/loxa-collector.yaml` updated with new fields.
- Strict gate: `go test ./... -race` green across all 9 modules; `go vet` clean.

### Added

- Collector owns production delivery:
  - Kafka, ClickHouse, Postgres, DuckDB, OTLP, S3, GCS, and Loki sink implementations are collector-side code.
  - application SDKs emit to the collector through lightweight transports.
- Collector-local event and sink contracts under `internal/event`.
- Dependency boundary removed: collector no longer imports or requires any LOXA SDK module.
- Collector fanout and worker paths cover heavy production sinks.

### Changed

- Collector module dependency graph no longer includes any LOXA SDK module.
- Removed dependency on the old SDK sinks module.
- Integration DuckDB stress test uses temp DB path to avoid cross-run file lock collisions.
- README/docs repositioned LOXA as canonical wide-event layer.

### Breaking

- Collector sink implementations now use `github.com/astraive/loxa/collector/internal/event` instead of SDK-owned interfaces.

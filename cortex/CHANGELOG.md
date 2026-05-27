# Changelog

All notable changes to this project are documented in this file.

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

- Added gRPC graceful shutdown (call `GracefulStop()` on SIGINT/SIGTERM)
- Added startup warning when authentication is disabled
- Sanitized HTTP error responses (no longer leak `err.Error()` to clients)

### Fixed

- Version bumped to 0.2.1

## [0.0.2] - 2026-05-20

### Added

- Persistent Context Engine (PCE) with 4 phases: ingestion, reconstruction, correlation, suggestion
- Incident reconstruction with causal chain analysis (fast and deep modes)
- Service graph topology via collector sync with edge weight tracking
- Similar incident matching with signature morphing and behavioral hashing
- Remediation learning and feedback loop with configurable learning rate
- HTTP, gRPC, WebSocket, and GraphQL API servers
- Shared DuckDB storage with collector for events, topology, graph, incidents, signatures, remediations, and feedback
- Rust FFI crate for pattern matching (cortex-match) with Go fallback
- Correlation analyzer for co-occurrence and deployment adjacency detection
- PII redaction on ingest with configurable mode (off, warn, enforce)
- Authentication middleware with API key support
- Rate limiting middleware (per-API-key and per-IP)
- Prometheus metrics endpoint
- Docker and Kubernetes deployment manifests
- Async event processing with micro-batch support

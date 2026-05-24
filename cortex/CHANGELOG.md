# Changelog

All notable changes to this project are documented in this file.

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

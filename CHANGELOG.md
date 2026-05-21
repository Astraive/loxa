# Changelog

All notable changes to the LOXA project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.0.0] - 2026-05-20

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
- Load generator via `loxa bench`

### Spec
- v1 event schema with JSON Schema, OpenAPI, Protobuf definitions
- Conformance runner: 12 groups x 4 SDKs = 48 checks
- Comprehensive verifier: 105 subchecks across 12 categories
- Ingest envelope and collector response contracts
- Schema versioning and compatibility policy

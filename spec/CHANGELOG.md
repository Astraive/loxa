# Changelog for LOXA Spec

All notable changes to the contract repository are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/0.0.2/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2] - 2026-05-20

### Added

- v1 Schema Structure with versioned schema directory
- Core Event Schema (v0.0.1) with required fields
- Optional fields: tenant, trace, user, host, data
- Enriched fields: received_at, collector_version, collector_hostname, ingestion_latency_ms
- Example events
- PII Classification with x-pii extension
- Schema Versioning policy
- Protobuf definitions for collector API
- OpenAPI specification
- JSON Schema definitions
- Conformance fixtures and runner
- Initial protobuf definitions for loxa-collector API
- OpenAPI specification
- JSON Schema definitions

### Changed

- Reorganized schema files into versioned directories (v1/)
- Standardized go_package option across all proto files

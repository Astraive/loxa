# LOXA Event Schema v1

This directory contains the v1 schema definitions for LOXA events.

## Schema Files

- `event.schema.json` - Core event schema with required and optional fields

## Required Fields

All LOXA events MUST include the following fields:

- `event_id` (string, UUID): Unique event identifier
- `event_type` (string): Event type in dot notation (e.g., `user.login`, `api.request`)
- `timestamp` (string, ISO 8601): Event timestamp
- `service` (object): Service identity with `name` and `version`
- `deployment` (object): Deployment context with `environment`

## Optional Fields

- `tenant` (object): Multi-tenant identifier
- `trace` (object): W3C distributed tracing context
- `user` (object): User context information
- `host` (object): Host or container information
- `data` (object): Event-specific payload

## Enriched Fields

The collector adds the following fields during processing:

- `received_at` (string, ISO 8601): Collector receipt timestamp
- `collector_version` (string): Collector version
- `collector_hostname` (string): Collector hostname
- `ingestion_latency_ms` (integer): Latency in milliseconds

## Examples

See the `examples/` directory for complete event examples:

- `user_login.json` - User authentication event
- `api_request.json` - API request event

## PII Classification

Fields marked with `x-pii` annotations indicate personally identifiable information:

- `internal`: Internal use only
- `confidential`: Requires redaction in certain contexts
- `restricted`: Highly sensitive, requires strict access controls

## Validation

Events can be validated against this schema using standard JSON Schema validators or the LOXA CLI:

```bash
loxa schema validate --file event.json --schema-version v1
```

## Version

- Schema Version: v0.0.1
- Status: Active
- Last Updated: 2024-01-15

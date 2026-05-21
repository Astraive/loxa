# Glossary

| Term | Definition | Component |
|------|-----------|-----------|
| Canonical Event | A normalized event structure where all SDKs produce the same field names and types regardless of language, enforced by the loxa spec schema. | Spec, SDKs |
| Canonical Wins | A redaction policy where canonical fields (service, event_name, duration_ms, etc.) are silently dropped from the attributes map at emit time to prevent duplication. | SDKs |
| Collector-First | Architecture pattern where SDKs emit events to the collector over HTTP rather than writing directly to storage sinks, enabling centralized PII policy, fanout, and delivery guarantees. | Collector, SDKs |
| Conformance | A test suite in `spec/conformance/` that validates all 4 SDKs against 12 groups of behavioral checks (48 total), ensuring cross-language parity. | Spec |
| Direct Mode | A delivery mode where the collector writes events to sinks synchronously during the ingest request path, offering lowest latency but no buffering. | Collector |
| DLQ (Dead Letter Queue) | A persistent queue for events that failed all retry attempts across all sinks, queryable via `loxa dlq list` and replayable via `loxa dlq replay`. | Collector |
| Fanout | The collector mechanism that routes each accepted event to all configured sinks simultaneously (DuckDB, Kafka, ClickHouse, Postgres, Loki, OTLP, S3, GCS). | Collector |
| Ingest Envelope | The wire format for POST /ingest requests, defined in `spec/contracts/ingest-envelope.schema.json`, wrapping one or more events with metadata. | Spec, Collector |
| PCE (Persistent Context Engine) | The cortex subsystem that maintains long-lived context across events, enabling incident reconstruction, causal chains, service graphs, and remediation learning. | Cortex |
| Parity | The state of all 4 SDKs implementing the same feature set, verified by conformance tests. v1.0.0 achieved full P0-P3 parity. | SDKs, Spec |
| Queue Mode | A delivery mode where the collector buffers events in an on-disk queue and a background worker drains them to sinks, providing durability and retry semantics. | Collector |
| Safety-Net Redactor | A redactor that scans all event attributes for 14 predefined PII key patterns (e.g., password, ssn, credit_card, api_key) and redacts values regardless of SDK-level redaction rules. | SDKs, Collector |
| Schema Version | A numeric version on the event schema that enables forward/backward compatibility. The collector validates events against the registered schema version. | Spec, Collector |
| Spool Mode | A delivery mode where the collector writes events to a local spool directory first, then a background process reads and forwards to sinks, offering crash-safe buffering. | Collector |
| Tail Sampling | A sampling strategy that makes the decision to keep or discard an event after the event completes (based on duration, status code, error presence), as opposed to head sampling which decides at start time. | SDKs |
| Wide Event | A telemetry event carrying a rich, structured attribute map (dozens to hundreds of key-value pairs) rather than a narrow metric or log line, enabling flexible querying and correlation. | Spec, SDKs |

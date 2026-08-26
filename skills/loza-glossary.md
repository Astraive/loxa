# LOZA terminology

| Term | Meaning |
|---|---|
| **Canonical event** | A normalized wide-event envelope with stable field names and types across SDKs. |
| **Collector-first** | SDKs send events to the Collector; applications do not write directly to storage sinks. |
| **Wide event** | One structured event containing operation identity, outcome, timing, correlation, and typed attributes. |
| **Scope** | The collector, environment, service, origin, or permission boundary applied to a credential/request. |
| **Direct mode** | Synchronous sink delivery in the ingest request path; low latency, little buffering. |
| **Queue mode** | On-disk queue plus a background worker that drains events to sinks with retry behavior. |
| **Spool mode** | Local spool-first delivery that provides crash-safe buffering before forwarding to sinks. |
| **Fanout** | Delivery of one accepted event to multiple configured sinks. |
| **DLQ** | Dead-letter queue for events that exhausted delivery attempts; inspect and replay only after fixing the cause. |
| **Quarantine** | Durable holding area for events rejected by schema/governance policy. |
| **Deduplication** | Suppression of repeated event IDs within the configured deduplication window/policy. |
| **PCE / Cortex** | The persistent context/control-plane subsystem for incident context, causal reconstruction, graphs, similarity, and remediation feedback. |
| **LQL** | LOZA Query Language compiled for supported query engines and executed through authorized Collector access. |
| **Canonical wins** | Reserved canonical fields take precedence over duplicate/custom attribute keys at emission. |
| **Conformance** | Cross-SDK behavioral checks that verify the same event/lifecycle contract across languages. |
| **Tail sampling** | Deciding whether to retain an event after completion using outcome, duration, or error context. |

Terms, routes, metrics, and CLI verbs can differ by release. Verify ambiguous terms with the installed artifact's README, schema, or `--help` output.

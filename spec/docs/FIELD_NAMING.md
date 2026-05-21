# Field Naming

Canonical field names use lowercase snake case JSON keys such as `event_id`, `request_id`, and `duration_ms`.

Nested domains should remain stable and language-neutral. SDKs may provide ergonomic builders, but emitted JSON should preserve the canonical field names defined by this repository.

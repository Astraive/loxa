# Privacy And Compliance

SDK redaction is best effort. Collector redaction is mandatory in production.
Never rely only on SDK-side redaction.

Required controls:

- PII classification metadata.
- Field allowlist and blocklist policies.
- Collector-side emergency redaction.
- Right-to-delete support for `user.id` and `tenant.id` where storage supports deletion.
- Encryption at rest documentation per sink.
- Secret scanning in events.
- Redaction audit tests.

Production collectors MUST run a redaction processor before durable storage and
fanout sinks.


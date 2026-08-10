# Rust Business Instrumentation

The Rust SDK follows the v0.0.2 lifecycle:

```text
start_event -> append/enrich -> checkpoint -> process/group/timer -> finish/finish_error -> emit
```

Use module-level `loza` functions for the default client, `create_loza(config)` for independent clients, and `alias("name")` for same-config aliases that emit `loza.alias`.

See the root guide: [docs/business-instrumentation.md](../../../docs/business-instrumentation.md).

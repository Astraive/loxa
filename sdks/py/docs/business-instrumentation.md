# Python Business Instrumentation

The Python SDK follows the v0.0.2 lifecycle:

```text
start_event -> append/enrich -> checkpoint -> process/group/timer -> finish/finish_error -> emit
```

Use `loza` for the default client, `create_loza(...)` for independent clients, and `loza.alias("name")` for same-config aliases that emit `loza.alias`.

See the root guide: [docs/business-instrumentation.md](../../../docs/business-instrumentation.md).

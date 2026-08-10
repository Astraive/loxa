# JS Business Instrumentation

The JS SDK follows the v0.0.2 lifecycle:

```text
startEvent -> append/enrich -> checkpoint -> process/group/timer -> finish/finishError -> emit
```

Use `loza` for the default client, `createLoza(config)` for independent clients, and `loza.alias("name")` for same-config aliases that emit `loza.alias`.

See the root guide: [docs/business-instrumentation.md](../../../docs/business-instrumentation.md).

# JS Business Instrumentation

The JS SDK follows the v0.0.2 lifecycle:

```text
startEvent -> append/enrich -> checkpoint -> process/group/timer -> finish/finishError -> emit
```

Use `loxa` for the default client, `createLoxa(config)` for independent clients, and `loxa.alias("name")` for same-config aliases that emit `loxa.alias`.

See the root guide: [docs/business-instrumentation.md](../../../docs/business-instrumentation.md).

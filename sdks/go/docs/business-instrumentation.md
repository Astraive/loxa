# Go Business Instrumentation

The Go SDK follows the v0.0.2 lifecycle:

```text
StartEvent -> Append/Enrich -> Checkpoint -> Process/Group/Timer -> Finish/FinishError -> Emit
```

Use package-level `loza` functions for the default client, `CreateLoza(config)` for independent clients, `New(config)` as the idiomatic Go alias, and `Alias("name")` for same-config aliases that emit `loza.alias`.

See the root guide: [docs/business-instrumentation.md](../../../docs/business-instrumentation.md).

# Loxa Implementation Todo

> **Rule**: Only export `loxa`. No bare `logger` export.  
> `loxa.<method>()` — default global client  
> `createLoxa(config)` — custom client factory  
> `loxa.alias(name)` — same-config child alias  

---

## Phase 1 — Framework middleware (RED across all SDKs)

None of these exist in any SDK yet. Only Go has net/http, gRPC, Gin, Fiber, Echo, Chi middleware.

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 1 | `expressMiddleware(config)` | MISSING | - | - | - |
| 2 | `fastifyPlugin(config)` | MISSING | - | - | - |
| 3 | `nextMiddleware(config)` | MISSING | - | - | - |
| 4 | `honoMiddleware(config)` | MISSING | - | - | - |
| 5 | `koaMiddleware(config)` | MISSING | - | - | - |
| 6 | `djangoMiddleware(config)` | - | EXISTS | - | - |
| 7 | `flaskMiddleware(config)` | - | EXISTS | - | - |
| 8 | `goHTTPMiddleware(config)` | - | - | EXISTS | - |
| 9 | `axumLayer(config)` | - | - | - | MISSING |
| 10 | `towerLayer(config)` | - | - | - | MISSING |

**Target**: Each SDK gets at least its ecosystem's primary framework(s).

---

## Phase 2 — HTTP helpers (RED across all SDKs)

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 11 | `httpRoute(route)` | EXISTS via `route()` | MISSING | EXISTS | MISSING |
| 12 | `httpMethod(method)` | EXISTS via `method()` | MISSING | EXISTS | MISSING |
| 13 | `httpPath(path)` | EXISTS via `path()` | MISSING | EXISTS | MISSING |
| 14 | `httpUserAgent(ua)` | MISSING | MISSING | MISSING | MISSING |
| 15 | `httpReferer(ref)` | MISSING | MISSING | MISSING | MISSING |

---

## Phase 3 — Missing typed attribute helpers

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 16 | `list(key, values)` | EXISTS (`tags`) | MISSING | MISSING | MISSING |
| 17 | `map(key, value)` | MISSING | MISSING | MISSING | MISSING |
| 18 | `enum(key, value, allowed?)` | MISSING | MISSING | MISSING | MISSING |
| 19 | `hash(key, value)` | MISSING | MISSING | MISSING | MISSING |
| 20 | `redacted(key)` | MISSING | MISSING | MISSING | MISSING |
| 21 | `orgId(id)` | MISSING | MISSING | MISSING | MISSING |
| 22 | `accountId(id)` | MISSING | MISSING | MISSING | MISSING |
| 23 | `deploymentId(id)` | MISSING (no-op) | MISSING | EXISTS | MISSING |
| 24 | `region(region)` | MISSING | EXISTS | EXISTS | EXISTS |

---

## Phase 4 — Missing sinks

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 25 | `httpSink(config)` | MISSING (only batch) | MISSING | MISSING | MISSING |
| 26 | `kafkaSink(config)` | MISSING | STUB | MISSING | MISSING |
| 27 | `otlpSink(config)` | STUB | STUB | STUB | STUB |

---

## Phase 5 — Sampling & policy methods (RED)

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 28 | `alwaysSample()` | EXISTS (`sampleAll`) | EXISTS (`sample_all`) | EXISTS (`SampleAll`) | EXISTS (`SampleAll`) |
| 29 | `neverSample()` | EXISTS (`sampleNone`) | EXISTS (`sample_none`) | EXISTS (`SampleNone`) | EXISTS (`SampleNone`) |
| 30 | `sampleErrors(rate)` | EXISTS | MISSING as standalone | EXISTS | EXISTS |
| 31 | `maxAttrLength(length)` | MISSING | MISSING | MISSING (has `MaxFieldBytes`) | MISSING |
| 32 | `maxEventBytes(bytes)` | MISSING | MISSING | EXISTS | MISSING (config has `max_event_bytes`) |
| 33 | `maxAttrs(count)` | MISSING | MISSING | EXISTS | MISSING |
| 34 | `cardinalityPolicy(policy)` | MISSING | MISSING | MISSING | MISSING |
| 35 | `validateEvent(event)` | MISSING | MISSING | MISSING | MISSING |
| 36 | `normalizeEvent(event)` | MISSING | MISSING | MISSING | MISSING |

---

## Phase 6 — Testing & conformance (RED)

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 37 | `testkit()` | EXISTS (`testLogger`) | EXISTS (`TestLogger`) | EXISTS (`TestLogger`) | EXISTS |
| 38 | `capture(fn)` | EXISTS | EXISTS | EXISTS | EXISTS |
| 39 | `lastEvent()` | EXISTS (via MemorySink) | EXISTS | EXISTS | MISSING |
| 40 | `events()` | EXISTS | EXISTS | EXISTS | MISSING |
| 41 | `clearEvents()` | EXISTS (MemorySink.clear) | EXISTS | EXISTS | MISSING |
| 42 | `goldenTest(path)` | MISSING | MISSING | MISSING | MISSING |
| 43 | `conformanceSuite()` | MISSING | MISSING | MISSING | MISSING |
| 44 | `setClock(clock)` | EXISTS (`FakeClock`) | EXISTS | EXISTS | STUB |
| 45 | `resetForTest()` | MISSING | MISSING | MISSING | MISSING |

---

## Phase 7 — Client config gaps

| # | Method | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 46 | `withEndpoint(url)` | EXISTS (`withCollectorUrl`) | EXISTS | EXISTS | EXISTS |
| 47 | `withRedaction(policy)` | EXISTS (`withRedactor`) | EXISTS | EXISTS | EXISTS |
| 48 | `withOtelBridge(config)` | NO-OP stub | EXISTS | MISSING | NO-OP stub |
| 49 | `fromEnv()` | EXISTS | EXISTS | EXISTS | STUB (returns base config) |
| 50 | `fromRequest(req)` | EXISTS (via HTTP event helpers) | MISSING | EXISTS | MISSING |

---

## Phase 8 — Rust SDK collector client stubs

The `CollectorHttpClient` in `sdks/rs/src/core/client.rs` has hardcoded stub responses for:

| # | Method | Status |
|---|---|---|
| 51 | `ingest(events)` | Delegates to `validate` (no real send) |
| 52 | `query(query)` | Returns hardcoded empty |
| 53 | `tail(count)` | Returns hardcoded empty |
| 54 | `delete(query)` | Returns `{"deleted": 0}` |
| 55 | `replay(event_ids)` | Returns count, no HTTP call |
| 56 | `dlq_list(limit)` | Returns hardcoded empty |
| 57 | `dlq_read(entry_id)` | Returns hardcoded null |
| 58 | `dlq_replay(entry_ids)` | Returns count, no HTTP call |
| 59 | `keys_create(name)` | Returns hardcoded stub |
| 60 | `keys_revoke(key_id)` | Returns hardcoded stub |
| 61 | `sinks_list()` | Returns hardcoded empty |
| 62 | `health()` | Returns hardcoded ok |

**Fix**: Wire these to real HTTP calls using the internal HTTP client.

---

## Phase 9 — Rust SDK testing stub fixes

| # | Method | File | Status |
|---|---|---|---|
| 63 | `CurrentEvent()` | `lib.rs` | Always returns `None` |
| 64 | `Pause()` / `Resume()` | `lib.rs` + `sinks/sinks.rs` | No-ops |
| 65 | `QueueSize()` | `lib.rs` + `sinks/sinks.rs` | Always 0 |
| 66 | `Health()` | `lib.rs` + `sinks/sinks.rs` | Always `true` |
| 67 | `ExpectEvent(...)` | `testkit/helpers.rs` | No-op |
| 68 | `FakeClock()` | `testkit/helpers.rs` | No-op |
| 69 | `SetIDGenerator(...)` | `testkit/helpers.rs` | No-op |
| 70 | `Drain()` | `lib.rs` + `sinks/sinks.rs` | Calls flush, discards error |

---

## Phase 10 — Rust SDK missing lifecycle methods

| # | Method | Status |
|---|---|---|
| 71 | `RunEvent(params, fn)` | MISSING (only in JS, Go) |
| 72 | `Run(ctx, fn)` | MISSING (only in Go) |
| 73 | `RunHTTP/Job/Queue/CLI/Cron` | Partially exists in `lib.rs` as free fns |
| 74 | `Process(ctx, name)` | MISSING (as free fn, only as Logger method) |
| 75 | `Group(ctx, name)` | MISSING (as free fn) |

---

## Phase 11 — Collector server keys management endpoints

| # | Method | Status |
|---|---|---|
| 76 | `POST /core/keys` | MISSING from HTTP server routes |
| 77 | `DELETE /core/keys/{id}` | MISSING from HTTP server routes |
| 78 | `POST /core/keys/{id}/rotate` | MISSING from HTTP server routes |
| 79 | `collector.retention.*` CLI command | MISSING (only internal background worker) |

The CLI (`loxa keys create/revoke/rotate`) calls client functions, but the collector server has no runtime key management HTTP endpoints. Keys are configuration-only.

---

## Phase 12 — Domain helper packs

| # | Pack | JS | Py | Go | Rs |
|---|---|---|---|---|---|
| 80 | Checkout helpers | ALL | ALL | ALL | ALL |
| 81 | Payment helpers | ALL | ALL | ALL | ALL |
| 82 | Billing helpers | ALL | ALL | ALL | ALL |
| 83 | AI agent helpers | ALL | ALL | ALL | ALL |
| 84 | RAG helpers | ALL | ALL | ALL | ALL |

All domain helpers are implemented. Verify when adding new ones.

---

## Phase 13 — SDK parity alignment

| # | Item | Status |
|---|---|---|
| 85 | Collector payload normalization across all 4 SDKs | Need golden fixtures |
| 86 | Same redaction defaults across all SDKs | JS has 22 keys, Rust has ~15, Py has 22 |
| 87 | Same `alias()` semantics (immutable child) | ✅ All 4 SDKs |
| 88 | Same `createLoxa` naming | ✅ All 4 SDKs |
| 89 | Sampler parity (should have same 19 samplers) | JS=19, Go=19, Py=20, Rs=18 |
| 90 | Schema parity (Default/Flat/Nested/OTel/EC/Datadog) | ✅ All 4 SDKs |

---

## Phase 14 — Dead/unused code cleanup

| # | File | Issue |
|---|---|---|
| 91 | `sdks/py/src/loxa/core/client.py` | Duplicate Logger class (477 lines), unused |
| 92 | `sdks/py/src/loxa/core/options.py` | Duplicate With* functions, unused |
| 93 | `sdks/py/src/loxa/config/config.py` | Duplicate of `core/config.py`, less complete |
| 94 | `sdks/js/src/sinks/standard-sinks.ts:RotatingFileSink` | Extends FileSink with no rotation logic |
| 95 | `sdks/rs/src/event.rs:apply_sensitive_flags` | Marked `#[allow(dead_code)]` |
| 96 | `sdks/rs/src/redaction/noop.rs:noop_redact` | Marked `#[allow(dead_code)]` |

---

## Phase 15 — Public API surface audit

Ensure only these are exported from each SDK:

```
# Default global client
loxa.<method>()

# Factory
createLoxa(config) / create_loxa(config) / CreateLoxa(config)

# Aliases (same config, different name)
loxa.alias(name)

# No bare 'logger' or 'Logger' as public export
# No 'new Logger()' as promoted API (internal use only)
```

Current state:
- ✅ JS: Only `loxa`, `createLoxa`, `Loxa` class
- ✅ Python: Only module-level `loxa.*` functions, `create_loxa`, `Logger` class
- ✅ Go: Only package-level `loxa.*` functions, `CreateLoxa`/`New`
- ✅ Rust: Only crate-level `loxa::*` functions, `create_loxa`, `Loxa::new`

---

## Summary counts

| Phase | Items | Priority |
|---|---|---|
| P1: Framework middleware | 10 | High |
| P2: HTTP helpers | 5 | Medium |
| P3: Typed attr helpers | 9 | Low |
| P4: Missing sinks | 3 | Medium |
| P5: Sampling/policy | 9 | Medium |
| P6: Testing/conformance | 9 | Medium |
| P7: Client config gaps | 5 | High |
| P8: Rust collector client stubs | 12 | High |
| P9: Rust testing stubs | 8 | Medium |
| P10: Rust missing lifecycle | 5 | Medium |
| P11: Collector keys endpoints | 4 | High |
| P12: Domain helper packs | 5 | Low (done) |
| P13: SDK parity alignment | 6 | High |
| P14: Dead code cleanup | 6 | Low |
| P15: Public API audit | 4 | Medium |
| **Total** | **96 items** | |

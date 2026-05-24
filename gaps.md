# LOXA Spec Implementation Gap Tracker

> **Architecture Note**: Core implementation lives in `collector/` and `cortex/`.  
> SDKs (`sdks/go`, `sdks/js`, `sdks/py`, `sdks/rs`) are lightweight connectors that serialize and ship events to the collector.  
> Heavy logic (sinks, query, policy, retention, schema management) belongs on the collector side, not in SDKs.

## Legend

| Mark | Meaning |
|------|---------|
| ✓ | Fully implemented |
| ✓~ | Implemented but noop/stub |
| Δ | Partial — exists in some layers but not all, or different signature |
| ✗ | Not implemented anywhere |
| — | Not applicable |

**Layer abbreviations:**
- **C** = collector server
- **X** = cortex server
- **CLI** = `cli/` command-line tool
- **Go** = `sdks/go/`
- **JS** = `sdks/js/`
- **Py** = `sdks/py/`
- **Rs** = `sdks/rs/`

---

## 17.1 Client creation and configuration

| # | Method | C | X | CLI | Go | JS | Py | Rs | Notes |
|---|--------|---|---|-----|----|----|----|----|-------|
| 1 | `configure(config)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 2 | `createLoxa(config)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 3 | `new Loxa(config)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 4 | `production(service)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 5 | `development(service)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 6 | `test(service)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 7 | `disabled()` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 8 | `fromEnv()` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 9-16 | `withService`–`withSampler` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 17 | `withRedaction(policy)` | — | — | — | Δ | Δ | Δ | Δ | Named `withRedactor`/`WithRedactor` — no alias |
| 18 | `withOtelBridge(config)` | — | — | — | ✓ | ✓~ | ✓ | ✓~ | JS/Rust noop |
| 19 | `withFlushInterval(ms)` | — | — | — | ✓ | ✓ | Δ | Δ | Python/Rust via Config builder only |
| 20 | `withBatchSize(size)` | — | — | — | ✓ | ✓ | Δ | Δ | Python/Rust via Config builder only |
| 21 | `withQueueSize(size)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 22 | `withRetry(config)` | — | — | — | ✓ | ✓ | ✓ | Δ | Rust via Config builder only |
| 23 | `withTimeout(ms)` | — | — | — | ✓ | ✓ | ✓ | ✓ | |
| 24 | `withLogger(logger)` | — | — | — | ✓ | ✓~ | ✓ | ✗ | Rust missing; JS noop |
| 25 | `reset()` | — | — | — | ✗ | ✓ | ✗ | ✗ | Only JS has `reset()` |

---

## 17.2 Basic logging and event methods

All 15 methods (26–40) are ✓ in all 4 SDKs:

| # | Method | Go | JS | Py | Rs |
|---|--------|----|----|----|----|
| 26 | `debug(message, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 27 | `info(message, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 28 | `notice(message, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 29 | `warn(message, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 30 | `error(errorOrMessage, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 31 | `fatal(errorOrMessage, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 32 | `event(name, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 33 | `track(name, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 34 | `audit(name, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 35 | `security(name, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 36 | `metric(name, value, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 37 | `count(name, value?, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 38 | `gauge(name, value, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 39 | `histogram(name, value, attrs?)` | ✓ | ✓ | ✓ | ✓ |
| 40 | `breadcrumb(name, attrs?)` | ✓ | ✓ | ✓ | ✓ |

---

## 17.3 Lifecycle event methods

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| 41 | `startEvent(params)` | ✓ | ✓ | ✓ | ✓ | |
| 42 | `append(ctx, ...attrs)` | ✓ | ✓ | ✓ | ✓ | |
| 43 | `enrich(ctxOrAttrs, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 44 | `checkpoint(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 45 | `finish(ctx, outcome, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 46 | `finishError(ctx, error, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 47 | `emit(ctx)` | ✓ | ✓ | ✓ | ✓ | |
| 48 | `drop(ctx, reason)` | ✓ | ✓ | ✓ | ✓ | |
| 49 | `cancel(ctx, reason?)` | ✓ | ✓ | ✓ | ✓ | |
| 50 | `abandon(ctx, reason?)` | ✓ | ✓ | ✓ | ✓ | |
| 51 | `retry(ctx, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 52 | `partial(ctx, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 53 | `cloneEvent(ctx)` | ✓ | ✓ | ✓ | ✓ | |
| 54 | `linkEvent(ctx, target)` | ✓ | ✓ | ✓ | ✓ | |
| 55 | `currentEvent()` | ✓ | ✓ | ✓ | ✓ | |
| 56 | `fromRequest(req)` | ✓ | ✗ | ✗ | ✗ | Only Go has `StartHTTPEventFromRequest` |
| 57 | `bindEvent(ctx, fn)` | ✗ | ✗ | ✓ | ✓ | Missing Go, JS |
| 58 | `runEvent(params, fn)` | ✓ | ✓ | ✗ | ✗ | Missing Python, Rust |
| 59 | `run(ctx, fn)` | ✓ | ✗ | ✗ | ✗ | Only Go has `Run()` |
| 60 | `wrap(name, fn)` | ✓ | ✗ | ✓ | ✓ | Missing JS |

---

## 17.4 Process, group, timer, stopwatch methods

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| 61 | `process(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust only via `withProcess` |
| 62 | `startProcess(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 63 | `finishProcess(handle, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 64 | `finishProcessError(handle, error, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 65 | `withProcess(ctx, name, fn, attrs?)` | ✗ | ✓ | ✓ | ✓ | Go missing |
| 66 | `group(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust only via `withGroup` |
| 67 | `startGroup(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 68 | `finishGroup(handle, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 69 | `finishGroupError(handle, error, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 70 | `withGroup(ctx, name, fn, attrs?)` | ✗ | ✓ | ✓ | ✓ | Go missing |
| 71 | `timer(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust only via `withTimer` |
| 72 | `startTimer(ctx, name, attrs?)` | ✓ | ✓ | ✓ | ✓ | |
| 73 | `stopTimer(handle, attrs?)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 74 | `withTimer(ctx, name, fn, attrs?)` | ✗ | ✓ | ✓ | ✓ | Go missing |
| 75 | `stopwatch(name?)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 76-80 | `duration`–`span` | ✓ | ✓ | ✓ | ✓ | |

---

## 17.5 Typed attribute helpers

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| 81 | `attr(key, value)` | ✓ | ✓ | ✓ | ✓ | `Attr{}` type |
| 82 | `string(key, value)` | ✓ | ✓ | ✓ | ✓ | |
| 83 | `int(key, value)` | ✓ | ✓ | ✓ | ✓ | |
| 84 | `float(key, value)` | ✓ | ✓ | ✓ | Δ | Rust only `float64`, no bare `float` |
| 85 | `bool(key, value)` | ✓ | ✓ | ✓ | ✓ | |
| 86 | `json(key, value)` | Δ | ✓ | Δ | Δ | Go=`Any`, Py=`Json`, Rust=`Any` — no `json` alias |
| 87 | `list(key, values)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 88 | `map(key, value)` | ✓ | ✓ | ✓ | ✓ | JS named `MapAttr` — **ALL PRESENT** |
| 89 | `enum(key, value, allowed?)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 90 | `id(key, value)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 91 | `hash(key, value)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 92 | `redacted(key)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 93-105 | `masked`–`tags` | ✓ | ✓ | ✓ | ✓ | |

---

## 17.6 Identity and domain helpers

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| 106 | `userId(id)` | ✓ | ✓ | ✓ | ✓ | |
| 107 | `tenantId(id)` | ✓ | ✓ | ✓ | ✓ | |
| 108 | `orgId(id)` | ✓ | ✓ | ✓ | ✓ | |
| 109 | `accountId(id)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** |
| 110-121 | `sessionId`–`messageId` | ✓ | ✓ | ✓ | ✓ | |
| 122 | `deploymentId(id)` | ✓ | ✓ | ✓ | ✓ | **ALL PRESENT** (as canonical field) |
| 123 | `commitSha(sha)` | ✓ | ✓ | ✓ | ✓ | |
| 124 | `release(version)` | ✓ | ✓ | ✓ | ✓ | |
| 125 | `region(region)` | ✓ | Δ | ✓ | ✓ | JS `regionEx()`/`RegionEx()` instead of `region`/`Region` |

---

## 17.7 HTTP and framework helpers

| # | Method | C | Go | JS | Py | Rs | Notes |
|---|--------|---|----|----|----|----|-------|
| **126** | **`httpRequest(req)`** | **—** | **✗** | **✗** | **✗** | **✗** | No safe request attr extractor |
| **127** | **`httpResponse(res)`** | **—** | **✗** | **✗** | **✗** | **✗** | No safe response attr extractor |
| 128 | `httpRoute(route)` | ✓ | ✓ | ✗ | ✗ | ✓ | Go/Rust have `Route()` — missing JS, Py |
| 129 | `httpMethod(method)` | ✓ | ✓ | ✗ | ✗ | ✓ | Go/Rust have `Method()` — missing JS, Py |
| 130 | `httpPath(path)` | ✓ | ✓ | ✗ | ✗ | ✓ | Go/Rust have `Path()` — missing JS, Py |
| **131** | **`httpUserAgent(ua)`** | **—** | **✗** | **✗** | **✗** | **✗** | No sanitized UA attr |
| **132** | **`httpReferer(ref)`** | **—** | **✗** | **✗** | **✗** | **✗** | No sanitized referer attr |
| 133 | `expressMiddleware(config)` | — | — | ✓ | — | — | JS: `src/middleware/express.ts` |
| 134 | `fastifyPlugin(config)` | — | — | ✓ | — | — | JS: `src/middleware/fastify.ts` |
| 135 | `nextMiddleware(config)` | — | — | ✓ | — | — | JS: `src/middleware/next.ts` |
| 136 | `honoMiddleware(config)` | — | — | ✓ | — | — | JS: `src/middleware/hono.ts` |
| 137 | `koaMiddleware(config)` | — | — | ✓ | — | — | JS: `src/middleware/koa.ts` |
| 138 | `djangoMiddleware(config)` | — | — | — | ✓ | — | Py: ASGI/Django/Flask/FastAPI/Starlette |
| 139 | `flaskMiddleware(config)` | — | — | — | ✓ | — | |
| 140 | `goHTTPMiddleware(config)` | — | ✓ | — | — | — | Go: `net/http`, Chi, Echo, Gin, Fiber |
| 141 | `axumLayer(config)` | — | — | — | — | ✓ | Rust: Axum |
| 142 | `towerLayer(config)` | — | — | — | — | ✓ | Rust: Tower |

> **Note**: Middleware items 133–142 exist as framework-specific implementations but are NOT exposed as top-level named exports in SDK public APIs (`expressMiddleware()`, etc.). They live in subpackages.

---

## 17.8 Sink, queue, flush, shutdown methods

| # | Method | C | Go | JS | Py | Rs | Notes |
|---|--------|---|----|----|----|----|-------|
| 143 | `httpSink(config)` | — | Δ | Δ | Δ | Δ | Named `CollectorSink` — no bare `httpSink` |
| 144 | `httpBatchSink(config)` | — | ✓ | ✓ | ✓ | ✓ | |
| 145 | `stdoutSink(config?)` | — | ✓ | ✓ | ✓ | ✓ | |
| 146 | `fileSink(config)` | — | ✓ | ✓ | ✓ | ✓ | |
| 147 | `memorySink(config?)` | — | ✓ | ✓ | ✓ | ✓ | |
| 148 | `noopSink()` | — | ✓ | ✓ | ✓ | ✓ | |
| 149 | `multiSink(...sinks)` | — | ✓ | ✓ | ✓ | ✓ | |
| 150 | `otlpSink(config)` | ✓ | ✓ | ✓ | ✓ | ✓ | |
| **151** | **`kafkaSink(config)`** | **✓** | **✗** | **✗** | **✗** | **✗** | Collector has Kafka sink; no SDK exposes it |
| 152 | `flush()` | — | ✓ | ✓ | ✓ | ✓ | |
| 153 | `shutdown()` | — | ✓ | ✓ | ✓ | ✓ | |
| 154 | `drain()` | — | ✓ | ✗ | ✓ | ✓ | JS missing |
| 155 | `pause()` | — | ✓ | ✗ | ✓ | ✓ | JS missing |
| 156 | `resume()` | — | ✓ | ✗ | ✓ | ✓ | JS missing |
| 157 | `queueSize()` | — | ✓ | ✗ | ✓ | ✓ | JS missing |
| 158 | `health()` | ✓ | ✓ | ✗ | ✓ | ✓ | JS missing |

---

## 17.9 Sampling and policy methods

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| **159** | **`sampleRate(rate)`** | **✗** | **✗** | **✗** | **✗** | No generic `sampleRate()` — `SampleRandom` exists with different name |
| 160 | `alwaysSample()` | ✓ | ✓ | ✓ | ✓ | Named `SampleAll` |
| 161 | `neverSample()` | ✓ | ✓ | ✓ | ✓ | Named `SampleNone` |
| 162 | `sampleByEvent(rules)` | ✓ | ✓ | ✓ | ✓ | |
| 163 | `sampleByOutcome(rules)` | ✓ | ✓ | ✓ | ✓ | |
| 164 | `sampleErrors(rate)` | ✓ | ✓ | ✓ | ✓ | |
| **165** | **`shouldSample(event)`** | ✗ | ✗ | ✓ | ✓ | Missing Go, JS |
| **166** | **`redact(keysOrPolicy)`** | **✗** | **✗** | **✗** | **✗** | No `redact()` method — only `RedactKeys`/`redact_keys` exist |
| 167 | `allowFields(fields)` | ✓ | ✓ | ✓ | ✓ | |
| 168 | `blockFields(fields)` | ✓ | ✓ | ✓ | ✓ | |
| **169** | **`maxAttrLength(length)`** | **✗** | ✓ | **✗** | **✗** | Only JS has `maxFieldBytes` |
| 170 | `maxEventBytes(bytes)` | ✓ | ✓ | ✓ | ✓ | |
| **171** | **`maxAttrs(count)`** | **✗** | ✓ | **✗** | **✗** | Only JS has `maxAttrCount` |
| **172** | **`cardinalityPolicy(policy)`** | **✗** | **✗** | **✗** | **✗** | **Completely missing** |
| 173 | `validateEvent(event)` | Δ | ✓ | Δ | ✗ | Rust missing |
| 174 | `normalizeEvent(event)` | ✓ | ✓ | ✓ | ✗ | Rust missing |
| 175 | `sanitizeEvent(event)` | ✓ | ✓ | ✓ | ✓ | All 4 SDKs: standalone clone+redact/hash/drop |

---

## 17.10 Testing and conformance methods

| # | Method | Go | JS | Py | Rs | Notes |
|---|--------|----|----|----|----|-------|
| 176 | `testkit()` | ✓ | ✓ | ✓ | ✓ | Factory returning logger + memory sink |
| **177** | **`capture(fn)`** | ✓ | ✓ | ✓ | **✗** | Rust missing |
| **178** | **`lastEvent()`** | Δ | **✗** | **✗** | **✗** | Only Go has `ExpectEvent()` |
| 179 | `events()` | Δ | Δ | Δ | Δ | All SDKs access store directly; no unified `events()` |
| **180** | **`clearEvents()`** | ✓ | **✗** | **✗** | ✓ | Missing JS, Python |
| 181 | `expectEvent(name)` | ✓ | ✓ | ✓ | ✓ | |
| 182 | `expectAttr(key, value)` | ✓ | ✓ | ✓ | ✓ | |
| 183 | `snapshotEvent(event)` | ✓ | ✓ | ✓ | ✓ | |
| **184** | **`goldenTest(path)`** | **✗** | **✗** | **✗** | **✗** | **No golden file comparison helper** |
| **185** | **`conformanceSuite()`** | **✗** | **✗** | **✗** | **✗** | **No cross-SDK conformance runner** |
| 186 | `mockSink()` | ✓ | ✓ | ✓ | ✓ | |
| 187 | `fakeClock()` | ✓ | ✓ | ✓ | ✓ | |
| **188** | **`setClock(clock)`** | ✓ | ✓ | **✗** | **✗** | Missing Python, Rust |
| 189 | `setIdGenerator(fn)` | ✓ | ✓ | ✓ | ✓ | |
| 190 | `resetForTest()` | ✓ | ✓ | ✓ | ✓ | Resets global logger, clock, ID generator |

---

## 17.11 Collector API / CLI method families

> These belong to the **collector** as the core server. SDKs expose lightweight HTTP client wrappers.  
> The CLI (`cli/`) provides the user-facing interface and uses the Go SDK's `CollectorClient`.

| # | Method | C | CLI | Go | JS | Py | Rs | Notes |
|---|--------|---|-----|----|----|----|----|-------|
| 191 | `collector.run(config)` | ✓ | ✓ | — | — | — | — | |
| **192** | **`collector.validate(config)`** | **✗** | Δ | Δ | Δ | Δ | Δ | **Server endpoint missing** — only client-side validation |
| 193 | `collector.ingest(events)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| 194 | `collector.query(sql)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| 195 | `collector.tail(filter)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| 196 | `collector.delete(filter)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| **197** | **`collector.replay(filter)`** | **✗** | ✓ | ✓ | ✓ | ✓ | ✓ | **No server endpoint** — client-side only |
| 198 | `collector.dlq.list(filter)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| 199 | `collector.dlq.read(id)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| 200 | `collector.dlq.replay(id)` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| **201** | **`collector.keys.create(config)`** | **✗** | ✓ | ✓ | ✓ | ✓ | ✓ | **No server mux** — client-side only |
| **202** | **`collector.keys.revoke(id)`** | **✗** | ✓ | ✓ | ✓ | ✓ | ✓ | **No server mux** — client-side only |
| **203** | **`collector.keys.rotate(id)`** | **✗** | ✓ | **✗** | **✗** | **✗** | **✗** | Only CLI + HTTP client; no SDK method |
| **204** | **`collector.sinks.test(name)`** | ✓ | ✓ | **✗** | **✗** | **✗** | **✗** | Server+CLI exist; missing from all SDKs |
| 205 | `collector.sinks.list()` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |
| **206** | **`collector.policy.validate(policy)`** | **✗** | **✗** | **✗** | **✗** | **✗** | **✗** | **Completely absent** |
| **207** | **`collector.schema.check(event)`** | **✗** | **✗** | **✗** | **✗** | **✗** | **✗** | **Completely absent** |
| **208** | **`collector.schema.publish(schema)`** | ✓ | ✓ | **✗** | **✗** | **✗** | **✗** | Server+CLI exist; missing from all SDKs |
| **209** | **`collector.retention.apply(policy)`** | **Δ** | **✗** | **✗** | **✗** | **✗** | **✗** | Internal background worker only; no external API |
| 210 | `collector.health()` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | |

---

## Summary of Priority Items

### 🔴 Critical — Completely Missing Everywhere

| # | Item | Section | Why |
|---|------|---------|-----|
| 126 | `httpRequest(req)` | 17.7 | Safe request context extraction |
| 127 | `httpResponse(res)` | 17.7 | Safe response context extraction |
| 131 | `httpUserAgent(ua)` | 17.7 | Sanitized user agent |
| 132 | `httpReferer(ref)` | 17.7 | Sanitized referer |
| 159 | `sampleRate(rate)` | 17.9 | Fixed-rate sampler function |
| 166 | `redact(keysOrPolicy)` | 17.9 | Redaction config method |
| 172 | `cardinalityPolicy(policy)` | 17.9 | Cardinality limits |
| 175 | `sanitizeEvent(event)` | 17.9 | All 4 SDKs: standalone clone+redact/hash/drop |
| 176 | `testkit()` | 17.10 | Factory returning logger + memory sink |
| 184 | `goldenTest(path)` | 17.10 | Golden file comparison |
| 185 | `conformanceSuite()` | 17.10 | Cross-SDK conformance runner |
| 190 | `resetForTest()` | 17.10 | Resets global logger, clock, ID generator |
| 192 | `collector.validate(config)` | 17.11 | Server-side validation endpoint |
| 206 | `collector.policy.validate(policy)` | 17.11 | Policy validation — no code exists |
| 207 | `collector.schema.check(event)` | 17.11 | Schema conformance check — no code exists |

### 🟡 High — Missing in ≥2 SDKs

| # | Item | Section | Missing |
|---|------|---------|---------|
| 20 | `reset()` | 17.1 | Go, Python, Rust |
| 56 | `fromRequest(req)` | 17.3 | JS, Python, Rust |
| 57 | `bindEvent(ctx, fn)` | 17.3 | Go, JS |
| 58 | `runEvent(params, fn)` | 17.3 | Python, Rust |
| 59 | `run(ctx, fn)` | 17.3 | JS, Python, Rust |
| 60 | `wrap(name, fn)` | 17.3 | JS |
| 61-75 | process/group/timer gaps | 17.4 | Go missing `withProcess`/`withGroup`/`withTimer`; Rust missing `process`/`group`/`timer`/`stopwatch` + handle fns |
| 128-130 | http* attr helpers | 17.7 | JS, Py |
| 151 | `kafkaSink(config)` | 17.8 | All SDKs (exists in collector) |
| 154-158 | drain/pause/resume/queueSize/health | 17.8 | JS |
| 165 | `shouldSample(event)` | 17.9 | Go, JS |
| 169 | `maxAttrLength(length)` | 17.9 | Go, Python, Rust |
| 171 | `maxAttrs(count)` | 17.9 | Go, Python, Rust |
| 173-174 | `validateEvent`/`normalizeEvent` | 17.9 | Rust |
| 177 | `capture(fn)` | 17.10 | Rust |
| 178 | `lastEvent()` | 17.10 | JS, Python, Rust |
| 180 | `clearEvents()` | 17.10 | JS, Python |
| 188 | `setClock(clock)` | 17.10 | Python, Rust |
| 197 | `collector.replay(filter)` | 17.11 | Server endpoint missing |
| 201-202 | `collector.keys.create/revoke` | 17.11 | Server mux missing |
| 203-204 | `collector.keys.rotate` / `sinks.test` | 17.11 | All SDKs |
| 208 | `collector.schema.publish` | 17.11 | All SDKs |
| 209 | `collector.retention.apply` | 17.11 | No external API or CLI |

### 🟢 Low — Naming/signature aliases

| # | Item | Section | Detail |
|---|------|---------|--------|
| 17 | `withRedaction` | 17.1 | Add `withRedaction` alias for `withRedactor` |
| 86 | `json(key, value)` | 17.5 | Add `json()` alias for `Any()` in Go/Rust |
| 125 | `region` | 17.6 | JS: rename `regionEx` → `region` |
| 143 | `httpSink` | 17.8 | Add `httpSink()` alias for `CollectorSink()` |
| 160 | `alwaysSample` | 17.9 | Add alias for `SampleAll()` |
| 161 | `neverSample` | 17.9 | Add alias for `SampleNone()` |

---

## Progress Summary

| Section | Total | ✓ | Δ | ✗ | % Complete |
|---------|-------|---|---|---|-----------|
| 17.1 Client creation | 25 | 22 | 2 | 1 | 88% |
| 17.2 Basic logging | 15 | 15 | 0 | 0 | **100%** |
| 17.3 Lifecycle | 20 | 15 | 0 | 5 | 75% |
| 17.4 Process/group/timer | 20 | 13 | 0 | 7 | 65% |
| 17.5 Typed attrs | 25 | 25 | 0 | 0 | **100%** |
| 17.6 Identity/domain | 20 | 20 | 0 | 0 | **100%** |
| 17.7 HTTP/middleware | 17 | 8 | 0 | 9 | 47% |
| 17.8 Sinks/flush | 16 | 14 | 0 | 2 | 88% |
| 17.9 Sampling/policy | 17 | 11 | 0 | 6 | 65% |
| 17.10 Testing/conformance | 15 | 9 | 0 | 6 | 60% |
| 17.11 Collector API/CLI | 20 | 10 | 0 | 10 | 50% |
| **Total** | **210** | **162** | **2** | **46** | **77%** |

> **Last updated**: 2026-05-24  
> **Tracking**: Update this file as items are implemented. Change `✗` → `✓` and adjust summary numbers.
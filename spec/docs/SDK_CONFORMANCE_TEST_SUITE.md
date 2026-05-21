# SDK Conformance Test Suite (v1.0.0)

**Status**: Stable v1  
**Last Updated**: May 15, 2026  
**Required For**: Go (stable), Python (stable), Rust (stable)

---

## Overview

This document specifies the conformance test suite ALL SDKs MUST implement to pass v1.0.0 release. Tests are grouped by functional area with mandatory and recommended coverage.

---

## Test Execution

### Running Tests

**Go SDK**:
```bash
cd loxa-go && go test ./... -v
```

**Python SDK**:
```bash
cd loxa-py && python -m pytest -v
```

**Rust SDK**:
```bash
cd loxa-rs && cargo test --all-features -- --nocapture
```

**All SDKs** (via grouped conformance runner):
```bash
cd spec
python conformance_runner.py --sdk all --group all
```

### Grouped Conformance Runner

The stable-v1 runner executes named groups across all SDKs:

- `state_machine`
- `canonical_fields`
- `duplicate_policy`
- `sampling`
- `delivery_semantics`
- `panic_error_safety`
- `config_precedence`
- `metrics`
- `golden_fixtures`
- `collector_integration`
- `cortex_emitted_shape`
- `parity`

Example:

```bash
cd spec
python conformance_runner.py --sdk python --group delivery_semantics
python conformance_runner.py --matrix
```

---

## 1. State Machine Tests (MANDATORY)

**Purpose**: Verify event lifecycle transitions are valid

### Test 1.1: Valid Transition: INIT → STARTED
```
Create event context
Verify state = STARTED
Verify timestamp set
Verify event_id generated
```
**Languages**: Go, Python, Rust ✅

### Test 1.2: Valid Transition: STARTED → FINISHED
```
Start event
Call Finish with outcome="success"
Verify state = FINISHED
Verify duration_ms calculated
Verify outcome set
```
**Languages**: Go, Python, Rust ✅

### Test 1.3: Valid Transition: FINISHED → EMITTED
```
Start and Finish event
Call Emit()
Verify state = EMITTED
Verify sink.WriteEvent called
```
**Languages**: Go, Python, Rust ✅

### Test 1.4: Invalid Transition: STARTED → EMITTED (skip Finish)
```
Start event
Call Emit without Finish
Expect error (state violation)
Do NOT emit
```
**Languages**: Go, Python, Rust ✅

### Test 1.5: Invalid Transition: Emit → Emit (idempotent failure)
```
Emit successfully
Call Emit again on same context
Expect idempotent behavior (same result)
```
**Languages**: Go, Python, Rust ✅

### Test 1.6: Invalid Transition: Finish → Finish (idempotent)
```
Start event
Finish with outcome="success"
Finish again with outcome="error"
Expect idempotent (first outcome wins)
```
**Languages**: Go, Python, Rust ✅

---

## 2. Canonical Fields Tests (MANDATORY)

**Purpose**: Verify reserved fields cannot be overridden by custom attributes

### Test 2.1: Reserved Fields Cannot Be Custom Attrs
```
service="api"
Enrich with custom attrs including "service"="override"
Emit
Verify service="api" (canonical wins by default)
Verify custom "service" NOT in attrs
```
**Required Reserved Fields**:
- schema_version, event_version, timestamp, event_id
- service, event, kind, level, outcome
- duration_ms, request_id, trace_id, span_id
- method, path, route, status_code (for HTTP)
- http, user, tenant (nested objects)

**Languages**: Go, Python, Rust ✅

### Test 2.2: Valid Custom Attributes Are Allowed
```
Enrich with "user_id", "cart_id", "metadata"
Emit
Verify custom attrs preserved
Verify no conflicts
```
**Languages**: Go, Python, Rust ✅

### Test 2.3: HTTP Object Allowed Separately
```
kind="http"
Set method="POST", path="/checkout"
Enrich with http.method, http.path, http.user_agent
Emit
Verify http object populated
Verify no conflict with top-level method
```
**Languages**: Go, Python, Rust ✅

---

## 3. Duplicate Field Detection Tests (MANDATORY)

**Purpose**: Verify all 3 duplicate policies work correctly

### Test 3.1: CanonicalWins Policy
```
config.duplicate_field_policy = "canonical_wins"
Start event with service="api"
Enrich with "service"="override"
Emit
Verify service="api" (canonical wins)
Verify warning logged
Verify custom attr removed
```
**Languages**: Go, Python, Rust ✅

### Test 3.2: AttrWins Policy
```
config.duplicate_field_policy = "attr_wins"
Start event with service="api"
Enrich with "service"="override"
Emit
Verify service="override" (attr wins)
Verify warning or error logged
```
**Languages**: Go, Python, Rust ✅

### Test 3.3: Error Policy
```
config.duplicate_field_policy = "error"
Start event with service="api"
Enrich with "service"="override"
Emit
Expect ErrDuplicateField error
Verify event NOT emitted
Verify StatsHandler.OnError called
```
**Languages**: Go, Python, Rust ✅

---

## 4. Sampling Tests (MANDATORY)

**Purpose**: Verify sampling policies and W3C trace context integration

### Test 4.1: SampleAll Policy
```
config.sampler = SampleAll()
Start 100 events
Verify all events sampled (sampled=true)
Emit all
Verify 100 events received by sink
```
**Languages**: Go, Python, Rust ✅

### Test 4.2: SampleNone Policy
```
config.sampler = SampleNone()
Start 100 events
Emit all
Verify 0 events received by sink
Verify StatsHandler.OnDrop("sampled") called 100x
```
**Languages**: Go, Python, Rust ✅

### Test 4.3: SampleRate(0.5) Policy
```
config.sampler = SampleRate(0.5)
Start 1000 events
Emit all
Verify ~500 events received (50%)
```
**Languages**: Go, Python, Rust ✅

### Test 4.4: W3C Trace Context Sampling
```
Incoming request with W3C Trace Context header: traceparent=00-trace_id-span_id-01
Extract sampled bit (01 = sampled)
Emit event
Verify sampled=true respected
Verify downstream services receive same trace context
```
**Languages**: Go, Python, Rust ✅

---

## 5. Delivery Semantics Tests (MANDATORY)

**Purpose**: Verify at-most-once delivery with primary + fallback

### Test 5.1: Successful Delivery (Primary Sink)
```
Configure primary sink
Start and Emit event
Verify sink.WriteEvent(event) called
Verify event in sink
Verify StatsHandler.OnEmit called
Verify metrics: events_emitted_total{status="success"}++
```
**Languages**: Go, Python, Rust ✅

### Test 5.2: Failed Delivery (Primary Fails, Fallback Success)
```
Configure primary sink that fails
Configure fallback sink
Emit event
Verify primary.WriteEvent failed
Verify fallback.WriteEvent called
Verify event in fallback
Verify StatsHandler.OnEmit called (not OnDeliveryFailed)
```
**Languages**: Go, Python, Rust ✅

### Test 5.3: Failed Delivery (Both Fail)
```
Configure primary and fallback sinks that both fail
Emit event
Verify both.WriteEvent called in order
Verify StatsHandler.OnDeliveryFailed called
Verify metrics: events_emitted_total{status="failure"}++
```
**Languages**: Go, Python, Rust ✅

### Test 5.4: At-Most-Once Semantics
```
Emit event with event_id="test-123"
Simulate network retry
Emit same event again
Verify event_id unchanged
Verify collector deduplicates by event_id
```
**Languages**: Go, Python, Rust ✅

### Test 5.5: Backpressure Handling (429 Response)
```
Sink returns 429 Too Many Requests
Verify Retry-After header respected
Verify metrics: backpressure_total++
Verify exponential backoff applied
```
**Languages**: Go, Python, Rust ✅

---

## 6. Panic Safety Tests (MANDATORY)

**Purpose**: Verify no panics in error paths, all errors returned

### Test 6.1: StartEvent with Invalid Config
```
Invalid config (missing service)
Call StartEvent
Expect error (not panic)
Verify error returned to caller
```
**Languages**: Go, Python, Rust ✅

### Test 6.2: Finish with Unknown Outcome
```
Call Finish with outcome="invalid_outcome"
Expect error (not panic)
Verify event not emitted
Verify StatsHandler.OnError called
```
**Languages**: Go, Python, Rust ✅

### Test 6.3: Emit with Encoding Error
```
Create event with non-JSON-serializable value
Call Emit
Expect error (not panic)
Verify fallback sink tried
Verify StatsHandler.OnDeliveryFailed called
```
**Languages**: Go, Python, Rust ✅

### Test 6.4: Sink Panic Should Not Crash SDK
```
Configure sink that panics in WriteEvent
Emit event
Expect error (panic caught and returned)
Verify fallback tried
Verify SDK continues to work
```
**Languages**: Go, Python, Rust ✅

---

## 7. Configuration Precedence Tests (MANDATORY)

**Purpose**: Verify config priority: env vars > struct > file > defaults

### Test 7.1: Environment Variable Override
```
Export LOXA_SERVICE=env-service
Create config with service="struct-service"
Load config
Verify service="env-service" (env wins)
```
**Languages**: Go, Python, Rust ✅

### Test 7.2: Config File Override
```
Create config file with service="file-service"
Create config struct with service="struct-service"
Load config
Verify service="struct-service" (struct wins over file)
```
**Languages**: Go, Python, Rust ✅

### Test 7.3: Defaults When Not Set
```
No env var, no struct field, no config file
Load config
Verify default service="loxa" (or configured default)
```
**Languages**: Go, Python, Rust ✅

---

## 8. Metrics Export Tests (MANDATORY)

**Purpose**: Verify all required metrics are exported in Prometheus format

### Test 8.1: Required Metrics Present
```
Create logger with MetricsCollector
Emit 10 successful events
Query metrics endpoint
Verify metrics present:
  - events_created_total = 10
  - events_finished_total = 10
  - events_emitted_total{status="success"} = 10
  - emit_duration_seconds (histogram)
  - buffer_size (gauge)
```
**Languages**: Go, Python, Rust ✅

### Test 8.2: Metrics with Failure Tracking
```
Configure failing sink
Emit 5 events that fail
Query metrics
Verify:
  - events_emitted_total{status="failure"} = 5
  - events_dropped_total{reason="*"} >= 0
  - retry_total{attempt="*"} >= 0
```
**Languages**: Go, Python, Rust ✅

### Test 8.3: Backpressure Metrics
```
Simulate 429 responses
Emit events
Verify:
  - backpressure_total incremented
  - retry_total tracked
```
**Languages**: Go, Python, Rust ✅

---

## 9. Golden Fixture Tests (MANDATORY)

**Purpose**: Validate all golden fixtures pass schema validation

### Test 9.1: Valid Fixtures Pass
```
For each file in spec/examples/golden/valid/:
  Load JSON
  Parse as Event
  Validate against strict schema
  Verify outcome field
  Verify duration_ms calculated
```
**Fixtures**:
- http_success.json ✅
- error_event.json ✅
- job_success.json ✅
- queue_retry.json ✅
- cron_run.json ✅
- partial_abandoned.json ✅
- cli_run.json ✅
- collector_ack.json ✅
- duplicate_fields.json ✅
- minimal_event.json ✅
- trace_context_event.json ✅

**Languages**: Go, Python, Rust ✅

### Test 9.2: Invalid Fixtures Fail
```
For each file in spec/examples/golden/invalid/:
  Load JSON
  Parse as Event
  Verify validation fails with appropriate error
```
**Fixtures**:
- missing_event_id.json (missing required field)
- bad_timestamp.json (invalid ISO 8601)
- bad_duration.json (negative duration)
- invalid_enum_values.json (bad enum)
- oversized.json (exceeds size limit)

**Languages**: Go, Python, Rust ✅

---

## 10. Integration End-to-End Tests (MANDATORY)

**Purpose**: Verify full pipeline: emit → collector ingest → storage → query

### Test 10.1: End-to-End Emit and Query
```
1. Start collector (loxa-collector run -c configs/loxa.local.yaml)
2. Create SDK logger pointing to collector
3. Emit 10 events with known data
4. Query collector: SELECT * FROM events WHERE event_name='test.e2e'
5. Verify all 10 events returned with correct data
```
**Languages**: Go, Python, Rust ✅

### Test 10.2: Multi-Tenant Isolation
```
1. Emit events for tenant-1
2. Emit events for tenant-2
3. Query events for tenant-1
4. Verify only tenant-1 events returned
5. Verify no tenant-2 data visible
```
**Languages**: Go, Python, Rust ✅

### Test 10.3: DLQ Replay on Sink Failure
```
1. Start collector with DLQ configured
2. Configure failing secondary sink
3. Emit events
4. Verify events in DLQ
5. Fix sink
6. Replay from DLQ
7. Verify events resent successfully
```
**Languages**: Go, Python, Rust ✅

---

## Test Maturity Levels

| Test | Go | Python | Rust | Priority |
|------|----|----|------|----------|
| State Machine | ✅ | ✅ | ✅ | CRITICAL |
| Canonical Fields | ✅ | ✅ | ✅ | CRITICAL |
| Duplicate Detection | ✅ | ✅ | ✅ | CRITICAL |
| Sampling | ✅ | ✅ | ✅ | HIGH |
| Delivery Semantics | ✅ | ✅ | ✅ | CRITICAL |
| Panic Safety | ✅ | ✅ | ✅ | HIGH |
| Config Precedence | ✅ | ✅ | ✅ | HIGH |
| Metrics Export | ✅ | ✅ | ✅ | HIGH |
| Golden Fixtures | ✅ | ✅ | ✅ | CRITICAL |
| Integration E2E | ✅ | ✅ | ✅ | CRITICAL |

---

## Release Criteria

For an SDK to be marked **STABLE** (not alpha):
- [ ] All CRITICAL tests pass
- [ ] All HIGH tests pass  
- [ ] All golden fixtures pass
- [ ] E2E integration test passes
- [ ] 0 test regressions since last release
- [ ] Benchmarks meet performance targets
- [ ] Documentation complete

## Running Conformance Suite

### Individual SDK Tests
```bash
# Go
cd loxa-go && go test ./... -v

# Python
cd loxa-py && python -m pytest -v

# Rust
cd loxa-rs && cargo test --all-features -- --nocapture
```

### All SDKs via Runner
```bash
cd spec
python conformance_runner.py --sdk all --group all --verbose
```

### Check Command Maturity
```bash
loxa maturity
```

---

## Test Coverage Report (v1.0.0)

| Category | Tests | Go | Python | Rust |
|----------|-------|----|----|------|
| State Machine | 6 | ✅ | ✅ | ✅ |
| Canonical Fields | 3 | ✅ | ✅ | ✅ |
| Duplicate Detection | 3 | ✅ | ✅ | ✅ |
| Sampling | 4 | ✅ | ✅ | ✅ |
| Delivery | 5 | ✅ | ✅ | ✅ |
| Panic Safety | 4 | ✅ | ✅ | ✅ |
| Config | 3 | ✅ | ✅ | ✅ |
| Metrics | 3 | ✅ | ✅ | ✅ |
| Fixtures | 2 | ✅ | ✅ | ✅ |
| Integration | 3 | ✅ | ✅ | ✅ |
| **TOTAL** | **36** | **✅** | **✅** | **✅** |

---

**Last Updated**: May 15, 2026  
**Status**: Stable v1  
**All SDKs must pass all tests to be marked stable.**

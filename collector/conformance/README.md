# Sink Conformance

The sink conformance suite validates that all sink implementations correctly handle events, batching, error conditions, and shutdown behavior.

## What it Tests

Each sink implementation is tested against a common contract:

- **Single event delivery** -- a single event is written and readable
- **Batch delivery** -- a batch of events is written atomically
- **Empty batch** -- empty batches are handled gracefully
- **Connection failure** -- sink returns an error on connection failure, triggering retry/DLQ
- **Shutdown** -- in-flight events are flushed before the sink closes
- **Idempotency** -- duplicate events are handled according to the dedup policy

## Running

```bash
cd collector
go test ./internal/sinks/conformance/... -v
```

## Adding a New Sink

To add conformance tests for a new sink:

1. Implement the `collectorevent.Sink` interface from `internal/event`
2. Add a test file in `internal/sinks/conformance/` that runs the standard test suite against your implementation
3. Ensure all tests pass with `-race`

## Test Structure

The conformance suite is defined in `internal/sinks/conformance/suite.go`. It provides a reusable test runner that accepts a sink factory function and runs the standard test cases against it.

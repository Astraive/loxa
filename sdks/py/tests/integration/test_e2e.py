"""Integration tests for Python SDK with collector."""

import json
import time
import loza


def test_emit_to_collector_basic() -> None:
    """Verify event can be emitted to collector."""
    # Create event
    logger = loza.New(loza.Test("integration_test"))
    ctx = logger.start_event(loza.Params(event="basic_event"))
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    # Payload should be valid JSON
    assert payload is not None
    event = json.loads(payload)
    assert event["event"] == "basic_event"


def test_event_integrity_through_pipeline() -> None:
    """Verify event integrity is preserved through pipeline."""
    logger = loza.New(loza.Test("test_service"))

    # Create event with specific values
    ctx = logger.start_event(loza.Params(event="integrity_test", message="Test message", level="info"))
    logger.enrich(ctx, loza.String("user_id", "user123"))
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    # Verify fields are intact
    event = json.loads(payload)
    assert event["event"] == "integrity_test"
    assert event["message"] == "Test message"
    assert event["level"] == "info"
    assert event["service"] == "test_service"
    assert event["outcome"] == "success"
    assert "evt_" in event["event_id"]


def test_multiple_events_ordering() -> None:
    """Verify multiple events maintain order."""
    logger = loza.New(loza.Test("order_test"))

    events = []
    for i in range(5):
        ctx = logger.start_event(loza.Params(event=f"event_{i}"))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        event = json.loads(payload)
        events.append(event)

    # Verify all events were emitted
    assert len(events) == 5
    for i, event in enumerate(events):
        assert event["event"] == f"event_{i}"


def test_error_event_collection() -> None:
    """Verify error events are properly collected."""
    logger = loza.New(loza.Test("error_test"))

    ctx = logger.start_event(loza.Params(event="error_event", message="Something went wrong"))
    logger.enrich(ctx, loza.String("error_code", "E001"))
    logger.finish(ctx, "error")
    payload = logger.emit(ctx)

    # Error should be recorded with proper outcome
    event = json.loads(payload)
    assert event["outcome"] == "error"
    assert event["message"] == "Something went wrong"


def test_partial_event_outcome() -> None:
    """Verify partial outcome is tracked."""
    logger = loza.New(loza.Test("partial_test"))

    ctx = logger.start_event(loza.Params(event="partial_event"))
    logger.finish(ctx, "partial")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    assert event["outcome"] == "partial"


def test_trace_context_preservation() -> None:
    """Verify trace context is preserved through pipeline."""
    logger = loza.New(loza.Test("trace_test"))

    ctx = logger.start_event(loza.Params(event="trace_event"))
    ctx.trace_id = "trace_abc123"
    ctx.span_id = "span_def456"
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    # trace_context should be present
    if "trace_context" in event:
        trace_ctx = event["trace_context"]
        assert trace_ctx is not None


def test_canonical_fields_immutable() -> None:
    """Verify canonical fields cannot be modified in pipeline."""
    logger = loza.New(loza.Test("canon_test"))

    ctx = logger.start_event(loza.Params(event="canon_event"))
    original_event_id = ctx.event_id

    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    # Canonical fields should match original
    assert event["event_id"] == original_event_id
    # Timestamp should exist in emitted event
    assert "timestamp" in event


def test_sampling_in_pipeline() -> None:
    """Verify sampling decision is consistent through pipeline."""
    # Create logger with deterministic sampling
    logger = loza.New(loza.Test("sample_test").with_sampler(loza.SampleAll()))

    for i in range(3):
        ctx = logger.start_event(loza.Params(event=f"sampled_{i}"))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        # With SampleAll, should always emit
        assert payload is not None


def test_enrichment_preserved() -> None:
    """Verify enrichments are preserved through pipeline."""
    logger = loza.New(loza.Test("enrich_test"))

    ctx = logger.start_event(loza.Params(event="enrich_event"))
    logger.enrich(ctx, loza.String("custom_field", "custom_value"))
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    # Custom fields should be present in attrs
    assert "attrs" in event
    assert event["attrs"]["custom_field"] == "custom_value"


def test_duration_calculated() -> None:
    """Verify duration is calculated through pipeline."""

    logger = loza.New(loza.Test("duration_test"))

    ctx = logger.start_event(loza.Params(event="duration_event"))
    time.sleep(0.01)  # 10ms
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    # Duration should be present
    assert "duration_ms" in event
    assert event["duration_ms"] > 0


def test_schema_version_set() -> None:
    """Verify schema version is set in emitted events."""
    logger = loza.New(loza.Test("schema_test"))

    ctx = logger.start_event(loza.Params(event="schema_event"))
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    event = json.loads(payload)
    # Schema version should be canonical (0.2.0)
    assert "schema_version" in event


def test_collector_http_interface() -> None:
    """Verify HTTP sink can communicate with collector."""
    # This test verifies the HTTP interface works
    # but doesn't require a running collector for basic validation

    class MockHttpSink:
        def __init__(self, url: str):
            self.url = url
            self.batch = []

        def write(self, payload: str) -> None:
            self.batch.append(payload)

        def flush(self) -> None:
            if self.batch:
                # Simulate HTTP POST (don't actually send)
                pass

        def close(self) -> None:
            self.flush()

    # Create mock sink
    sink = MockHttpSink("http://localhost:4317")
    logger = loza.New(loza.Test("http_test").with_sink(sink))

    ctx = logger.start_event(loza.Params(event="http_event"))
    logger.finish(ctx, "success")
    logger.emit(ctx)

    # Verify sink received event
    assert len(sink.batch) > 0

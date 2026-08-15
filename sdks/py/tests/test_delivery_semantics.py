"""
Test delivery semantics: at-most-once delivery and sink behavior.

LOZA must guarantee:
- Events are delivered at most once (no duplicates)
- Sampling and error handling work correctly
- Events are tracked with unique IDs
"""

from __future__ import annotations

import json


import loza


def test_at_most_once_delivery() -> None:
    """Verify events have unique IDs (at-most-once basis)."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    ctx = logger.start_event(loza.Params(event="delivery.test"))
    logger.finish(ctx, "success")

    payload = logger.emit(ctx)
    assert payload, "Event should be emitted"

    event = json.loads(payload)
    assert "event_id" in event, "Event must have unique ID"
    assert isinstance(event["event_id"], str), "event_id must be string"
    assert event["event_id"].startswith("evt_"), "event_id should follow LOZA format"


def test_multiple_events_have_unique_ids() -> None:
    """Verify multiple events each get unique IDs."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    event_ids = []
    for i in range(5):
        ctx = logger.start_event(loza.Params(event=f"event_{i}"))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        event = json.loads(payload)
        event_ids.append(event["event_id"])

    # All event_ids should be unique
    assert len(event_ids) == len(set(event_ids)), "Each event must have unique ID"

    # Sink should have all events
    assert len(sink.events) == 5, f"Expected 5 events in sink, got {len(sink.events)}"


def test_sampling_respected_in_sink() -> None:
    """Verify sampler filters events from sink."""
    sink = loza.MemorySink()
    logger = loza.New(
        loza.Test("test").with_sink(sink).with_sampler(loza.SampleNone())  # 0% sample rate
    )

    ctx = logger.start_event(loza.Params(event="sampled_out"))
    logger.finish(ctx, "success")

    payload = logger.emit(ctx)
    # Sampled-out event should return empty string
    assert payload == "", "Sampled-out events should not be emitted"
    # Sink should not receive sampled-out events
    assert len(sink.events) == 0, "Sampled-out events should not reach sink"


def test_error_events_recorded_in_sink() -> None:
    """Verify error events are recorded properly."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    ctx = logger.start_event(loza.Params(event="error.test"))
    logger.finish(ctx, "error", error="TestException")

    payload = logger.emit(ctx)
    assert payload, "Error event should be emitted"

    # Parse emitted payload
    emitted = json.loads(payload)
    assert emitted.get("outcome") == "error", "Error event should have outcome=error"

    # Verify sink has the error event
    assert len(sink.events) == 1, "Error event should be in sink"
    sink_event = json.loads(sink.events[0])
    assert sink_event["event_id"] == emitted["event_id"]


def test_success_vs_error_events_both_delivered() -> None:
    """Verify both success and error events are delivered."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    # Success event
    ctx1 = logger.start_event(loza.Params(event="success_event"))
    logger.finish(ctx1, "success")
    payload1 = logger.emit(ctx1)

    # Error event
    ctx2 = logger.start_event(loza.Params(event="error_event"))
    logger.finish(ctx2, "error")
    payload2 = logger.emit(ctx2)

    # Both should be emitted
    assert payload1 and payload2, "Both events should emit"

    # Both should be in sink
    assert len(sink.events) == 2, f"Expected 2 events in sink, got {len(sink.events)}"

    # Verify outcomes
    event1 = json.loads(payload1)
    event2 = json.loads(payload2)
    assert event1["outcome"] == "success"
    assert event2["outcome"] == "error"


def test_events_ordered_in_sink() -> None:
    """Verify events maintain order in sink."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    event_ids = []
    for i in range(3):
        ctx = logger.start_event(loza.Params(event=f"order_test_{i}"))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        event_ids.append(json.loads(payload)["event_id"])

    # Sink events are stored as JSON strings, parse them
    sink_event_ids = [json.loads(e)["event_id"] for e in sink.events]
    assert sink_event_ids == event_ids, "Sink events should match emission order"


def test_idempotent_emit_returns_same_payload() -> None:
    """Verify emit() is idempotent."""
    logger = loza.New(loza.Test("test"))

    ctx = logger.start_event(loza.Params(event="idempotent.test"))
    logger.finish(ctx, "success")

    # Emit multiple times
    payload1 = logger.emit(ctx)
    payload2 = logger.emit(ctx)
    payload3 = logger.emit(ctx)

    # All should return the same payload
    assert payload1 == payload2 == payload3, "Repeated emit() calls should return same payload"

    # Parse and verify
    event1 = json.loads(payload1)
    event2 = json.loads(payload2)
    assert event1["event_id"] == event2["event_id"]


def test_delivery_preserves_event_integrity() -> None:
    """Verify event data is not corrupted during delivery."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("checkout").with_sink(sink))

    ctx = logger.start_event(
        loza.Params(event="purchase.completed", request_id="req_123", trace_id="trace_456", user_id="user_789")
    )
    logger.enrich(ctx, loza.String("order_id", "ord_001"))
    logger.enrich(ctx, loza.Int("amount_cents", 9999))
    logger.finish(ctx, "success")

    payload = logger.emit(ctx)
    event = json.loads(payload)

    # Verify all data preserved
    assert event["service"] == "checkout"
    assert event["event"] == "purchase.completed"
    assert event["request_id"] == "req_123"
    assert event["trace_id"] == "trace_456"
    assert event["attrs"]["order_id"] == "ord_001"
    assert event["attrs"]["amount_cents"] == 9999


def test_large_event_still_delivered() -> None:
    """Verify large events are delivered correctly."""
    sink = loza.MemorySink()
    logger = loza.New(loza.Test("test").with_sink(sink))

    ctx = logger.start_event(loza.Params(event="large.test"))

    # Add lots of attributes
    for i in range(100):
        logger.enrich(ctx, loza.String(f"attr_{i}", f"value_{i}" * 10))

    logger.finish(ctx, "success")
    payload = logger.emit(ctx)

    assert payload, "Large event should be emitted"
    event = json.loads(payload)
    assert len(event.get("attrs", {})) >= 100, "Large event should preserve all attributes"


def test_concurrent_loggers_dont_interfere() -> None:
    """Verify multiple loggers don't interfere."""
    sink1 = loza.MemorySink()
    sink2 = loza.MemorySink()

    logger1 = loza.New(loza.Test("service1").with_sink(sink1))
    logger2 = loza.New(loza.Test("service2").with_sink(sink2))

    ctx1 = logger1.start_event(loza.Params(event="service1.event"))
    ctx2 = logger2.start_event(loza.Params(event="service2.event"))

    logger1.finish(ctx1, "success")
    logger2.finish(ctx2, "success")

    payload1 = logger1.emit(ctx1)
    payload2 = logger2.emit(ctx2)

    # Each sink should have exactly one event
    assert len(sink1.events) == 1
    assert len(sink2.events) == 1

    # Payloads should be different
    assert payload1 != payload2
    assert json.loads(payload1)["service"] == "service1"
    assert json.loads(payload2)["service"] == "service2"

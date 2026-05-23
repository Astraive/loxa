"""
Test that canonical fields are protected and cannot be overridden.

Canonical fields are core LOXA event fields that should never be overridden
by user code. These include: schema_version, event_version, timestamp,
event_id, service, event, outcome, duration_ms, level, and trace context.
"""

from __future__ import annotations

import json


import loxa


def test_schema_version_is_protected() -> None:
    """Verify schema_version cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["schema_version"] == "v1"

    # Attempt to override - should be ignored or stored in attrs, not replace canonical
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(event="test.event"))
    logger2.enrich(ctx2, loxa.String("schema_version", "v999"))
    logger2.finish(ctx2, "success")

    payload2 = json.loads(logger2.emit(ctx2))
    # Canonical should remain v1, not override to v999
    assert payload2["schema_version"] == "v1", \
        "schema_version should be protected, not overridable by user"


def test_event_version_is_protected() -> None:
    """Verify event_version cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["event_version"] == "v1"

    # Attempt to override - should be ignored
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(event="test.event"))
    logger2.enrich(ctx2, loxa.String("event_version", "v999"))
    logger2.finish(ctx2, "success")

    payload2 = json.loads(logger2.emit(ctx2))
    assert payload2["event_version"] == "v1", \
        "event_version should be protected, not overridable by user"


def test_event_id_is_protected() -> None:
    """Verify event_id cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    original_id = ctx.event_id

    # Attempt to override - should be ignored
    logger.enrich(ctx, loxa.String("event_id", "user_attempted_override"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["event_id"] == original_id, \
        "event_id should be protected, not overridable by user"
    assert payload["event_id"] != "user_attempted_override"


def test_timestamp_is_protected() -> None:
    """Verify timestamp cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    payload["timestamp"]

    # Attempt to override - should be ignored
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(event="test.event"))
    logger2.enrich(ctx2, loxa.String("timestamp", "1970-01-01T00:00:00Z"))
    logger2.finish(ctx2, "success")

    payload2 = json.loads(logger2.emit(ctx2))
    assert payload2["timestamp"] != "1970-01-01T00:00:00Z", \
        "timestamp should be protected, not overridable by user"


def test_service_is_protected() -> None:
    """Verify service name cannot be overridden by user."""
    logger = loxa.New(loxa.Test("my-service"))
    ctx = logger.start_event(loxa.Params(event="test.event"))

    # Attempt to override - should be ignored
    logger.enrich(ctx, loxa.String("service", "attacker-service"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["service"] == "my-service", \
        "service should be protected, not overridable by user"
    assert payload["service"] != "attacker-service"


def test_event_name_is_protected() -> None:
    """Verify event name cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="checkout.completed"))

    # Attempt to override - should be ignored
    logger.enrich(ctx, loxa.String("event", "fake.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["event"] == "checkout.completed", \
        "event name should be protected, not overridable by user"
    assert payload["event"] != "fake.event"


def test_duration_ms_is_protected() -> None:
    """Verify duration_ms is calculated and not overridable."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    original_duration = payload["duration_ms"]
    assert isinstance(original_duration, (int, float))
    assert original_duration >= 0

    # Attempt to override - should be ignored (based on actual timing, not user input)
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(event="test.event"))
    logger2.enrich(ctx2, loxa.String("duration_ms", "9999999"))
    logger2.finish(ctx2, "success")

    payload2 = json.loads(logger2.emit(ctx2))
    # Should use actual measured duration, not user-provided value
    assert payload2["duration_ms"] != 9999999, \
        "duration_ms should be calculated, not overridable by user"


def test_outcome_is_protected() -> None:
    """Verify outcome cannot be overridden by user."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test.event"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))
    assert payload["outcome"] == "success"

    # Attempt to override outcome by enriching before finish
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(event="test.event"))
    logger2.enrich(ctx2, loxa.String("outcome", "fake_outcome"))
    logger2.finish(ctx2, "error")  # Finish with error

    payload2 = json.loads(logger2.emit(ctx2))
    # Outcome should be "error" (set by finish), not "fake_outcome"
    assert payload2["outcome"] == "error", \
        "outcome should be determined by finish() call, not user enrichment"


def test_trace_context_is_protected() -> None:
    """Verify trace context fields are protected if set."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(
        event="test.event",
        trace_id="trace_123",
        span_id="span_456",
        request_id="req_789"
    ))

    logger.finish(ctx, "success")
    payload = json.loads(logger.emit(ctx))

    # Verify original values are present
    assert payload.get("trace_id") == "trace_123"
    assert payload.get("span_id") == "span_456"
    assert payload.get("request_id") == "req_789"

    # Attempt to override trace context - enriched values should not override
    logger2 = loxa.New(loxa.Test("test"))
    ctx2 = logger2.start_event(loxa.Params(
        event="test.event",
        trace_id="trace_123",
        span_id="span_456",
        request_id="req_789"
    ))

    logger2.enrich(
        ctx2,
        loxa.String("trace_id", "fake_trace"),
        loxa.String("span_id", "fake_span"),
        loxa.String("request_id", "fake_request")
    )
    logger2.finish(ctx2, "success")

    payload2 = json.loads(logger2.emit(ctx2))
    # Original values should be preserved
    assert payload2.get("trace_id") == "trace_123", \
        "trace_id should be protected from user override"
    assert payload2.get("span_id") == "span_456", \
        "span_id should be protected from user override"
    assert payload2.get("request_id") == "req_789", \
        "request_id should be protected from user override"


def test_canonical_fields_with_all_policies() -> None:
    """Verify canonical protection works with all duplicate policies."""
    for policy in [loxa.CanonicalWins, loxa.UserWins, loxa.KeepBoth]:
        logger = loxa.New(
            loxa.Test("test").with_duplicate_policy(policy)
        )
        ctx = logger.start_event(loxa.Params(event="policy_test"))
        original_id = ctx.event_id

        # Attempt to override with duplicate
        logger.enrich(ctx, loxa.String("event_id", "override_1"))
        logger.enrich(ctx, loxa.String("event_id", "override_2"))
        logger.finish(ctx, "success")

        payload = json.loads(logger.emit(ctx))
        assert payload["event_id"] == original_id, \
            f"event_id should be protected even with {policy.__name__} policy"

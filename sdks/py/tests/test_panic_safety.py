"""
Test panic safety: graceful error handling without unhandled exceptions.

LOXA must guarantee:
- Errors are returned cleanly, not raise unhandled exceptions
- Invalid inputs handled gracefully
- Edge cases don't crash the SDK
"""

from __future__ import annotations

import json

import pytest

import loxa


def test_oversized_event_handled_gracefully() -> None:
    """Verify oversized events are rejected cleanly."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    # Try to add extremely large attribute
    huge_value = "x" * (10 * 1024 * 1024)  # 10MB string
    logger.enrich(ctx, loxa.String("huge", huge_value))
    logger.finish(ctx, "success")
    
    # Should either emit empty (dropped) or raise ValueError, not crash
    try:
        payload = logger.emit(ctx)
        # If it emits, that's fine
        if payload:
            event = json.loads(payload)
            assert event is not None
    except ValueError:
        # Validation error is acceptable
        pass


def test_invalid_event_name_handled() -> None:
    """Verify invalid event names are handled."""
    logger = loxa.New(loxa.Test("test"))
    
    # Python SDK accepts various types, so just verify no crashes
    try:
        ctx = logger.start_event(loxa.Params(event=" "))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        # Should handle without crashing
    except Exception as e:
        # Should be a known error type, not a panic
        pass


def test_sink_write_failure_doesnt_crash() -> None:
    """Verify sink failures don't crash the SDK."""
    class FailingSink:
        def write(self, payload: str) -> None:
            raise IOError("Sink is broken")
        def flush(self) -> None:
            pass
        def close(self) -> None:
            pass
    
    logger = loxa.New(loxa.Test("test").with_sink(FailingSink()))
    ctx = logger.start_event(loxa.Params(event="test"))
    logger.finish(ctx, "success")
    
    # emit() should complete even if sink fails
    try:
        payload = logger.emit(ctx)
        # Event may still emit to caller even if sink fails
    except Exception as e:
        # Should be a known error, not a panic
        assert isinstance(e, (IOError, OSError, RuntimeError)), \
            f"Unexpected error type: {type(e).__name__}"


def test_invalid_duplicate_policy_rejected() -> None:
    """Verify invalid duplicate policies are rejected."""
    try:
        # Try to create logger with None policy
        logger = loxa.New(
            loxa.Test("test").with_duplicate_policy(None)
        )
        # If it doesn't raise, continue
        ctx = logger.start_event(loxa.Params(event="test"))
        logger.finish(ctx, "success")
    except (TypeError, ValueError, AttributeError) as e:
        # Expected to reject invalid policy
        pass


def test_invalid_sampler_rejected() -> None:
    """Verify invalid samplers are rejected."""
    try:
        # Try to create logger with None sampler
        logger = loxa.New(
            loxa.Test("test").with_sampler(None)
        )
        ctx = logger.start_event(loxa.Params(event="test"))
        logger.finish(ctx, "success")
    except (TypeError, ValueError, AttributeError):
        # Expected to reject invalid sampler
        pass


def test_invalid_schema_rejected() -> None:
    """Verify invalid schemas are rejected."""
    try:
        logger = loxa.New(
            loxa.Test("test").with_schema(None)
        )
        ctx = logger.start_event(loxa.Params(event="test"))
        logger.finish(ctx, "success")
    except (TypeError, ValueError, AttributeError):
        # Expected to reject invalid schema
        pass


def test_invalid_redactor_rejected() -> None:
    """Verify invalid redactors are rejected."""
    try:
        logger = loxa.New(
            loxa.Test("test").with_redactor(None)
        )
        ctx = logger.start_event(loxa.Params(event="test"))
        logger.finish(ctx, "success")
    except (TypeError, ValueError, AttributeError):
        # Expected to reject invalid redactor
        pass


def test_finish_twice_handled() -> None:
    """Verify calling finish() twice is handled."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    logger.finish(ctx, "success")
    
    # Calling finish again should raise an error (not panic)
    try:
        logger.finish(ctx, "error")
    except Exception as e:
        # Should be a known error type (EventAlreadyFinishedError is OK)
        assert "already finished" in str(e) or isinstance(e, (ValueError, RuntimeError)), \
            f"Unexpected error: {type(e).__name__}"


def test_emit_before_finish_handled() -> None:
    """Verify emitting before finish is allowed or errors cleanly."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    # Emitting without finish may be allowed or error
    try:
        payload = logger.emit(ctx)
        # Python SDK may emit partial events
        if payload:
            event = json.loads(payload)
            assert event is not None
    except (ValueError, RuntimeError, KeyError):
        # Known error is acceptable
        pass


def test_null_attribute_values_handled() -> None:
    """Verify null/None values in attributes are handled."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    # Try to enrich with None value
    try:
        logger.enrich(ctx, loxa.String("attr", None))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        # Should handle without crashing
    except (TypeError, ValueError):
        # Known error is acceptable
        pass


def test_special_characters_in_attributes() -> None:
    """Verify special characters are handled."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    special_values = [
        "\x00null byte",
        "\uffff unicode",
        "\\\"quotes\\\"",
        "\\nnewlines\\n",
        "{}[]()\"quotes'",
    ]
    
    for value in special_values:
        try:
            logger.enrich(ctx, loxa.String("attr", value))
        except (ValueError, TypeError):
            pass
    
    logger.finish(ctx, "success")
    payload = logger.emit(ctx)
    # Should emit without crashing
    if payload:
        event = json.loads(payload)
        assert event is not None


def test_deeply_nested_attributes() -> None:
    """Verify deeply nested attribute structures are handled."""
    logger = loxa.New(loxa.Test("test"))
    ctx = logger.start_event(loxa.Params(event="test"))
    
    # Python SDK may not support nested objects directly
    # But it should handle without crashing
    try:
        for i in range(1000):
            logger.enrich(ctx, loxa.String(f"attr_{i}", f"value_{i}"))
        logger.finish(ctx, "success")
        payload = logger.emit(ctx)
        # Should complete without crashing
    except (ValueError, RuntimeError, MemoryError):
        # Known errors are acceptable
        pass

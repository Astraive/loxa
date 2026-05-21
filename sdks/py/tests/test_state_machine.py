from __future__ import annotations

import pytest

import loxa


def test_duplicate_emit_is_idempotent_and_finish_after_emit_is_typed() -> None:
    logger = loxa.New(loxa.Test("checkout").with_sink(loxa.NoopSink()))
    ctx = logger.start_event(loxa.Params(event="state.machine"))
    logger.finish(ctx, "success")
    first = logger.emit(ctx)
    second = logger.emit(ctx)

    assert first == second
    with pytest.raises(loxa.EventClosedError):
        logger.finish(ctx, "success")


def test_validation_failure_does_not_mark_emitted() -> None:
    logger = loxa.New(loxa.Test("checkout"))
    ctx = logger.start_event(loxa.Params(event="bad", kind="bad-kind"))

    with pytest.raises(loxa.EventValidationError):
        logger.emit(ctx)

    assert not ctx.is_emitted()
    assert ctx.event_state == "failed_validation"


def test_created_transitions_to_active_on_enrich() -> None:
    logger = loxa.New(loxa.Test("checkout").with_sink(loxa.NoopSink()))
    ctx = logger.start_event(loxa.Params(event="state.transition"))

    assert ctx.event_state == "created"
    logger.enrich(ctx, loxa.String("user.id", "u1"))
    assert ctx.event_state == "active"

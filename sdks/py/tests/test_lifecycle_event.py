from __future__ import annotations

import json

import loza


def test_lifecycle_event_helpers():
    logger, sink = loza.TestLogger()
    ctx = logger.start_event(loza.Params(event="checkout.request"))
    logger.enrich(ctx, user_id="u_123", tenant_id="t_123")
    logger.enrich(ctx, payment_provider="stripe")
    logger.checkpoint(ctx, "validated")

    cloned = logger.clone_event(ctx)
    assert cloned.event_id == ctx.event_id
    assert cloned.params.event == "checkout.request"

    parent = logger.start_event(loza.Params(event="parent", trace_id="trace1"))
    child = logger.start_event(loza.Params(event="child", trace_id="trace2"))
    logger.link_event(parent, child)
    assert parent.params.links == [child.event_id]

    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert payload["outcome"] == "success"


def test_lifecycle_outcome_helpers():
    logger, _ = loza.TestLogger()

    ctx = logger.start_event(loza.Params(event="test.drop"))
    logger.drop(ctx)
    assert ctx.outcome == "abandoned"
    assert ctx.emitted

    ctx = logger.start_event(loza.Params(event="test.cancel"))
    logger.cancel(ctx)
    assert ctx.outcome == "cancelled"

    ctx = logger.start_event(loza.Params(event="test.abandon"))
    logger.abandon(ctx)
    assert ctx.outcome == "abandoned"

    ctx = logger.start_event(loza.Params(event="test.retry"))
    logger.retry(ctx)
    assert ctx.outcome == "retried"

    ctx = logger.start_event(loza.Params(event="test.partial"))
    logger.partial(ctx, reason="timeout")
    assert ctx.partial
    assert ctx.partial_reason == "timeout"


def test_lifecycle_wrap_helpers():
    logger, sink = loza.TestLogger()
    ctx = logger.start_event(loza.Params(event="test.wrap"))
    result = logger.wrap(ctx, lambda x: x + 1, 41)
    assert result == 42
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert payload["outcome"] == "success"

    ctx = logger.start_event(loza.Params(event="test.wrap.fail"))
    try:
        logger.wrap(ctx, lambda: 1 / 0)
    except ZeroDivisionError:
        pass
    logger.emit(ctx)
    payload = json.loads(sink.events[-1])
    assert payload["outcome"] == "error"

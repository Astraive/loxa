from __future__ import annotations

import json
from datetime import timedelta

import pytest

import loxa


def test_schema_redaction_and_flattening() -> None:
    sink = loxa.MemorySink()
    cfg = (
        loxa.Test("checkout")
        .with_sink(sink)
        .with_schema(loxa.FlatSchema())
        .with_redactor(loxa.RedactKeys("password"))
    )
    logger = loxa.New(cfg)
    ctx = logger.start_event(loxa.Params(event="checkout.run"))
    logger.enrich(ctx, loxa.String("user.id", "u1"), loxa.String("password", "secret"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))

    assert payload["user_id"] == "u1"
    assert payload["attrs_password"] == "[REDACTED]"
    assert sink.events


def test_sampler_can_drop_without_sink_write() -> None:
    sink = loxa.MemorySink()
    logger = loxa.New(loxa.Test("checkout").with_sink(sink).with_sampler(loxa.SampleNone()))
    ctx = logger.start_event(loxa.Params(event="dropped"))
    logger.finish(ctx, "success")

    assert logger.emit(ctx) == ""
    assert sink.events == []


def test_duplicate_policy_error_on_duplicate() -> None:
    logger = loxa.New(loxa.Test("checkout").with_duplicate_policy(loxa.ErrorOnDuplicate))
    ctx = logger.start_event(loxa.Params(event="dupe"))
    logger.enrich(ctx, loxa.String("cart.id", "a"))

    with pytest.raises(ValueError):
        logger.enrich(ctx, loxa.String("cart.id", "b"))


def test_custom_schema_gets_event_view() -> None:
    sink = loxa.MemorySink()
    logger = loxa.New(
        loxa.Test("checkout")
        .with_sink(sink)
        .with_schema(loxa.CustomSchema(lambda ev: {"id": ev.event_id, "took": ev.duration_ms()}))
    )
    ctx = logger.start_event(loxa.Params(event="custom"))
    logger.finish(ctx, "success")

    payload = json.loads(logger.emit(ctx))

    assert payload["id"] == ctx.event_id
    assert payload["took"] >= 0


def test_sampler_combinators_and_error_sampler() -> None:
    logger = loxa.New(
        loxa.Test("checkout")
        .with_sink(loxa.MemorySink())
        .with_sampler(loxa.AnySampler(loxa.SampleErrors(), loxa.SampleSlowRequests(timedelta(seconds=100))))
    )
    ctx = logger.start_event(loxa.Params(event="failed"))
    logger.finish_error(ctx, RuntimeError("boom"))

    assert json.loads(logger.emit(ctx))["outcome"] == "error"


def test_checkpoint_emit_immediately_writes_checkpoint_event() -> None:
    sink = loxa.MemorySink()
    cfg = loxa.Test("checkout").with_sink(sink)
    cfg.checkpoint_emit_immediately = True
    logger = loxa.New(cfg)
    ctx = logger.start_event(loxa.Params(event="checkout.run"))

    loxa.Checkpoint(ctx, "db.started", loxa.String("phase", "db"))
    logger.finish(ctx, "success")
    logger.emit(ctx)

    assert len(sink.events) == 2
    checkpoint_payload = json.loads(sink.events[0])
    final_payload = json.loads(sink.events[1])
    assert checkpoint_payload["event"] == "checkpoint.db.started"
    assert checkpoint_payload["kind"] == "checkpoint"
    assert checkpoint_payload["attrs"]["phase"] == "db"
    assert checkpoint_payload["request_id"] == final_payload["request_id"]

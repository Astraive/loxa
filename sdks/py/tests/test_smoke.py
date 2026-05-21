from __future__ import annotations

import json

from loxa import Config, LOXA_EVENT_VERSION, LOXA_SPEC_VERSION, Logger, MemorySink, Params


def test_emit_smoke() -> None:
    sink = MemorySink()
    logger = Logger(Config(service="test", environment="test", level="debug", strict=True, sinks=[sink]))

    ctx = logger.start_event(Params(event="test.run", kind="cli", message="hello"))
    logger.enrich(ctx, answer=42)
    logger.merge(ctx, "user", id="user_001")
    logger.merge(ctx, "tenant", id="tenant_001")
    logger.finish(ctx, "success")
    encoded = logger.emit(ctx)

    payload = json.loads(encoded)
    assert payload["schema_version"] == LOXA_SPEC_VERSION
    assert payload["event_version"] == LOXA_EVENT_VERSION
    assert payload["service"] == "test"
    assert payload["event"] == "test.run"
    assert payload["kind"] == "cli"
    assert payload["user"]["id"] == "user_001"
    assert payload["tenant"]["id"] == "tenant_001"
    assert sink.events


def test_finish_error_sets_error_payload() -> None:
    sink = MemorySink()
    logger = Logger(Config(service="test", environment="test", level="debug", strict=True, sinks=[sink]))
    ctx = logger.start_event(Params(event="test.error"))
    logger.finish_error(ctx, ValueError("boom"))
    payload = json.loads(logger.emit(ctx))
    assert payload["outcome"] == "error"
    assert payload["error"]["message"] == "boom"

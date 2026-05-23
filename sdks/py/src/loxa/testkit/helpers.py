from __future__ import annotations

import json
from collections.abc import Callable
from contextlib import contextmanager
from datetime import datetime
from typing import Any

from .. import Config, Logger, MemorySink


def TestLogger(service: str = "test"):
    sink = MemorySink()
    return Logger(Config.test(service).with_sink(sink)), sink


def Capture(fn: Callable[[Logger], Any]):
    logger, sink = TestLogger()
    fn(logger)
    return [json.loads(e) for e in sink.events]


def AssertEvent(payload, **fields):
    for key, value in fields.items():
        actual = _get_path(payload, key)
        assert actual == value, f"{key}: expected {value!r}, got {actual!r}"


def AssertRedacted(payload: dict[str, Any], *keys: str) -> None:
    for key in keys:
        assert _get_path(payload, key) == "[REDACTED]"


def AssertHasCheckpoint(payload: dict[str, Any], name: str) -> None:
    checkpoints = payload.get("checkpoints", [])
    assert any(item.get("name") == name for item in checkpoints), f"missing checkpoint {name}"


def DecodeEvents(sink) -> list[dict[str, Any]]:
    return [json.loads(event) for event in sink.events]


@contextmanager
def CapturingLogger(service: str = "test"):
    logger, sink = TestLogger(service)
    try:
        yield logger, sink
    finally:
        logger.shutdown()


def expect_event(captured: list[dict[str, Any]], **fields: Any) -> dict[str, Any]:
    for event in captured:
        match = True
        for key, value in fields.items():
            if _get_path(event, key) != value:
                match = False
                break
        if match:
            return event
    raise AssertionError(f"no event matching {fields} in captured events")


def expect_attr(event: dict[str, Any], key: str, expected: Any) -> None:
    actual = _get_path(event, key)
    assert actual == expected, f"attr {key}: expected {expected!r}, got {actual!r}"


def snapshot_event(event: dict[str, Any]) -> dict[str, Any]:
    snap = {}
    for k, v in event.items():
        if k not in ("event_id", "timestamp", "request_id", "trace_id", "span_id", "duration_ms"):
            snap[k] = v
    return snap


def mock_sink() -> MemorySink:
    return MemorySink()


_fake_now: datetime | None = None


def fake_clock(value: datetime | None = None) -> datetime:
    global _fake_now
    if value is not None:
        _fake_now = value
    if _fake_now is not None:
        return _fake_now
    return datetime.utcnow()


def set_id_generator(fn: Callable[[], str] | None = None) -> None:
    from ..core.uuidv7 import uuidv7_like
    global _id_gen
    _id_gen = fn or (lambda: uuidv7_like("evt"))


_id_gen: Callable[[], str] | None = None


def _get_path(payload: dict[str, Any], key: str) -> Any:
    current: Any = payload
    for part in key.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current

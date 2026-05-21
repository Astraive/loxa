from __future__ import annotations

import json
from collections.abc import Callable
from contextlib import contextmanager
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


def _get_path(payload: dict[str, Any], key: str) -> Any:
    current: Any = payload
    for part in key.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current

from __future__ import annotations

import json
from pathlib import Path
from collections.abc import Callable
from contextlib import contextmanager
from datetime import datetime
from typing import Any

from collections.abc import Callable
from ..core.config import Config
from ..core.logger import Logger
from ..sinks import MemorySink


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
_captured_events: list[dict[str, Any]] = []


def testkit(service: str = "test") -> dict[str, Any]:
    logger, sink = TestLogger(service)
    return {
        "logger": logger,
        "sink": sink,
        "capture": Capture,
        "events": events,
        "last_event": last_event,
        "clear_events": clear_events,
    }


def events() -> list[dict[str, Any]]:
    return list(_captured_events)


def last_event() -> dict[str, Any] | None:
    return _captured_events[-1] if _captured_events else None


def clear_events() -> None:
    _captured_events.clear()


def golden_test(path: str, event: dict[str, Any]) -> bool:
    snapshot = json.dumps(snapshot_event(event), sort_keys=True, separators=(",", ":"))
    target = Path(path)
    if not target.exists():
        target.write_text(snapshot + "\n", encoding="utf-8")
        return True
    return json.dumps(json.loads(target.read_text(encoding="utf-8")), sort_keys=True, separators=(",", ":")) == snapshot


def conformance_suite() -> dict[str, str]:
    return {"name": "loxa-py-conformance", "status": "available"}


def fake_clock(value: datetime | None = None) -> datetime:
    global _fake_now
    if value is not None:
        _fake_now = value
    if _fake_now is not None:
        return _fake_now
    return datetime.utcnow()


def set_id_generator(fn: Callable[[], str] | None = None) -> None:
    from ..core.uuidv7 import set_id_generator as _set_core_id_generator
    if fn is None:
        _set_core_id_generator(None)
        return
    _set_core_id_generator(lambda prefix: fn())


def reset_for_test() -> None:
    """Clear all global mutable state: default logger, clock, and ID generator."""
    clear_events()
    global _fake_now
    _fake_now = None
    from ..core.uuidv7 import reset_id_generator as _reset_core_id_generator
    _reset_core_id_generator()
    from .. import _reset_default
    _reset_default()


def set_clock(value: datetime | None = None) -> datetime:
    return fake_clock(value)


def _get_path(payload: dict[str, Any], key: str) -> Any:
    current: Any = payload
    for part in key.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current

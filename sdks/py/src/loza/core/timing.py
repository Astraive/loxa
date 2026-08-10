"""Timing primitives: Process, Timer, Group, Stopwatch."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from .event import Attr


@dataclass(slots=True)
class EventProcess:
    step: int
    name: str
    started_at_ms: int
    ended_at_ms: int
    duration_ms: int
    status_code: int = 0
    attrs: dict[str, object] = field(default_factory=dict)

    def to_dict(self) -> dict[str, object]:
        d: dict[str, object] = {
            "step": self.step,
            "name": self.name,
            "started_at_ms": self.started_at_ms,
            "ended_at_ms": self.ended_at_ms,
            "duration_ms": self.duration_ms,
        }
        if self.status_code:
            d["status_code"] = self.status_code
        d.update(self.attrs)
        return d


@dataclass(slots=True)
class EventGroup:
    name: str
    started_at_ms: int
    ended_at_ms: int
    duration_ms: int
    status_code: int = 0
    attrs: dict[str, object] = field(default_factory=dict)

    def to_dict(self) -> dict[str, object]:
        d: dict[str, object] = {
            "name": self.name,
            "started_at_ms": self.started_at_ms,
            "ended_at_ms": self.ended_at_ms,
            "duration_ms": self.duration_ms,
        }
        if self.status_code:
            d["status_code"] = self.status_code
        d.update(self.attrs)
        return d


@dataclass(slots=True)
class EventTimer:
    name: str
    duration_ms: int
    status_code: int = 0
    attrs: dict[str, object] = field(default_factory=dict)

    def to_dict(self) -> dict[str, object]:
        d: dict[str, object] = {
            "name": self.name,
            "duration_ms": self.duration_ms,
        }
        if self.status_code:
            d["status_code"] = self.status_code
        d.update(self.attrs)
        return d


def _extract_status_code(attrs: dict[str, object]) -> tuple[int, dict[str, object]]:
    """Extract status_code from attrs if present, return (code, remaining_attrs)."""
    code = 0
    remaining = {}
    for k, v in attrs.items():
        if k == "status_code" and isinstance(v, int):
            code = v
        else:
            remaining[k] = v
    return code, remaining


def _now_ms(started_at: datetime) -> int:
    """Milliseconds since started_at."""
    return int((datetime.now(timezone.utc) - started_at).total_seconds() * 1000)


class ProcessHandle:
    """Tracks a named process step with automatic duration."""

    __slots__ = ("_event", "_name", "_step", "_started_at", "_started_at_ms")

    def __init__(self, event: Any, name: str, step: int, started_at: datetime):
        self._event = event
        self._name = name
        self._step = step
        self._started_at = started_at
        self._started_at_ms = _now_ms(event.started_at)

    def finish(self, **attrs: object) -> None:
        now = datetime.now(timezone.utc)
        ended_ms = int((now - self._event.started_at).total_seconds() * 1000)
        status_code, clean_attrs = _extract_status_code(attrs)
        entry = EventProcess(
            step=self._step,
            name=self._name,
            started_at_ms=self._started_at_ms,
            ended_at_ms=ended_ms,
            duration_ms=ended_ms - self._started_at_ms,
            status_code=status_code,
            attrs=clean_attrs,
        )
        self._event.processes.append(entry.to_dict())

    def finish_error(self, error: BaseException, **attrs: object) -> None:
        attrs["error_message"] = str(error)
        self.finish(**attrs)

    def duration(self) -> timedelta:
        return datetime.now(timezone.utc) - self._started_at


class TimerHandle:
    """Tracks a named duration measurement."""

    __slots__ = ("_event", "_name", "_started_at")

    def __init__(self, event: Any, name: str, started_at: datetime):
        self._event = event
        self._name = name
        self._started_at = started_at

    def stop(self, **attrs: object) -> None:
        duration_ms = int((datetime.now(timezone.utc) - self._started_at).total_seconds() * 1000)
        status_code, clean_attrs = _extract_status_code(attrs)
        entry = EventTimer(
            name=self._name,
            duration_ms=duration_ms,
            status_code=status_code,
            attrs=clean_attrs,
        )
        self._event.timers.append(entry.to_dict())

    def duration(self) -> timedelta:
        return datetime.now(timezone.utc) - self._started_at


class GroupHandle:
    """Tracks a parent phase containing processes."""

    __slots__ = ("_event", "_name", "_started_at", "_started_at_ms")

    def __init__(self, event: Any, name: str, started_at: datetime):
        self._event = event
        self._name = name
        self._started_at = started_at
        self._started_at_ms = _now_ms(event.started_at)

    def finish(self, **attrs: object) -> None:
        now = datetime.now(timezone.utc)
        ended_ms = int((now - self._event.started_at).total_seconds() * 1000)
        status_code, clean_attrs = _extract_status_code(attrs)
        entry = EventGroup(
            name=self._name,
            started_at_ms=self._started_at_ms,
            ended_at_ms=ended_ms,
            duration_ms=ended_ms - self._started_at_ms,
            status_code=status_code,
            attrs=clean_attrs,
        )
        self._event.groups.append(entry.to_dict())

    def duration(self) -> timedelta:
        return datetime.now(timezone.utc) - self._started_at


class StopwatchHandle:
    """Standalone elapsed-time measurer with no event reference."""

    __slots__ = ("_started_at",)

    def __init__(self) -> None:
        self._started_at = datetime.now(timezone.utc)

    def elapsed(self) -> timedelta:
        return datetime.now(timezone.utc) - self._started_at


# --- Timing helper methods (added to EventContext at runtime) ---


def with_process(ctx: Any, name: str, *attrs: Attr, **fields: Any) -> ProcessHandle:
    return ctx.start_process(name, **fields)


def with_group(ctx: Any, name: str, *attrs: Attr, **fields: Any) -> GroupHandle:
    return ctx.start_group(name, **fields)


def with_timer(ctx: Any, name: str, *attrs: Attr, **fields: Any) -> TimerHandle:
    return ctx.start_timer(name, **fields)


def finish_group_error(handle: GroupHandle, error: BaseException, **attrs: object) -> None:
    attrs["error_message"] = str(error)
    handle.finish(**attrs)


def measure(ctx: Any, name: str, fn: Callable[..., Any], *args: Any, **kwargs: Any) -> Any:
    timer = ctx.start_timer(name)
    try:
        return fn(*args, **kwargs)
    finally:
        timer.stop()


def step(ctx: Any, name: str) -> ProcessHandle:
    return ctx.start_process(name)


def phase(ctx: Any, name: str) -> GroupHandle:
    return ctx.start_group(name)


def span(ctx: Any, name: str) -> TimerHandle:
    return ctx.start_timer(name)

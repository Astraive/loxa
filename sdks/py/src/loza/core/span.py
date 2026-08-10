"""Span context for distributed tracing integration."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(slots=True)
class SpanContext:
    """Represents a distributed tracing span."""

    trace_id: str = ""
    span_id: str = ""
    parent_span_id: str = ""
    trace_flags: int = 0
    trace_state: dict[str, str] = field(default_factory=dict)
    is_remote: bool = False

    def is_valid(self) -> bool:
        return bool(self.trace_id) and bool(self.span_id)


def create_span_id() -> str:
    """Generate a new random span ID."""
    from uuid import uuid4

    return uuid4().hex[:16]


def create_trace_id() -> str:
    """Generate a new random trace ID."""
    from uuid import uuid4

    return uuid4().hex

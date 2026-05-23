from __future__ import annotations

from typing import Any

from .file import FileSink
from .httpbatch import HTTPBatchSink, NoopSink
from .memory import MemorySink
from .stdout import StdoutSink


class MultiSink:
    """Fan-out sink that writes to multiple sinks."""

    def __init__(self, *sinks: Any) -> None:
        self._sinks = list(sinks)

    def write(self, encoded: str) -> None:
        for sink in self._sinks:
            sink.write(encoded)

    def write_batch(self, encoded_events: list[str]) -> None:
        for sink in self._sinks:
            write_batch = getattr(sink, "write_batch", None)
            if callable(write_batch):
                write_batch(encoded_events)
            else:
                for event in encoded_events:
                    sink.write(event)

    def flush(self) -> None:
        for sink in self._sinks:
            flush_fn = getattr(sink, "flush", None)
            if callable(flush_fn):
                flush_fn()

    def close(self) -> None:
        for sink in self._sinks:
            close_fn = getattr(sink, "close", None)
            if callable(close_fn):
                close_fn()


def multi_sink(*sinks: Any) -> MultiSink:
    return MultiSink(*sinks)


def drain(sink: Any) -> None:
    flush_fn = getattr(sink, "flush", None)
    if callable(flush_fn):
        flush_fn()


def pause(sink: Any) -> None:
    pause_fn = getattr(sink, "pause", None)
    if callable(pause_fn):
        pause_fn()


def resume(sink: Any) -> None:
    resume_fn = getattr(sink, "resume", None)
    if callable(resume_fn):
        resume_fn()


def queue_size(sink: Any) -> int:
    qs = getattr(sink, "queue_size", None)
    if callable(qs):
        return qs()
    if isinstance(qs, int):
        return qs
    return 0


def health(sink: Any) -> bool:
    health_fn = getattr(sink, "health", None)
    if callable(health_fn):
        return health_fn()
    return True


def otlp_sink(endpoint: str = "", **kwargs: Any) -> NoopSink:
    return NoopSink()


# PascalCase aliases
MultiSinkFactory = multi_sink
Drain = drain
Pause = pause
Resume = resume
QueueSize = queue_size
Health = health
OTLPSink = otlp_sink

__all__ = [
    "FileSink", "HTTPBatchSink", "MemorySink", "NoopSink", "StdoutSink",
    "MultiSink", "multi_sink", "MultiSinkFactory",
    "drain", "Drain", "pause", "Pause", "resume", "Resume",
    "queue_size", "QueueSize", "health", "Health",
    "otlp_sink", "OTLPSink",
]

from __future__ import annotations

import json
import os
import time
from collections import deque
from dataclasses import dataclass
from pathlib import Path
from queue import Empty, Full, Queue
from threading import Event, Lock, Thread
from typing import Callable, Iterable, Protocol


class WritableSink(Protocol):
    def write(self, encoded: str) -> None:
        ...


class BatchWritableSink(Protocol):
    def write_batch(self, encoded_events: list[str]) -> None:
        ...


@dataclass(slots=True)
class DeliveryStats:
    enqueued: int = 0
    emitted: int = 0
    dropped: int = 0
    failed: int = 0
    retried: int = 0
    batches: int = 0
    last_error: str = ""

    def snapshot(self) -> dict[str, int | str]:
        return {
            "enqueued": self.enqueued,
            "emitted": self.emitted,
            "dropped": self.dropped,
            "failed": self.failed,
            "retried": self.retried,
            "batches": self.batches,
            "last_error": self.last_error,
        }


class ByteBatcher:
    """Byte-bounded batcher used by async delivery and collector transports."""

    def __init__(self, max_batch_bytes: int = 256 * 1024, max_events: int = 1024) -> None:
        self.max_batch_bytes = max(1, max_batch_bytes)
        self.max_events = max(1, max_events)
        self._events: list[str] = []
        self._bytes = 0

    def push(self, encoded: str) -> list[str] | None:
        size = len(encoded.encode("utf-8"))
        should_flush = bool(self._events) and (
            self._bytes + size > self.max_batch_bytes or len(self._events) >= self.max_events
        )
        if should_flush:
            flushed = self.drain()
            self._events.append(encoded)
            self._bytes = size
            return flushed
        self._events.append(encoded)
        self._bytes += size
        return None

    def drain(self) -> list[str]:
        events = self._events
        self._events = []
        self._bytes = 0
        return events

    @property
    def pending(self) -> int:
        return len(self._events)


class MemoryOfflineBuffer:
    """Bounded in-memory retry buffer for collector disconnects."""

    def __init__(self, max_events: int = 8192) -> None:
        self.max_events = max(1, max_events)
        self._events: deque[str] = deque()
        self._lock = Lock()
        self.dropped = 0

    def append(self, encoded: str) -> None:
        with self._lock:
            if len(self._events) >= self.max_events:
                self._events.popleft()
                self.dropped += 1
            self._events.append(encoded)

    def drain(self, limit: int = 1024) -> list[str]:
        with self._lock:
            events: list[str] = []
            while self._events and len(events) < limit:
                events.append(self._events.popleft())
            return events

    def __len__(self) -> int:
        with self._lock:
            return len(self._events)


class DiskOfflineBuffer:
    """Small disk spool for SDK-to-collector retry without becoming a collector."""

    def __init__(self, path: str | os.PathLike[str], max_bytes: int = 32 * 1024 * 1024) -> None:
        self.path = Path(path)
        self.max_bytes = max_bytes
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = Lock()
        self.dropped = 0

    def append(self, encoded: str) -> None:
        line = encoded.rstrip("\n") + "\n"
        with self._lock:
            if self.path.exists() and self.path.stat().st_size + len(line.encode("utf-8")) > self.max_bytes:
                self._truncate_half()
            with self.path.open("a", encoding="utf-8") as handle:
                handle.write(line)
                handle.flush()
                os.fsync(handle.fileno())

    def drain(self, limit: int = 1024) -> list[str]:
        with self._lock:
            if not self.path.exists():
                return []
            lines = self.path.read_text(encoding="utf-8").splitlines()
            head, tail = lines[:limit], lines[limit:]
            if tail:
                self.path.write_text("\n".join(tail) + "\n", encoding="utf-8")
            else:
                self.path.unlink(missing_ok=True)
            return [line for line in head if line.strip()]

    def _truncate_half(self) -> None:
        lines = self.path.read_text(encoding="utf-8").splitlines()
        kept = lines[len(lines) // 2 :]
        self.dropped += len(lines) - len(kept)
        self.path.write_text("\n".join(kept) + ("\n" if kept else ""), encoding="utf-8")


@dataclass(slots=True)
class RetryPolicy:
    max_attempts: int = 3
    base_delay: float = 0.05
    max_delay: float = 2.0

    def delay(self, attempt: int) -> float:
        return min(self.max_delay, self.base_delay * (2 ** max(0, attempt - 1)))


class Pipeline:
    def __init__(
        self,
        sinks: Iterable[object],
        queue_size: int = 8192,
        max_batch_bytes: int = 256 * 1024,
        retry_policy: RetryPolicy | None = None,
        offline_buffer: MemoryOfflineBuffer | DiskOfflineBuffer | None = None,
        error_handler: Callable[[Exception], None] | None = None,
        metrics: object | None = None,
    ) -> None:
        self.sinks = list(sinks)
        self.queue: Queue[str] = Queue(maxsize=queue_size)
        self.stop = Event()
        self.max_batch_bytes = max_batch_bytes
        self.retry_policy = retry_policy or RetryPolicy()
        self.offline_buffer = offline_buffer
        self.error_handler = error_handler
        self.stats = DeliveryStats()
        self._metrics = metrics
        self._workers: list[Thread] = []
        self._closed = False
        self._lock = Lock()

    def write_sync(self, encoded: str) -> None:
        self._write_batch([encoded])

    def try_enqueue(self, encoded: str) -> bool:
        try:
            self.queue.put_nowait(encoded)
            self.stats.enqueued += 1
            if self._metrics is not None:
                self._metrics.set_buffer_size(self.queue.qsize())
            return True
        except Full:
            self.stats.dropped += 1
            if self._metrics is not None:
                self._metrics.on_event_dropped("queue_full")
            if self.offline_buffer is not None:
                self.offline_buffer.append(encoded)
            return False

    def drain_once(self) -> int:
        batcher = ByteBatcher(self.max_batch_bytes)
        drained = 0
        while True:
            try:
                encoded = self.queue.get_nowait()
            except Empty:
                break
            flushed = batcher.push(encoded)
            if flushed:
                self._write_batch(flushed)
            drained += 1
        tail = batcher.drain()
        if tail:
            self._write_batch(tail)
        if self._metrics is not None:
            self._metrics.set_buffer_size(self.queue.qsize())
        return drained

    def start(self, workers: int = 1) -> None:
        with self._lock:
            if self._workers:
                return
            for idx in range(max(1, workers)):
                worker = Thread(target=self._run_worker, name=f"loxa-pipeline-{idx}", daemon=True)
                worker.start()
                self._workers.append(worker)

    def close(self, timeout: float = 30.0) -> None:
        self.stop.set()
        deadline = time.monotonic() + timeout
        for worker in self._workers:
            remaining = max(0.1, deadline - time.monotonic())
            worker.join(remaining)
        self.drain_once()
        if self.offline_buffer is not None:
            buffered = self.offline_buffer.drain(limit=4096)
            if buffered:
                self._write_batch(buffered)
        self._closed = True
        for sink in self.sinks:
            flush = getattr(sink, "flush", None)
            if callable(flush):
                flush()
            close_fn = getattr(sink, "close", None)
            if callable(close_fn):
                close_fn()

    def _run_worker(self) -> None:
        batcher = ByteBatcher(self.max_batch_bytes)
        while not self.stop.is_set() or not self.queue.empty():
            try:
                encoded = self.queue.get(timeout=0.05)
            except Empty:
                tail = batcher.drain()
                if tail:
                    self._write_batch(tail)
                continue
            flushed = batcher.push(encoded)
            if flushed:
                self._write_batch(flushed)
        tail = batcher.drain()
        if tail:
            self._write_batch(tail)

    def _write_batch(self, encoded_events: list[str]) -> None:
        if not encoded_events:
            return
        for sink in self.sinks:
            self._write_sink_with_retry(sink, encoded_events)
        self.stats.emitted += len(encoded_events)
        self.stats.batches += 1

    def _write_sink_with_retry(self, sink: object, encoded_events: list[str]) -> None:
        last_error: Exception | None = None
        for attempt in range(1, self.retry_policy.max_attempts + 1):
            try:
                write_batch = getattr(sink, "write_batch", None)
                if callable(write_batch):
                    write_batch(encoded_events)
                else:
                    for encoded in encoded_events:
                        sink.write(encoded)
                return
            except Exception as exc:
                last_error = exc
                self.stats.retried += int(attempt < self.retry_policy.max_attempts)
                if self._metrics is not None:
                    self._metrics.on_retry(attempt)
                time.sleep(self.retry_policy.delay(attempt))
        self.stats.failed += len(encoded_events)
        self.stats.last_error = str(last_error or "unknown sink failure")
        if self._metrics is not None:
            self._metrics.on_event_dropped("transport")
        if self.offline_buffer is not None:
            for encoded in encoded_events:
                self.offline_buffer.append(encoded)
        if self.error_handler is not None and last_error is not None:
            self.error_handler(last_error)


def encode_batch_envelope(encoded_events: Iterable[str]) -> str:
    events = []
    for encoded in encoded_events:
        try:
            events.append(json.loads(encoded))
        except json.JSONDecodeError:
            events.append({"malformed": encoded})
    service = "unknown"
    for event in events:
        if isinstance(event, dict):
            candidate = event.get("service")
            if isinstance(candidate, str) and candidate:
                service = candidate
                break
    return json.dumps(
        {
            "api_version": "v1",
            "source": {
                "sdk": "loxa-py",
                "version": "0.0.1",
                "service": service,
            },
            "events": events,
        },
        separators=(",", ":"),
    )

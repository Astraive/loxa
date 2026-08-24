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

from ..version import SDK_VERSION


class WritableSink(Protocol):
    def write(self, encoded: str) -> None: ...


class BatchWritableSink(Protocol):
    def write_batch(self, encoded_events: list[str]) -> None: ...


def _safe_buffer_path(path: str | os.PathLike[str]) -> Path:
    candidate = Path(path)
    if "\x00" in str(candidate):
        raise ValueError("offline buffer path contains a null byte")
    if any(part == ".." for part in candidate.parts):
        raise ValueError("offline buffer path must not contain parent-directory traversal")
    if candidate.exists() and candidate.is_dir():
        raise ValueError("offline buffer path must be a file")
    return candidate


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

    def peek(self, limit: int = 1024) -> list[str]:
        with self._lock:
            return list(self._events)[: max(0, limit)]

    def ack(self, count: int) -> None:
        with self._lock:
            for _ in range(min(max(0, count), len(self._events))):
                self._events.popleft()

    def __len__(self) -> int:
        with self._lock:
            return len(self._events)


class DiskOfflineBuffer:
    """Small disk spool for SDK-to-collector retry without becoming a collector."""

    def __init__(self, path: str | os.PathLike[str], max_bytes: int = 32 * 1024 * 1024) -> None:
        self.path = _safe_buffer_path(path)
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

    def peek(self, limit: int = 1024) -> list[str]:
        with self._lock:
            return self._read_lines()[: max(0, limit)]

    def ack(self, count: int) -> None:
        with self._lock:
            lines = self._read_lines()
            self._replace_lines(lines[min(max(0, count), len(lines)) :])

    def _read_lines(self) -> list[str]:
        if not self.path.exists():
            return []
        return [line for line in self.path.read_text(encoding="utf-8").splitlines() if line.strip()]

    def _replace_lines(self, lines: list[str]) -> None:
        if not lines:
            self.path.unlink(missing_ok=True)
            return
        temporary = self.path.with_name(f".{self.path.name}.tmp")
        with temporary.open("w", encoding="utf-8") as handle:
            handle.write("\n".join(lines) + "\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, self.path)

    def _truncate_half(self) -> None:
        lines = self._read_lines()
        kept = lines[len(lines) // 2 :]
        self.dropped += len(lines) - len(kept)
        self._replace_lines(kept)


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
        self._offline_lock = Lock()

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

    def replay_offline(self, limit: int = 4096) -> int:
        if self.offline_buffer is None:
            return 0
        delivered = 0
        with self._offline_lock:
            while buffered := self.offline_buffer.peek(limit=limit):
                if not self._write_batch(buffered, buffer_on_failure=False):
                    break
                self.offline_buffer.ack(len(buffered))
                delivered += len(buffered)
        return delivered

    def start(self, workers: int = 1) -> None:
        with self._lock:
            if self._workers:
                return
            for idx in range(max(1, workers)):
                worker = Thread(target=self._run_worker, name=f"loza-pipeline-{idx}", daemon=True)
                worker.start()
                self._workers.append(worker)

    def close(self, timeout: float = 30.0) -> None:
        with self._lock:
            if self._closed:
                return
            self.stop.set()
            deadline = time.monotonic() + max(0.0, timeout)
            for worker in self._workers:
                worker.join(max(0.0, deadline - time.monotonic()))
            live_workers = [worker.name for worker in self._workers if worker.is_alive()]
            if live_workers:
                names = ", ".join(live_workers)
                raise TimeoutError(f"pipeline workers did not stop before timeout: {names}")

            self.drain_once()
            self.replay_offline()

            for sink in self.sinks:
                flush = getattr(sink, "flush", None)
                if callable(flush):
                    flush()
                close_fn = getattr(sink, "close", None)
                if callable(close_fn):
                    close_fn()
            self._closed = True

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

    def _write_batch(self, encoded_events: list[str], *, buffer_on_failure: bool = True) -> bool:
        if not encoded_events:
            return True
        errors = [
            error for sink in self.sinks if (error := self._write_sink_with_retry(sink, encoded_events)) is not None
        ]
        if errors:
            self.stats.failed += len(encoded_events)
            self.stats.last_error = "; ".join(str(error) for error in errors)
            if self._metrics is not None:
                self._metrics.on_event_dropped("transport")
            if buffer_on_failure and self.offline_buffer is not None:
                for encoded in encoded_events:
                    self.offline_buffer.append(encoded)
            if self.error_handler is not None:
                for error in errors:
                    self.error_handler(error)
        else:
            self.stats.emitted += len(encoded_events)
        self.stats.batches += 1
        return not errors

    def _write_sink_with_retry(self, sink: object, encoded_events: list[str]) -> Exception | None:
        last_error: Exception | None = None
        for attempt in range(1, self.retry_policy.max_attempts + 1):
            try:
                write_batch = getattr(sink, "write_batch", None)
                if callable(write_batch):
                    write_batch(encoded_events)
                else:
                    for encoded in encoded_events:
                        sink.write(encoded)
                return None
            except Exception as exc:
                last_error = exc
                self.stats.retried += int(attempt < self.retry_policy.max_attempts)
                if self._metrics is not None:
                    self._metrics.on_retry(attempt)
                time.sleep(self.retry_policy.delay(attempt))
        return last_error or RuntimeError("unknown sink failure")


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
                "sdk": "loza-py",
                "version": SDK_VERSION,
                "service": service,
            },
            "events": events,
        },
        separators=(",", ":"),
    )

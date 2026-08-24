from __future__ import annotations
from threading import Event

import pytest

from loza.core.pipeline import DiskOfflineBuffer, Pipeline, RetryPolicy

import loza


def test_sink_queue_flush_shutdown_helpers():
    logger, sink = loza.TestLogger()
    logger.info("hello")
    logger.flush()
    assert len(loza.DecodeEvents(sink)) == 1

    ms = loza.multi_sink(loza.MemorySink(), loza.MemorySink())
    ms.write('{"test": true}')
    assert len(ms._sinks) == 2

    s = loza.MemorySink()
    loza.drain(s)
    loza.pause(s)
    loza.resume(s)
    assert loza.queue_size(s) == 0
    assert loza.health(s) is True
    assert loza.otlp_sink() is not None

    loza.flush()
    loza.shutdown()


def test_disk_offline_buffer_acknowledges_only_after_delivery(tmp_path):
    path = tmp_path / "offline.ndjson"
    buffer = DiskOfflineBuffer(path)
    buffer.append("one")
    buffer.append("two")

    assert buffer.peek(1) == ["one"]
    assert DiskOfflineBuffer(path).peek(2) == ["one", "two"]

    buffer.ack(1)
    assert DiskOfflineBuffer(path).peek(2) == ["two"]

class _ReplaySink:
    def __init__(self, failure: Exception | None = None) -> None:
        self.failure = failure
        self.events: list[str] = []

    def write_batch(self, events: list[str]) -> None:
        if self.failure is not None:
            raise self.failure
        self.events.extend(events)


def test_pipeline_acknowledges_successful_disk_replay(tmp_path):
    buffer = DiskOfflineBuffer(tmp_path / "successful.ndjson")
    buffer.append("one")
    sink = _ReplaySink()

    Pipeline([sink], offline_buffer=buffer).close()

    assert sink.events == ["one"]
    assert buffer.peek() == []


def test_pipeline_retains_failed_disk_replay(tmp_path):
    buffer = DiskOfflineBuffer(tmp_path / "failed.ndjson")
    buffer.append("one")

    Pipeline(
        [_ReplaySink(RuntimeError("offline"))],
        retry_policy=RetryPolicy(max_attempts=1, base_delay=0),
        offline_buffer=buffer,
    ).close()

    assert DiskOfflineBuffer(buffer.path).peek() == ["one"]


class _BlockingSink:
    def __init__(self) -> None:
        self.entered = Event()
        self.release = Event()
        self.closed = False

    def write_batch(self, _events: list[str]) -> None:
        self.entered.set()
        self.release.wait()

    def close(self) -> None:
        self.closed = True


def test_pipeline_timeout_does_not_close_sinks_under_live_workers():
    sink = _BlockingSink()
    pipeline = Pipeline([sink], retry_policy=RetryPolicy(max_attempts=1, base_delay=0))
    pipeline.start()
    assert pipeline.try_enqueue("event")
    assert sink.entered.wait(1)

    with pytest.raises(TimeoutError, match="worker"):
        pipeline.close(timeout=0.01)
    assert sink.closed is False

    sink.release.set()
    pipeline.close(timeout=1)
    assert sink.closed is True

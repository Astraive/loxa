from __future__ import annotations

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

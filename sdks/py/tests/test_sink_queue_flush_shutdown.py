from __future__ import annotations

import loxa


def test_sink_queue_flush_shutdown_helpers():
    logger, sink = loxa.TestLogger()
    logger.info("hello")
    logger.flush()
    assert len(loxa.DecodeEvents(sink)) == 1

    ms = loxa.multi_sink(loxa.MemorySink(), loxa.MemorySink())
    ms.write('{"test": true}')
    assert len(ms._sinks) == 2

    s = loxa.MemorySink()
    loxa.drain(s)
    loxa.pause(s)
    loxa.resume(s)
    assert loxa.queue_size(s) == 0
    assert loxa.health(s) is True
    assert loxa.otlp_sink() is not None

    loxa.flush()
    loxa.shutdown()

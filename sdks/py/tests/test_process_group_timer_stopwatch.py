from __future__ import annotations

import json
import time

import loxa


def test_process_group_timer_helpers():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.timing"))

    p = loxa.with_process(ctx, "step1")
    p.finish()
    g = loxa.with_group(ctx, "phase1")
    g.finish()
    t = loxa.with_timer(ctx, "timer1")
    t.stop()

    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert len(payload.get("processes", [])) == 1
    assert len(payload.get("groups", [])) == 1
    assert len(payload.get("timers", [])) == 1


def test_measure_step_phase_span_helpers():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.spq"))

    result = loxa.measure(ctx, "op1", lambda x: x * 2, 21)
    assert result == 42

    p = loxa.step(ctx, "step1")
    p.finish()
    g = loxa.phase(ctx, "phase1")
    g.finish()
    t = loxa.span(ctx, "span1")
    t.stop()

    sw = loxa.stopwatch()
    time.sleep(0.01)
    assert sw.elapsed().total_seconds() > 0

    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert len(payload.get("processes", [])) >= 1
    assert len(payload.get("groups", [])) >= 1
    assert len(payload.get("timers", [])) >= 2

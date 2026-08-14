"""Tests for timing primitives: Process, Timer, Group, Stopwatch."""

import time
import loza


def test_process_basic():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    proc = ev.start_process("redirect_to_gateway")
    time.sleep(0.01)
    proc.finish(status_code=302, gateway="stripe")

    assert len(ev.processes) == 1
    p = ev.processes[0]
    assert p["step"] == 1
    assert p["name"] == "redirect_to_gateway"
    assert p["status_code"] == 302
    assert p["gateway"] == "stripe"
    assert p["duration_ms"] >= 1
    assert p["started_at_ms"] >= 0
    assert p["ended_at_ms"] > p["started_at_ms"]


def test_process_finish_error():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    proc = ev.start_process("payment_attempt")
    time.sleep(0.01)
    proc.finish_error(ValueError("gateway timeout"), status_code=504)

    assert len(ev.processes) == 1
    p = ev.processes[0]
    assert p["status_code"] == 504
    assert p["error_message"] == "gateway timeout"


def test_step_counter():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    for _ in range(3):
        proc = ev.start_process("step")
        proc.finish()

    assert len(ev.processes) == 3
    for i, p in enumerate(ev.processes):
        assert p["step"] == i + 1


def test_timer_start_stop():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    timer = ev.start_timer("stripe.create_session")
    time.sleep(0.01)
    timer.stop(status_code=200)

    assert len(ev.timers) == 1
    t = ev.timers[0]
    assert t["name"] == "stripe.create_session"
    assert t["duration_ms"] >= 1
    assert t["status_code"] == 200


def test_group_start_finish():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    group = ev.start_group("payment_flow")
    time.sleep(0.01)
    group.finish(status_code=402, final_reason="insufficient_funds")

    assert len(ev.groups) == 1
    g = ev.groups[0]
    assert g["name"] == "payment_flow"
    assert g["status_code"] == 402
    assert g["duration_ms"] >= 1
    assert g["final_reason"] == "insufficient_funds"


def test_stopwatch_elapsed():
    sw = loza.stopwatch()
    time.sleep(0.01)
    elapsed = sw.elapsed()
    assert elapsed.total_seconds() >= 0.01


def test_timing_in_to_dict():
    ev = loza.start_event(loza.Params(service="test", event="test.event"))
    proc = ev.start_process("step1")
    proc.finish(status_code=200)
    group = ev.start_group("phase1")
    group.finish(status_code=200)
    timer = ev.start_timer("t1")
    timer.stop(status_code=200)

    d = ev.to_dict()
    assert "processes" in d
    assert "groups" in d
    assert "timers" in d
    assert len(d["processes"]) == 1
    assert len(d["groups"]) == 1
    assert len(d["timers"]) == 1

from __future__ import annotations

import json

import loza


def test_level_notice():
    assert loza.LevelNotice == "notice"
    assert loza.ParseLevel("notice") == "notice"


def test_notice_facade():
    logger, sink = loza.TestLogger()
    logger.notice("test notice", foo="bar")
    payload = json.loads(sink.events[0])
    assert payload["level"] == "notice"
    assert payload["message"] == "test notice"


def test_event_facade():
    logger, sink = loza.TestLogger()
    logger.event("my.event", foo="bar")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "my.event"


def test_track_facade():
    logger, sink = loza.TestLogger()
    logger.track("page_view", page="/home")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "page_view"


def test_audit_facade():
    logger, sink = loza.TestLogger()
    logger.audit("user.login", user_id="abc")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "user.login"


def test_security_facade():
    logger, sink = loza.TestLogger()
    logger.security("auth.fail", reason="bad_password")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "auth.fail"
    assert payload["level"] == "warn"


def test_metric_facade():
    logger, sink = loza.TestLogger()
    logger.metric("api.latency", 42, unit="ms")
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["metric_value"] == 42
    assert payload["attrs"]["unit"] == "ms"


def test_count_facade():
    logger, sink = loza.TestLogger()
    logger.count("api.requests", 5)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["count"] == 5
    assert payload["attrs"]["metric_kind"] == "count"


def test_gauge_facade():
    logger, sink = loza.TestLogger()
    logger.gauge("cpu.usage", 78.5)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["gauge"] == 78.5
    assert payload["attrs"]["metric_kind"] == "gauge"


def test_histogram_facade():
    logger, sink = loza.TestLogger()
    logger.histogram("response.size", 2048.0)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["histogram_value"] == 2048.0
    assert payload["attrs"]["metric_kind"] == "histogram"


def test_breadcrumb_facade():
    logger, sink = loza.TestLogger()
    logger.breadcrumb("nav.click", button="submit")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "nav.click"
    assert payload["kind"] == "checkpoint"
    assert payload["level"] == "debug"


def test_uppercase_facade_methods():
    logger, sink = loza.TestLogger()
    logger.notice("hello notice", foo="bar")
    payload = json.loads(sink.events[-1])
    assert payload["level"] == "notice"

    logger.event("my.event")
    payload = json.loads(sink.events[-1])
    assert payload["event"] == "my.event"

    logger.track("page_view")
    logger.audit("user.login")
    logger.security("auth.fail")
    logger.metric("latency", 42)
    logger.count("requests")
    logger.gauge("cpu", 50.0)
    logger.histogram("size", 100.0)
    logger.breadcrumb("nav.click")

"""Tests for loza:// DSN parser — runs all 25 shared test vectors."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from loza.core.dsn import LozaDSN, parse


_TEST_CASES_PATH = (
    Path(__file__).resolve().parents[3] / "spec" / "dsn" / "test-cases.json"
)


def _load_cases():
    data = json.loads(_TEST_CASES_PATH.read_text(encoding="utf-8"))
    return data["cases"]


CASES = _load_cases()
VALID_CASES = [c for c in CASES if c["valid"]]
INVALID_CASES = [c for c in CASES if not c["valid"]]


@pytest.mark.parametrize(
    "case",
    VALID_CASES,
    ids=[c["name"] for c in VALID_CASES],
)
def test_valid_dsn(case):
    dsn = parse(case["input"])
    expected = case["expected"]

    assert dsn.scheme == expected.get("scheme", "loza")
    assert dsn.host == expected["host"]
    assert dsn.port == expected["port"]
    assert dsn.project == expected["project"]
    if "collectorName" in expected:
        assert dsn.collector_name == expected["collectorName"]
        assert dsn.project == dsn.collector_name
    assert dsn.env == expected.get("env", "default")
    assert dsn.service == expected.get("service", "")
    assert dsn.tls == expected["tls"]
    assert dsn.transport == expected.get("transport", "http")

    if "baseURL" in expected:
        assert dsn.base_url == expected["baseURL"]
    if "eventsURL" in expected:
        assert dsn.events_url == expected["eventsURL"]
    if "batchURL" in expected:
        assert dsn.batch_url == expected["batchURL"]
    if "otlpURL" in expected:
        assert dsn.otlp_url == expected["otlpURL"]
    if "tailWSURL" in expected:
        assert dsn.tail_ws_url == expected["tailWSURL"]


@pytest.mark.parametrize(
    "case",
    INVALID_CASES,
    ids=[c["name"] for c in INVALID_CASES],
)
def test_invalid_dsn(case):
    with pytest.raises(ValueError):
        parse(case["input"])


def test_dsn_repr_redacts_credentials() -> None:
    dsn = parse("loza://key-id:s%40cret%3Avalue@example.com/project")
    rendered = repr(dsn)
    assert "s@cret:value" not in rendered
    assert "credentials='<redacted>'" in rendered


def test_public_dsn_routes_and_redacts_bearer_capability() -> None:
    capability = "lx_pub_6DJvd3D0izOaQx3n5BhKqN"
    dsn = parse(f"loza://{capability}:@example.com/public-collector")

    assert dsn.username == capability
    assert dsn.password == ""
    assert dsn.collector_name == "public-collector"
    assert dsn.events_url == "https://example.com:443/collectors/public-collector/events"
    assert dsn.batch_url == "https://example.com:443/collectors/public-collector/events/batch"
    assert dsn.otlp_url == "https://example.com:443/collectors/public-collector/otlp/logs"
    assert dsn.tail_ws_url == "wss://example.com:443/collectors/public-collector/tail"
    assert capability not in repr(dsn)
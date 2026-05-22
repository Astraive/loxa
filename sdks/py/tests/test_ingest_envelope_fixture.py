from __future__ import annotations

import json
from pathlib import Path

import pytest

from loxa.core.http_client import CollectorClient
from loxa.core.http_client import _validate_ingest_envelope


def _fixture(name: str) -> dict:
    root = Path(__file__).resolve().parents[3] / "spec" / "examples" / "golden" / "ingest-envelopes"
    return json.loads((root / name).read_text())


def test_collector_client_matches_wrapped_batch_envelope_fixture() -> None:
    fixture = _fixture("wrapped_batch_json.json")
    client = CollectorClient("http://collector.example/v1/events", service="checkout")
    encoded = [json.dumps(event, separators=(",", ":")) for event in fixture["input_events"]]
    body = client.envelope(encoded)
    payload = json.loads(body.decode("utf-8"))

    expected = fixture["expected"]
    assert payload["api_version"] == expected["api_version"]
    assert payload["source"]["service"] == expected["source.service"]
    assert len(payload["events"]) == expected["events_count"]
    assert payload["events"][0]["event"] == expected["first_event.event"]
    assert payload["events"][0]["service"] == expected["first_event.service"]


def test_validate_ingest_envelope_rejects_missing_events() -> None:
    with pytest.raises(ValueError):
        _validate_ingest_envelope(
            {
                "api_version": "v1",
                "source": {"sdk": "loxa-py", "version": "1.0.0", "service": "checkout"},
                "events": [],
            }
        )

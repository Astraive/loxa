from __future__ import annotations

from copy import deepcopy

from loxa.generated.spec_contract import build_ingest_envelope, normalize_event_aliases


def _event_with_alias_only() -> dict[str, object]:
    return {
        "schema_version": "v1",
        "event_version": "v1",
        "event_id": "evt_non_mutating_alias",
        "timestamp": "2026-05-19T14:12:22Z",
        "service": "checkout",
        "event_type": "checkout.request",
        "kind": "event",
    }


def test_normalize_event_aliases_returns_copy_without_mutating_input() -> None:
    payload = _event_with_alias_only()
    original = deepcopy(payload)
    normalized, changed = normalize_event_aliases(payload)

    assert changed is True
    assert payload == original
    assert "event_type" in payload
    assert "event" not in payload
    assert normalized["event"] == "checkout.request"
    assert "event_type" not in normalized


def test_build_ingest_envelope_normalizes_copy_without_mutating_input_event() -> None:
    payload = _event_with_alias_only()
    original = deepcopy(payload)

    envelope = build_ingest_envelope([payload], "loxa-py", "0.0.1", "checkout")
    envelope_event = envelope["events"][0]

    assert payload == original
    assert "event_type" in payload
    assert "event" not in payload
    assert envelope_event["event"] == "checkout.request"
    assert "event_type" not in envelope_event

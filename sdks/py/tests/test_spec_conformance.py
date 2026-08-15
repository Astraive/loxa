from __future__ import annotations

import json
from pathlib import Path
from typing import Any


def _spec_root() -> Path:
    return Path(__file__).resolve().parents[3] / "spec"


def _load_manifest() -> dict[str, Any]:
    manifest_path = _spec_root() / "conformance" / "manifest.json"
    if not manifest_path.exists():
        manifest_path = _spec_root() / "examples" / "golden" / "manifest.json"
    payload = json.loads(manifest_path.read_text())
    payload["_manifest_dir"] = str(manifest_path.parent)
    return payload


def _load_payload(path: Path) -> Any:
    return json.loads(path.read_text())


def _fixture_paths(manifest: dict[str, Any], key: str) -> list[Path]:
    root = Path(manifest["_manifest_dir"])
    return [(root / rel).resolve() for rel in manifest.get(key, [])]


def _validate_event(payload: dict[str, Any]) -> None:
    assert payload["schema_version"] == "v1"
    assert payload["event_version"] == "v1"
    assert isinstance(payload["event_id"], str) and payload["event_id"].strip()
    assert isinstance(payload["timestamp"], str) and payload["timestamp"].strip()
    assert "service" in payload
    service = payload["service"]
    if isinstance(service, str):
        assert service.strip()
    else:
        assert isinstance(service, dict)
        assert isinstance(service.get("name"), str) and service["name"].strip()
    assert isinstance(payload["event"], str) and payload["event"].strip()
    assert payload["kind"] in {"event", "http", "job", "queue", "cli", "cron", "log", "checkpoint", "agent", "ai"}


def test_valid_golden_fixtures_match_expected_contract() -> None:
    manifest = _load_manifest()
    for path in _fixture_paths(manifest, "valid"):
        payload = _load_payload(path)
        if {"request_id", "status", "accepted", "rejected"} <= set(payload):
            assert payload["status"] in {"accepted", "partial", "rejected", "invalid"}
            continue
        _validate_event(payload)


def test_invalid_golden_fixture_examples_stay_invalid() -> None:
    manifest = _load_manifest()
    invalid_files = {path.name: path for path in _fixture_paths(manifest, "invalid")}
    missing_event_id = _load_payload(invalid_files["missing_event_id.json"])
    assert "event_id" not in missing_event_id

    invalid_enums = _load_payload(invalid_files["invalid_enum_values.json"])
    assert invalid_enums["kind"] not in {
        "event",
        "http",
        "job",
        "queue",
        "cli",
        "cron",
        "log",
        "checkpoint",
        "agent",
        "ai",
    }

    missing_versions = _load_payload(invalid_files["missing_versions.json"])
    assert "schema_version" not in missing_versions or "event_version" not in missing_versions


def test_alias_mode_fixtures_stay_explicit() -> None:
    manifest = _load_manifest()
    loose_only = _fixture_paths(manifest, "loose_only_valid")
    strict_only_invalid = _fixture_paths(manifest, "strict_only_invalid")
    assert loose_only and strict_only_invalid
    for path in loose_only + strict_only_invalid:
        payload = _load_payload(path)
        assert "event_type" in payload
        assert "event" in payload


def test_manifest_fixture_groups_exist() -> None:
    manifest = _load_manifest()
    for key in (
        "valid",
        "loose_only_valid",
        "invalid",
        "strict_only_invalid",
        "invalid_ingest",
        "invalid_collector_response",
        "invalid_limits",
        "emitted_shape",
        "collector_ack_behavior",
        "ingest_envelopes",
    ):
        paths = _fixture_paths(manifest, key)
        assert paths, f"expected fixtures in group {key}"
        for path in paths:
            assert path.exists()

from __future__ import annotations

import json
from typing import Any


def render_conformance_manifest(contract: dict[str, Any]) -> str:
    fixtures = contract["fixtures"]
    def remap(path: str) -> str:
        if path.startswith("valid/") or path.startswith("invalid/"):
            return f"../fixtures/{path}"
        if path.startswith("collector-acks/"):
            return f"../fixtures/collector-responses/{path.split('/', 1)[1]}"
        if path.startswith("ingest-envelopes/"):
            return f"../fixtures/ingest/{path.split('/', 1)[1]}"
        if path.startswith("emitted-shape/"):
            return f"../fixtures/emitted-shape/{path.split('/', 1)[1]}"
        return f"../fixtures/{path}"
    payload = {
        "version": contract["spec_version"],
        "strict_schema": "../schema/event.strict.schema.json",
        "loose_schema": "../schema/event.loose.schema.json",
        "valid": [remap(path) for path in fixtures.get("valid", [])],
        "loose_only_valid": [remap(path) for path in fixtures.get("loose_only_valid", [])],
        "invalid": [remap(path) for path in fixtures.get("invalid", [])],
        "strict_only_invalid": [remap(path) for path in fixtures.get("strict_only_invalid", [])],
        "invalid_ingest": [remap(path) for path in fixtures.get("invalid_ingest", [])],
        "invalid_collector_response": [remap(path) for path in fixtures.get("invalid_collector_response", [])],
        "invalid_limits": [remap(path) for path in fixtures.get("invalid_limits", [])],
        "emitted_shape": [remap(path) for path in fixtures.get("emitted_shape", [])],
        "collector_ack_behavior": [remap(path) for path in fixtures.get("collector_ack_behavior", [])],
        "ingest_envelopes": [remap(path) for path in fixtures.get("ingest_envelopes", [])],
        "fixtures_by_coverage": fixtures.get("fixtures_by_coverage", {}),
    }
    return json.dumps(payload, indent=2) + "\n"

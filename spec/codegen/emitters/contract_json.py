from __future__ import annotations

import json
from typing import Any


def render_contract_json(contract: dict[str, Any]) -> str:
    payload = {
        "spec_version": contract["spec_version"],
        "api_version": contract["api_version"],
        "event_version": contract["event_version"],
        "schemas": contract["schemas"],
        "required_fields": contract["required_fields"],
        "allowed_top_level_fields": contract["allowed_top_level_fields"],
        "canonical_fields": contract["canonical_fields"],
        "enums": contract["enums"],
        "collector_statuses": contract["collector_statuses"],
        "collector_ack_statuses": contract["collector_ack_statuses"],
        "aliases": contract["aliases"],
        "alias_policy": contract["alias_policy"],
        "strict_mode": contract["strict_mode"],
        "loose_mode": contract["loose_mode"],
        "validation_modes": contract["validation_modes"],
        "wire_formats": contract["wire_formats"],
        "limits": contract["limits"],
    }
    return json.dumps(payload, indent=2) + "\n"

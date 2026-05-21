from __future__ import annotations

import json
from typing import Any


def render_cortex_contract_json(contract: dict[str, Any]) -> str:
    payload = {
        "spec_version": contract["spec_version"],
        "api_version": contract["api_version"],
        "schemas": contract["schemas"],
        "event_kinds": contract["event_kinds"],
        "event_levels": contract["event_levels"],
        "event_outcomes": contract["event_outcomes"],
        "provenance": contract["provenance"],
        "graph_node_types": contract["graph_node_types"],
        "graph_edge_types": contract["graph_edge_types"],
        "routes": contract["routes"],
    }
    return json.dumps(payload, indent=2) + "\n"

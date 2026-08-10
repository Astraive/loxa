from __future__ import annotations

import json
from pathlib import Path
from typing import Any


_FALLBACK_PRODUCT_VERSION = "0.2.6"


def _load_product_version(spec_root: Path) -> str:
    """Read version from loza-spec.yaml, falling back to hardcoded default."""
    candidates = [
        spec_root / "loza-spec.yaml",
        spec_root.parent / "loza-spec.yaml",
    ]
    for path in candidates:
        if path.is_file():
            try:
                text = path.read_text(encoding="utf-8")
                for line in text.splitlines():
                    stripped = line.strip()
                    if stripped.startswith("version:"):
                        value = stripped.split(":", 1)[1].strip().strip("\"'")
                        if value:
                            return value
            except OSError:
                continue
    return _FALLBACK_PRODUCT_VERSION


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def first_existing(*paths: Path) -> Path:
    for path in paths:
        if path.exists():
            return path
    raise FileNotFoundError("none of these paths exist: " + ", ".join(str(p) for p in paths))


def normalize_manifest(manifest: dict[str, Any]) -> dict[str, Any]:
    def normalize_entry(path: str) -> str:
        normalized = path.replace("\\", "/")
        while normalized.startswith("../"):
            normalized = normalized[3:]
        mappings = {
            "fixtures/valid/": "valid/",
            "fixtures/invalid/": "invalid/",
            "fixtures/emitted-shape/": "emitted-shape/",
            "fixtures/collector-responses/": "collector-acks/",
            "fixtures/ingest/": "ingest-envelopes/",
            "examples/golden/valid/": "valid/",
            "examples/golden/invalid/": "invalid/",
            "examples/golden/emitted-shape/": "emitted-shape/",
            "examples/golden/collector-acks/": "collector-acks/",
            "examples/golden/ingest-envelopes/": "ingest-envelopes/",
        }
        for prefix, replacement in mappings.items():
            if normalized.startswith(prefix):
                return replacement + normalized[len(prefix):]
        return normalized

    out = dict(manifest)
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
        out[key] = [normalize_entry(item) for item in manifest.get(key, [])]
    return out


def build_contract(spec_root: Path) -> dict[str, Any]:
    # Canonical paths first (spec/), then legacy mirrors for compatibility
    event_schema_path = first_existing(
        spec_root / "spec" / "schemas" / "json" / "event.schema.json",
        spec_root / "schema" / "event.schema.json",
    )
    strict_schema_path = first_existing(
        spec_root / "spec" / "schemas" / "json" / "event.strict.schema.json",
        spec_root / "schema" / "event.strict.schema.json",
    )
    loose_schema_path = first_existing(
        spec_root / "spec" / "schemas" / "json" / "event.loose.schema.json",
        spec_root / "schema" / "event.loose.schema.json",
    )
    ingest_schema_path = first_existing(
        spec_root / "spec" / "schemas" / "json" / "ingest-envelope.schema.json",
        spec_root / "schema" / "ingest.schema.json",
    )
    collector_response_path = first_existing(
        spec_root / "spec" / "schemas" / "json" / "collector-response.schema.json",
        spec_root / "schema" / "collector-response.schema.json",
    )
    manifest_path = first_existing(
        spec_root / "conformance" / "manifest.json",
        spec_root / "examples" / "golden" / "manifest.json",
    )

    event_schema = load_json(event_schema_path)
    ingest_schema = load_json(ingest_schema_path)
    collector_response_schema = load_json(collector_response_path)
    manifest = normalize_manifest(load_json(manifest_path))

    props = event_schema["properties"]
    source_sdk_values: set[str] = set()
    for variant in ingest_schema["properties"]["source"]["properties"]["sdk"]["oneOf"]:
        if isinstance(variant, dict):
            source_sdk_values.update(variant.get("enum", []))
            source_sdk_values.update(
                variant.get("properties", {}).get("name", {}).get("enum", [])
            )

    collector_statuses = collector_response_schema["properties"]["status"]["enum"]
    collector_ack_statuses = (
        collector_response_schema["properties"]["acks"]["items"]["properties"]["status"]["enum"]
    )
    required_fields = event_schema["required"]
    allowed_top_level_fields = sorted(props.keys())

    return {
        "product_version": _load_product_version(spec_root),
        "spec_version": props["schema_version"]["enum"][0],
        "api_version": ingest_schema["properties"]["api_version"]["enum"][0],
        "event_version": props["event_version"]["enum"][0],
        "schemas": {
            "event": "spec/schemas/json/event.schema.json",
            "event_strict": "spec/schemas/json/event.strict.schema.json",
            "event_loose": "spec/schemas/json/event.loose.schema.json",
            "ingest_envelope": "spec/schemas/json/ingest-envelope.schema.json",
            "collector_response": "spec/schemas/json/collector-response.schema.json",
        },
        "required_fields": required_fields,
        "allowed_top_level_fields": allowed_top_level_fields,
        "canonical_fields": allowed_top_level_fields,
        "enums": {
            "schema_versions": props["schema_version"]["enum"],
            "event_versions": props["event_version"]["enum"],
            "kinds": props["kind"]["enum"],
            "levels": props["level"]["enum"],
            "outcomes": props["outcome"]["enum"],
            "partial_reasons": props["partial_reason"]["enum"],
            "event_states": props["event_state"]["enum"],
            "source_sdks": sorted(source_sdk_values),
        },
        "collector_statuses": collector_statuses,
        "collector_ack_statuses": collector_ack_statuses,
        "aliases": {
            "event_type": "event",
        },
        "alias_policy": {
            "strict_mode": "reject_before_normalization",
            "loose_mode": "normalize_then_use",
        },
        "strict_mode": {
            "allow_unknown_top_level_fields": False,
            "allow_aliases": False,
            "enforce_required_fields": True,
            "enforce_enums": True,
            "enforce_status_codes": True,
            "enforce_timestamps": True,
            "normalize_aliases": False,
        },
        "loose_mode": {
            "allow_unknown_top_level_fields": True,
            "allow_aliases": True,
            "enforce_required_fields": True,
            "enforce_enums": True,
            "normalize_aliases": True,
        },
        "validation_modes": {
            "off": {
                "name": "off",
                "description": "Accept everything, still normalize payload",
                "accept_all": True,
                "normalize": True,
                "validate": False,
                "reject_on_failure": False,
            },
            "warn": {
                "name": "warn",
                "description": "Accept and report schema issues as warnings",
                "accept_all": True,
                "normalize": True,
                "validate": True,
                "reject_on_failure": False,
            },
            "enforce": {
                "name": "enforce",
                "description": "Reject invalid events, accept valid ones only",
                "accept_all": False,
                "normalize": True,
                "validate": True,
                "reject_on_failure": True,
            },
            "quarantine": {
                "name": "quarantine",
                "description": "Store invalid events separately in quarantine, accept valid ones",
                "accept_all": False,
                "normalize": True,
                "validate": True,
                "reject_on_failure": False,
                "quarantine_on_failure": True,
            },
            "strict": {
                "name": "strict",
                "description": "Reject unknown fields, aliases, and enforce all constraints",
                "allow_aliases": False,
                "allow_unknown_top_level_fields": False,
                "reject_on_failure": True,
            },
            "loose": {
                "name": "loose",
                "description": "Accept aliases and some unknown fields, normalize before validation",
                "allow_aliases": True,
                "allow_unknown_top_level_fields": True,
            },
        },
        "wire_formats": ["json", "jsonl", "protobuf"],
        "limits": {
            "max_event_size_bytes": 65536,
            "max_batch_events": 1000,
            "max_batch_size_bytes": 1048576,
            "max_attrs_depth": 8,
            "max_attr_key_length": 128,
            "max_attr_value_length": 4096,
            "max_error_stack_length": 16384,
        },
        "fixtures": manifest,
        "paths": {
            "event_schema": str(event_schema_path.relative_to(spec_root)).replace("\\", "/"),
            "strict_schema": str(strict_schema_path.relative_to(spec_root)).replace("\\", "/"),
            "loose_schema": str(loose_schema_path.relative_to(spec_root)).replace("\\", "/"),
            "ingest_schema": str(ingest_schema_path.relative_to(spec_root)).replace("\\", "/"),
            "collector_response_schema": str(collector_response_path.relative_to(spec_root)).replace("\\", "/"),
            "manifest": str(manifest_path.relative_to(spec_root)).replace("\\", "/"),
        },
    }


def build_cortex_contract(spec_root: Path) -> dict[str, Any]:
    """Build Cortex-specific contract from schemas and definitions."""
    return {
        "spec_version": "v1",
        "api_version": "v1",
        "schemas": {
            "event": "spec/cortex/schemas/json/cortex-event.schema.json",
            "graph_node": "spec/cortex/schemas/json/cortex-graph-node.schema.json",
            "graph_edge": "spec/cortex/schemas/json/cortex-graph-edge.schema.json",
            "reconstruct_response": "spec/cortex/schemas/json/cortex-reconstruct-response.schema.json",
            "feedback": "spec/cortex/schemas/json/cortex-feedback.schema.json",
        },
        "event_kinds": [
            "event", "http", "job", "queue", "cli", "cron", "log",
            "checkpoint", "deploy", "metric", "trace", "topology",
            "incident_signal", "remediation", "loza_event", "otel_log",
            "otel_span", "collector_event"
        ],
        "event_levels": ["debug", "info", "warning", "error", "critical"],
        "event_outcomes": ["success", "failure", "partial", "unknown"],
        "provenance": [
            "loza", "collector", "otlp", "jsonl", "manual", "replay"
        ],
        "graph_node_types": [
            "service", "event", "trace", "span", "request",
            "deployment", "metric", "log", "incident", "remediation", "resource"
        ],
        "graph_edge_types": [
            "depends_on", "same_trace", "same_incident", "parent_span",
            "calls", "deployed_before", "metric_spiked_after",
            "log_error_after", "caused_probably", "remediated_by", "similar_shape"
        ],
        "routes": {
            "ingest_event": "POST /events",
            "ingest_batch": "POST /events/batch",
            "ingest_jsonl": "POST /events/jsonl",
            "reconstruct": "POST /incidents/{incident_id}/reconstruct",
            "service_graph": "GET /graph/service/{service}",
            "incident_graph": "GET /graph/incident/{incident_id}",
            "record_feedback": "POST /feedback/remediation",
            "stream_events": "GET /ws",
        },
    }

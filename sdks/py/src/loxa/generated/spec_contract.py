from __future__ import annotations

# This module acts as a compatibility layer and re-exports from the authoritative loxa-spec contract.
# Do NOT add new constants here—all definitions come from loxa-spec/generated/python/loxa_contract.py

import sys
import importlib.machinery
import importlib.util
from pathlib import Path

# Try importing from loxa-spec (primary path - when installed as dependency)
try:
    from loxa_contract import (
        LOXA_SPEC_VERSION,
        LOXA_INGEST_API_VERSION,
        LOXA_EVENT_VERSION,
        MAX_EVENT_BYTES,
        ALLOWED_TOP_LEVEL_FIELDS,
        CANONICAL_FIELDS,
        ALLOWED_KINDS,
        ALLOWED_LEVELS,
        ALLOWED_OUTCOMES,
        ALLOWED_PARTIAL_REASONS,
        ALLOWED_EVENT_STATES,
        CollectorAck,
        CollectorError,
        CollectorResponse,
        parse_collector_response,
        looks_like_loxa_event,
        normalize_event_aliases,
        build_ingest_envelope,
        validate_event_payload,
        validate_event_payload_detailed,
        validate_flexible_json_bytes,
        ValidationError,
        ValidationErrors,
    )
except Exception:
    # Fallback: try importing from sibling loxa-spec directory (for development/monorepo)
    try:
        spec_dir = Path(__file__).parent.parent.parent.parent.parent.parent / "spec" / "generated" / "python"
        if spec_dir.exists():
            sys.path.insert(0, str(spec_dir))
            from loxa_contract import (
                LOXA_SPEC_VERSION,
                LOXA_INGEST_API_VERSION,
                LOXA_EVENT_VERSION,
                MAX_EVENT_BYTES,
                ALLOWED_TOP_LEVEL_FIELDS,
                CANONICAL_FIELDS,
                ALLOWED_KINDS,
                ALLOWED_LEVELS,
                ALLOWED_OUTCOMES,
                ALLOWED_PARTIAL_REASONS,
                ALLOWED_EVENT_STATES,
                CollectorAck,
                CollectorError,
                CollectorResponse,
                parse_collector_response,
                looks_like_loxa_event,
                normalize_event_aliases,
                build_ingest_envelope,
                validate_event_payload,
                validate_event_payload_detailed,
                validate_flexible_json_bytes,
                ValidationError,
                ValidationErrors,
            )
        else:
            raise ImportError(f"loxa-spec not found at {spec_dir}")
    except Exception as e:
        backup_path = Path(__file__).with_name("spec_contract.py.bak")
        if not backup_path.exists():
            raise ImportError(
                "Cannot import loxa contract. Install loxa-spec as a dependency or ensure it's available at spec/generated/python/"
            ) from e
        loader = importlib.machinery.SourceFileLoader(
            "loxa_generated_spec_contract_backup", str(backup_path)
        )
        spec = importlib.util.spec_from_loader(loader.name, loader)
        if spec is None:
            raise ImportError(f"Cannot load fallback contract module from {backup_path}") from e
        backup = importlib.util.module_from_spec(spec)
        sys.modules[loader.name] = backup
        loader.exec_module(backup)

        LOXA_SPEC_VERSION = backup.LOXA_SPEC_VERSION
        LOXA_INGEST_API_VERSION = backup.LOXA_INGEST_API_VERSION
        LOXA_EVENT_VERSION = backup.LOXA_EVENT_VERSION
        MAX_EVENT_BYTES = getattr(backup, "MAX_EVENT_BYTES", 65536)
        ALLOWED_TOP_LEVEL_FIELDS = backup.ALLOWED_TOP_LEVEL_FIELDS
        CANONICAL_FIELDS = backup.CANONICAL_FIELDS
        ALLOWED_KINDS = backup.ALLOWED_KINDS
        ALLOWED_LEVELS = backup.ALLOWED_LEVELS
        ALLOWED_OUTCOMES = backup.ALLOWED_OUTCOMES
        ALLOWED_PARTIAL_REASONS = backup.ALLOWED_PARTIAL_REASONS
        ALLOWED_EVENT_STATES = backup.ALLOWED_EVENT_STATES
        CollectorAck = backup.CollectorAck
        CollectorError = backup.CollectorError
        CollectorResponse = backup.CollectorResponse
        parse_collector_response = backup.parse_collector_response
        looks_like_loxa_event = backup.looks_like_loxa_event
        normalize_event_aliases = backup.normalize_event_aliases
        build_ingest_envelope = backup.build_ingest_envelope
        validate_event_payload = backup.validate_event_payload
        validate_event_payload_detailed = getattr(backup, "validate_event_payload_detailed", backup.validate_event_payload)
        validate_flexible_json_bytes = getattr(backup, "validate_flexible_json_bytes", None)
        ValidationError = getattr(backup, "ValidationError", ValueError)
        ValidationErrors = getattr(backup, "ValidationErrors", list)

__all__ = [
    "LOXA_SPEC_VERSION",
    "LOXA_INGEST_API_VERSION",
    "LOXA_EVENT_VERSION",
    "MAX_EVENT_BYTES",
    "ALLOWED_TOP_LEVEL_FIELDS",
    "CANONICAL_FIELDS",
    "ALLOWED_KINDS",
    "ALLOWED_LEVELS",
    "ALLOWED_OUTCOMES",
    "ALLOWED_PARTIAL_REASONS",
    "ALLOWED_EVENT_STATES",
    "CollectorAck",
    "CollectorError",
    "CollectorResponse",
    "parse_collector_response",
    "looks_like_loxa_event",
    "normalize_event_aliases",
    "build_ingest_envelope",
    "validate_event_payload",
    "validate_event_payload_detailed",
    "validate_flexible_json_bytes",
    "ValidationError",
    "ValidationErrors",
]

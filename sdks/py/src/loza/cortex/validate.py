"""Cortex validation stub - lightweight SDK interface.

Full validation logic lives in the loza-cortex package.
"""

from __future__ import annotations

from typing import Any


def validate_cortex_event(event: dict[str, Any], strict: bool = False) -> list[str]:
    """Validate an event for cortex ingestion (stub)."""
    errors: list[str] = []
    if not event.get("event_id"):
        errors.append("missing event_id")
    return errors


def is_valid_cortex_event(event: dict[str, Any]) -> bool:
    return len(validate_cortex_event(event)) == 0

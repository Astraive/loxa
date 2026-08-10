"""Cortex normalization stub - lightweight SDK interface.

Full normalization lives in the loza-cortex package.
"""

from __future__ import annotations

from typing import Any


def normalize_cortex_event(event: dict[str, Any]) -> dict[str, Any]:
    """Normalize an event for cortex ingestion (stub)."""
    return dict(event)

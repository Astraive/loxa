"""No-op redactor - passes events through unchanged."""

from __future__ import annotations

from typing import Any


def noop_redactor() -> Any:
    """Return a redactor that performs no transformation."""
    return lambda payload: payload

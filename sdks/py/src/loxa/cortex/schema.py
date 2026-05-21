"""Cortex schema stub - lightweight SDK interface.

Full cortex schema lives in the loxa-cortex package.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class CortexEventSchema:
    """Stub schema for cortex event fields."""

    incident_id: str = ""
    severity: str = "info"
    category: str = ""
    source: str = ""
    tags: list[str] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)


CORTEX_SEVERITY_LEVELS = ("info", "warn", "error", "critical")

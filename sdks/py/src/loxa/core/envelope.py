"""Event envelope for transport encoding."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(slots=True)
class Envelope:
    """Wraps an event payload with transport metadata."""

    payload: dict[str, Any] = field(default_factory=dict)
    content_type: str = "application/json"
    encoding: str = "utf-8"
    compressed: bool = False
    batch_size: int = 1
    sequence: int = 0

    def to_json(self) -> str:
        import json
        return json.dumps(self.payload, separators=(",", ":"), sort_keys=True)

    @classmethod
    def from_event(cls, event_dict: dict[str, Any]) -> Envelope:
        return cls(payload=event_dict)

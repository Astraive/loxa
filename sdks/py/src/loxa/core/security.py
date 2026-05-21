from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any

from .config import SecurityConfig
from .event import Attr


@dataclass(slots=True)
class SecurityDecision:
    allowed: bool
    reason: str = ""
    event_bytes: int = 0
    attr_count: int = 0


class SecurityLimiter:
    def __init__(self, config: SecurityConfig | None = None) -> None:
        self.config = config or SecurityConfig()

    def check_payload(self, payload: dict[str, Any]) -> SecurityDecision:
        encoded = json.dumps(payload, separators=(",", ":"), default=str).encode("utf-8")
        attr_count = self._count_attrs(payload)
        if len(encoded) > self.config.max_event_bytes:
            return SecurityDecision(
                allowed=not self.config.drop_oversized_events,
                reason="max_event_bytes",
                event_bytes=len(encoded),
                attr_count=attr_count,
            )
        if attr_count > self.config.max_attr_count:
            return SecurityDecision(False, "max_attr_count", len(encoded), attr_count)
        if self._has_oversized_field(payload):
            return SecurityDecision(False, "max_field_bytes", len(encoded), attr_count)
        return SecurityDecision(True, event_bytes=len(encoded), attr_count=attr_count)

    def _count_attrs(self, value: Any) -> int:
        if isinstance(value, dict):
            return sum(1 + self._count_attrs(child) for child in value.values())
        if isinstance(value, list):
            return sum(self._count_attrs(child) for child in value)
        return 0

    def _has_oversized_field(self, value: Any) -> bool:
        if isinstance(value, dict):
            return any(self._has_oversized_field(child) for child in value.values())
        if isinstance(value, list):
            return any(self._has_oversized_field(child) for child in value)
        if isinstance(value, str):
            return len(value.encode("utf-8")) > self.config.max_field_bytes
        return False


def hash_value(value: Any) -> str:
    return hashlib.sha256(str(value).encode("utf-8")).hexdigest()


def sensitive_string(key: str, value: str) -> Attr:
    return Attr(key, value, sensitive=True)


def hash_string(key: str, value: str) -> Attr:
    return Attr(key, hash_value(value), hash_value=True)


__all__ = [
    "SecurityConfig",
    "SecurityDecision",
    "SecurityLimiter",
    "hash_value",
    "sensitive_string",
    "hash_string",
]

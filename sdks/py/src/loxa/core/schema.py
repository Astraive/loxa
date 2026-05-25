from __future__ import annotations

import copy
from dataclasses import dataclass
from typing import Any, Callable, Protocol

from .event import EventContext


class EventView:
    """Read-only view of an EventContext for custom schemas."""

    def __init__(self, event: EventContext) -> None:
        self._event = event

    @property
    def event_id(self) -> str:
        return self._event.event_id

    def id(self) -> str:
        return self._event.event_id

    def name(self) -> str:
        return self._event.params.event or self._event.params.name

    def service(self) -> str:
        return self._event.service

    def kind(self) -> str:
        return self._event.params.kind

    def level(self) -> str:
        return self._event.params.level

    def message(self) -> str:
        return self._event.params.message

    def outcome(self) -> str:
        return self._event.outcome

    def duration_ms(self) -> int:
        return self._event.duration_ms()

    def attrs(self) -> dict[str, Any]:
        return copy.deepcopy(self._event.attrs)

    def attr(self, key: str) -> Any:
        return _lookup_path(self.attrs(), key)

    def group(self, name: str) -> dict[str, Any] | None:
        groups: dict[str, dict[str, Any]] = {
            "user": self._event.user,
            "tenant": self._event.tenant,
            "resource": self._event.resource,
            "http": self._event.http,
        }
        value = groups.get(name)
        return copy.deepcopy(value) if value else None

    def checkpoints(self) -> list[dict[str, Any]]:
        return copy.deepcopy(self._event.checkpoints)

    def error(self) -> dict[str, Any] | None:
        return copy.deepcopy(self._event.error)

    def to_dict(self) -> dict[str, Any]:
        return self._event.to_dict()


def _lookup_path(data: dict[str, Any], key: str) -> Any:
    current: Any = data
    for part in key.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current


class Schema(Protocol):
    def encode(self, event: EventView) -> dict[str, Any]: ...


SchemaFunc = Callable[[EventView], dict[str, Any]]


@dataclass(slots=True)
class CallableSchema:
    fn: SchemaFunc

    def encode(self, event: EventView) -> dict[str, Any]:
        return self.fn(event)


class DefaultSchema:
    def encode(self, event: EventView) -> dict[str, Any]:
        return event.to_dict()


class NestedSchema(DefaultSchema):
    pass


class FlatSchema:
    def encode(self, event: EventView) -> dict[str, Any]:
        payload = event.to_dict()
        out: dict[str, Any] = {}
        self._flatten("", payload, out)
        return out

    def _flatten(self, prefix: str, value: Any, out: dict[str, Any]) -> None:
        if isinstance(value, dict):
            for key, child in value.items():
                next_key = f"{prefix}_{key}" if prefix else key
                self._flatten(next_key, child, out)
            return
        out[prefix] = value


class OTelLogSchema:
    def encode(self, event: EventView) -> dict[str, Any]:
        payload = event.to_dict()
        attrs = {k: v for k, v in payload.items() if k not in {"timestamp", "level", "message"}}
        return {
            "time_unix_nano": payload.get("timestamp"),
            "severity_text": payload.get("level", "info"),
            "body": payload.get("message") or payload.get("event"),
            "attributes": attrs,
        }


class ECSchema:
    def encode(self, event: EventView) -> dict[str, Any]:
        payload = event.to_dict()
        return {
            "@timestamp": payload.get("timestamp"),
            "event": {
                "id": payload.get("event_id"),
                "action": payload.get("event"),
                "kind": payload.get("kind"),
                "outcome": payload.get("outcome"),
                "duration": payload.get("duration_ms"),
            },
            "log": {"level": payload.get("level")},
            "service": {"name": payload.get("service")},
            "labels": payload.get("attrs", {}),
        }


class DatadogSchema:
    def encode(self, event: EventView) -> dict[str, Any]:
        payload = event.to_dict()
        payload["ddsource"] = "loxa"
        payload["status"] = payload.get("level", "info")
        return payload


def custom_schema(fn: SchemaFunc) -> CallableSchema:
    return CallableSchema(fn)

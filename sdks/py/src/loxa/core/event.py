from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Callable
from .config import LOXA_EVENT_VERSION, LOXA_SPEC_VERSION
from .uuidv7 import uuidv7_like
from .errors import EventAlreadyFinishedError, EventClosedError

EVENT_CREATED = "created"
EVENT_ACTIVE = "active"
EVENT_FINISHED = "finished"
EVENT_EMITTING = "emitting"
EVENT_EMITTED = "emitted"
EVENT_FAILED_VALIDATION = "failed_validation"
EVENT_DELIVERY_FAILED = "delivery_failed"


@dataclass(frozen=True, slots=True)
class Attr:
    key: str
    value: Any
    sensitive: bool = False
    hash_value: bool = False
    drop: bool = False


@dataclass(slots=True)
class Params:
    event: str = ""
    name: str = ""
    kind: str = "event"
    message: str = ""
    level: str = "info"
    method: str = ""
    path: str = ""
    route: str = ""
    host: str = ""
    status_code: int = 0
    service: str = ""
    version: str = ""
    environment: str = ""
    region: str = ""
    deployment_id: str = ""
    request_id: str = ""
    trace_id: str = ""
    span_id: str = ""
    user_id: str = ""
    tenant_id: str = ""
    workspace_id: str = ""
    custom: list[Attr] = field(default_factory=list)


@dataclass(slots=True)
class EventContext:
    service: str
    params: Params
    schema_version: str = LOXA_SPEC_VERSION
    event_version: str = LOXA_EVENT_VERSION
    event_id: str = field(default_factory=lambda: uuidv7_like("evt"))
    request_id: str = field(default_factory=lambda: uuidv7_like("req"))
    trace_id: str = ""
    span_id: str = ""
    started_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    finished_at: datetime | None = None
    outcome: str = ""
    attrs: dict[str, object] = field(default_factory=dict)
    user: dict[str, object] = field(default_factory=dict)
    tenant: dict[str, object] = field(default_factory=dict)
    resource: dict[str, object] = field(default_factory=dict)
    http: dict[str, object] = field(default_factory=dict)
    error: dict[str, object] | None = None
    pii: dict[str, object] = field(default_factory=dict)
    checkpoints: list[dict[str, object]] = field(default_factory=list)
    processes: list[dict[str, object]] = field(default_factory=list)
    groups: list[dict[str, object]] = field(default_factory=list)
    timers: list[dict[str, object]] = field(default_factory=list)
    _process_step: int = field(default=0, repr=False)
    partial: bool = False
    partial_reason: str = ""
    event_state: str = EVENT_CREATED
    delivery_attempts: int = 0
    emitted: bool = False
    emitted_payload: str = ""
    _checkpoint_writer: Callable[["EventContext", dict[str, object]], None] | None = field(default=None, repr=False)
    _max_checkpoints: int = field(default=32, repr=False)
    _sensitive_keys: set[str] = field(default_factory=set, repr=False)
    _hash_keys: set[str] = field(default_factory=set, repr=False)
    _drop_keys: set[str] = field(default_factory=set, repr=False)

    def _ensure_mutable(self) -> None:
        if self.event_state in {EVENT_EMITTED, EVENT_EMITTING, EVENT_DELIVERY_FAILED, EVENT_FAILED_VALIDATION}:
            raise EventClosedError(f"event {self.event_id} is closed in state {self.event_state}")

    def _mark_active(self) -> None:
        if self.event_state == EVENT_CREATED:
            self.event_state = EVENT_ACTIVE

    def checkpoint(self, name: str, **attrs: object) -> None:
        self._ensure_mutable()
        max_cp = getattr(self, '_max_checkpoints', 32)
        if len(self.checkpoints) >= max_cp:
            return
        checkpoint = {
            "name": name,
            "at_ms": self.duration_ms(),
            **attrs,
        }
        self.checkpoints.append(checkpoint)
        if callable(self._checkpoint_writer):
            self._checkpoint_writer(self, dict(checkpoint))

    def start_process(self, name: str, **attrs: object) -> "ProcessHandle":
        from .timing import ProcessHandle
        self._ensure_mutable()
        self._mark_active()
        self._process_step += 1
        return ProcessHandle(self, name, self._process_step, datetime.now(timezone.utc))

    def start_timer(self, name: str, **attrs: object) -> "TimerHandle":
        from .timing import TimerHandle
        self._ensure_mutable()
        self._mark_active()
        return TimerHandle(self, name, datetime.now(timezone.utc))

    def start_group(self, name: str, **attrs: object) -> "GroupHandle":
        from .timing import GroupHandle
        self._ensure_mutable()
        self._mark_active()
        return GroupHandle(self, name, datetime.now(timezone.utc))

    def finish(self, outcome: str) -> None:
        self._ensure_mutable()
        if self.finished_at is not None:
            raise EventAlreadyFinishedError(f"event {self.event_id} already finished")
        self.outcome = outcome
        self.finished_at = datetime.now(timezone.utc)
        self.event_state = EVENT_FINISHED

    def finish_error(self, error: BaseException) -> None:
        self._ensure_mutable()
        if self.finished_at is not None:
            raise EventAlreadyFinishedError(f"event {self.event_id} already finished")
        self.outcome = "error"
        self.finished_at = datetime.now(timezone.utc)
        self.event_state = EVENT_FINISHED
        self.params.level = "error"
        self.error = {"type": error.__class__.__name__, "message": str(error)}

    def mark_partial(self, reason: str) -> None:
        self._ensure_mutable()
        self.partial = True
        self.partial_reason = reason
        self.event_state = EVENT_FINISHED

    def duration_ms(self) -> int:
        finished_at = self.finished_at or datetime.now(timezone.utc)
        return max(0, int((finished_at - self.started_at).total_seconds() * 1000))

    def to_dict(self) -> dict[str, object]:
        payload: dict[str, object] = {
            "timestamp": self.started_at.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"),
            "schema_version": self.schema_version,
            "event_version": self.event_version,
            "event_id": self.event_id,
            "request_id": self.params.request_id or self.request_id,
            "service": self.service,
            "event": self.params.event or self.params.name,
            "kind": self.params.kind,
            "level": self.params.level,
        }
        if self.params.trace_id or self.trace_id:
            payload["trace_id"] = self.params.trace_id or self.trace_id
        if self.params.span_id or self.span_id:
            payload["span_id"] = self.params.span_id or self.span_id
        if self.params.version:
            payload["version"] = self.params.version
        if self.params.environment:
            payload["environment"] = self.params.environment
        if self.params.region:
            payload["region"] = self.params.region
        if self.params.deployment_id:
            payload["deployment_id"] = self.params.deployment_id
        if self.params.message:
            payload["message"] = self.params.message
        if self.outcome:
            payload["outcome"] = self.outcome
            payload["duration_ms"] = self.duration_ms()
        if self.params.method:
            payload["method"] = self.params.method
        if self.params.path:
            payload["path"] = self.params.path
        if self.params.route:
            payload["route"] = self.params.route
        if self.params.host:
            payload["host"] = self.params.host
        if self.params.status_code:
            payload["status_code"] = self.params.status_code

        http_payload = dict(self.http)
        if self.params.method:
            http_payload.setdefault("method", self.params.method)
        if self.params.path:
            http_payload.setdefault("path", self.params.path)
        if self.params.route:
            http_payload.setdefault("route", self.params.route)
        if self.params.host:
            http_payload.setdefault("host", self.params.host)
        if self.params.status_code:
            http_payload.setdefault("status_code", self.params.status_code)
        if http_payload:
            payload["http"] = http_payload
        if self.user:
            payload["user"] = dict(self.user)
        if self.tenant:
            payload["tenant"] = dict(self.tenant)
        if self.resource:
            payload["resource"] = dict(self.resource)
        if self.attrs:
            payload["attrs"] = dict(self.attrs)
        if self.error:
            payload["error"] = dict(self.error)
        if self.pii:
            payload["pii"] = dict(self.pii)
        if self.checkpoints:
            payload["checkpoints"] = list(self.checkpoints)
        if self.processes:
            payload["process"] = list(self.processes)
        if self.groups:
            payload["groups"] = list(self.groups)
        if self.timers:
            payload["timers"] = list(self.timers)
        if self.partial:
            payload["partial"] = True
            payload["partial_reason"] = self.partial_reason
        if self.event_state:
            if self.event_state == EVENT_EMITTING and self.finished_at is not None:
                payload["event_state"] = EVENT_FINISHED
            else:
                payload["event_state"] = self.event_state
        if self.delivery_attempts:
            payload["delivery_attempts"] = self.delivery_attempts
        return payload

    def id(self) -> str:
        return self.event_id

    def is_finished(self) -> bool:
        return self.finished_at is not None

    def is_emitted(self) -> bool:
        return self.emitted

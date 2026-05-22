from __future__ import annotations

import json
import sys
import time
from dataclasses import replace
from datetime import datetime, timezone
from typing import Any

from .config import (
    CanonicalWins,
    Config,
    ErrorOnDuplicate,
    FirstWins,
    KeepBoth,
    LastWins,
    UserWins,
)
from .errors import EventClosedError, EventValidationError
from .event import (
    EVENT_DELIVERY_FAILED,
    EVENT_EMITTED,
    EVENT_EMITTING,
    EVENT_FINISHED,
    EVENT_FAILED_VALIDATION,
    Attr,
    EventContext,
    Params,
)
from .pipeline import MemoryOfflineBuffer, Pipeline
from .redactor import default_redactor
from .schema import DefaultSchema
from ..sinks.httpbatch import HTTPBatchSink
from ..generated.spec_contract import ALLOWED_KINDS, ALLOWED_LEVELS, validate_event_payload


class Logger:
    def __init__(self, config: Config) -> None:
        config.validate()
        self._config = config
        self._pipeline: Pipeline | None = None
        self._metrics = config.metrics
        self._stats_handler = config.stats_handler
        self._install_default_collector_sink()
        if not self._config.sinks and self._config.environment != "test":
            self._config = replace(self._config, sinks=Config.dev(config.service).sinks)
        if self._config.sampler is None:
            self._config.sampler = lambda event: True
        if self._config.redactor is None:
            self._config.redactor = default_redactor()
        if self._config.schema is None:
            self._config.schema = DefaultSchema()
        if self._config.async_config.enabled and self._config.sinks:
            self._pipeline = Pipeline(
                self._config.sinks,
                queue_size=self._config.async_config.queue_size,
                max_batch_bytes=self._config.async_config.max_batch_bytes,
                offline_buffer=MemoryOfflineBuffer(self._config.async_config.queue_size),
                metrics=self._metrics,
            )
            self._pipeline.start(self._config.async_config.workers)

    def _install_default_collector_sink(self) -> None:
        endpoint = self._config.collector_endpoint.strip()
        if not endpoint:
            return
        if any(isinstance(sink, HTTPBatchSink) for sink in self._config.sinks):
            return
        if not self._config.sinks or all(self._is_default_terminal_sink(sink) for sink in self._config.sinks):
            self._config = replace(self._config, sinks=[HTTPBatchSink(endpoint, api_key=self._config.api_key, service=self._config.service)])

    @staticmethod
    def _is_default_terminal_sink(sink: object) -> bool:
        return sink.__class__.__name__ in {"StdoutSink", "StderrSink"}

    def start_event(self, params: Params) -> EventContext:
        params = replace(params, custom=list(params.custom))
        if not params.service and self._config.service:
            params.service = self._config.service
        if not params.version and self._config.version:
            params.version = self._config.version
        if not params.environment and self._config.environment:
            params.environment = self._config.environment
        if not params.region and self._config.region:
            params.region = self._config.region
        if not params.deployment_id and self._config.deployment_id:
            params.deployment_id = self._config.deployment_id
        if self._config.include_host and not params.host:
            import socket
            params.host = socket.gethostname()
        ctx = EventContext(service=params.service or self._config.service, params=params)
        ctx._max_checkpoints = self._config.max_checkpoints
        for attr in params.custom:
            self._apply_attr(ctx, attr)
        if params.user_id:
            self.merge(ctx, "user", id=params.user_id)
        if params.tenant_id:
            self.merge(ctx, "tenant", id=params.tenant_id)
        if params.workspace_id:
            self.merge(ctx, "tenant", workspace_id=params.workspace_id)
        if self._config.checkpoint_emit_immediately:
            ctx._checkpoint_writer = self._emit_checkpoint_immediate
        if self._metrics is not None:
            self._metrics.on_event_created()
        return ctx

    def enrich(self, ctx: EventContext, *attrs: Attr, **named: Any) -> None:
        ctx._ensure_mutable()
        ctx._mark_active()
        for attr in attrs:
            self._apply_attr(ctx, attr)
        for key, value in named.items():
            self._apply_attr(ctx, Attr(key, value))

    def append(self, ctx: EventContext, *attrs: Attr, **named: Any) -> None:
        self.enrich(ctx, *attrs, **named)

    def _apply_attr(self, ctx: EventContext, attr: Attr) -> None:
        if attr.key.startswith("user."):
            self._set_path(ctx.user, attr.key.removeprefix("user."), attr.value, self._config.duplicate_policy)
            return
        if attr.key.startswith("tenant."):
            self._set_path(ctx.tenant, attr.key.removeprefix("tenant."), attr.value, self._config.duplicate_policy)
            return
        if attr.key.startswith("resource."):
            self._set_path(ctx.resource, attr.key.removeprefix("resource."), attr.value, self._config.duplicate_policy)
            return
        if attr.key.startswith("http."):
            self._set_path(ctx.http, attr.key.removeprefix("http."), attr.value, self._config.duplicate_policy)
            return
        self._set_path(ctx.attrs, attr.key, attr.value, self._config.duplicate_policy)

    def set(self, ctx: EventContext, *attrs: Attr, **named: Any) -> None:
        self.enrich(ctx, *attrs, **named)

    def merge(self, ctx: EventContext, group: str, *attrs: Attr, **named: Any) -> None:
        ctx._ensure_mutable()
        ctx._mark_active()
        target = self._target_map(ctx, group)
        for attr in attrs:
            self._set_path(target, attr.key, attr.value, self._config.duplicate_policy)
        for key, value in named.items():
            self._set_path(target, key, value, self._config.duplicate_policy)

    def _legacy_enrich(self, ctx: EventContext, **attrs: Any) -> None:
        for key, value in attrs.items():
            self._set_path(ctx.attrs, key, value)

    def delete(self, ctx: EventContext, *keys: str) -> None:
        ctx._ensure_mutable()
        ctx._mark_active()
        for key in keys:
            self._delete_path(ctx.attrs, key)

    def finish(self, ctx: EventContext, outcome: str, *attrs: Attr, **named: Any) -> None:
        ctx.finish(outcome)
        if self._metrics is not None:
            self._metrics.on_event_finished()
        if attrs or named:
            self.enrich(ctx, *attrs, **named)

    def finish_error(self, ctx: EventContext, error: Exception, *attrs: Attr, **named: Any) -> None:
        ctx.finish_error(error)
        if self._metrics is not None:
            self._metrics.on_event_finished()
        if attrs or named:
            self.enrich(ctx, *attrs, **named)

    def emit(self, ctx: EventContext) -> str:
        if self._config.panic_recovery:
            try:
                return self._emit_inner(ctx)
            except BaseException:
                ctx.event_state = EVENT_DELIVERY_FAILED
                ctx.emitted = False
                raise
        return self._emit_inner(ctx)

    def _emit_inner(self, ctx: EventContext) -> str:
        if ctx.event_state == EVENT_EMITTED or ctx.emitted:
            return ctx.emitted_payload
        if ctx.event_state in {EVENT_EMITTING, EVENT_DELIVERY_FAILED, EVENT_FAILED_VALIDATION}:
            raise EventClosedError(f"event {ctx.event_id} is closed in state {ctx.event_state}")

        emit_start = time.monotonic()
        try:
            self._validate(ctx)

            if not self._config.sampler(ctx):
                ctx.event_state = EVENT_EMITTED
                ctx.emitted = True
                ctx.emitted_payload = ""
                self._notify_drop("sampled_out")
                return ""

            ctx.event_state = EVENT_EMITTING
            ctx.delivery_attempts += 1
            self._apply_duplicate_field_policy(ctx)
            self._enforce_attr_limit(ctx)
            payload = self._config.schema.encode(ctx)
            payload = self._config.redactor(payload)
            encoded = json.dumps(payload, separators=(",", ":"), sort_keys=True)
            if len(encoded.encode("utf-8")) > self._config.security.max_event_bytes:
                if self._config.security.drop_oversized_events:
                    ctx.event_state = EVENT_EMITTED
                    ctx.emitted = True
                    ctx.emitted_payload = ""
                    self._notify_drop("oversized_event")
                    return ""
                raise EventValidationError("event exceeds max_event_bytes")
            if self._pipeline is not None:
                accepted = self._pipeline.try_enqueue(encoded)
                if not accepted and self._pipeline.offline_buffer is None:
                    self._notify_drop("queue_full")
                    raise EventValidationError("async pipeline backpressure drop")
            else:
                for sink in self._config.sinks:
                    sink.write(encoded)
            ctx.emitted = True
            ctx.event_state = EVENT_EMITTED
            ctx.emitted_payload = encoded
            self._notify_emit(ctx)
            return encoded
        except EventValidationError:
            ctx.event_state = EVENT_FAILED_VALIDATION
            ctx.emitted = False
            self._notify_emit_failure(ctx)
            raise
        except ValueError as exc:
            ctx.event_state = EVENT_FAILED_VALIDATION
            ctx.emitted = False
            self._notify_emit_failure(ctx)
            raise EventValidationError(str(exc)) from exc
        except Exception as exc:
            ctx.event_state = EVENT_DELIVERY_FAILED
            ctx.emitted = False
            self._notify_delivery_failed(ctx, exc)
            raise
        finally:
            if self._metrics is not None:
                self._metrics.observe_emit_duration(time.monotonic() - emit_start)

    def flush(self, timeout: float = 30.0) -> None:
        if self._pipeline is not None:
            self._pipeline.drain_once()
            if self._pipeline.offline_buffer is not None:
                buffered = self._pipeline.offline_buffer.drain(limit=4096)
                if buffered:
                    self._pipeline._write_batch(buffered)
        for sink in self._config.sinks:
            flush = getattr(sink, "flush", None)
            if callable(flush):
                flush()

    def shutdown(self, timeout: float = 30.0) -> None:
        if self._pipeline is not None:
            self._pipeline.close(timeout=timeout)
            self._pipeline = None
        else:
            self.flush(timeout=timeout)
        for sink in self._config.sinks:
            close = getattr(sink, "close", None)
            if callable(close):
                close()

    def debug(self, message: str, **attrs: Any) -> str:
        return self._emit_immediate("debug", message, **attrs)

    def info(self, message: str, **attrs: Any) -> str:
        return self._emit_immediate("info", message, **attrs)

    def warn(self, message: str, **attrs: Any) -> str:
        return self._emit_immediate("warn", message, **attrs)

    def error(self, message: str, **attrs: Any) -> str:
        return self._emit_immediate("error", message, **attrs)

    def fatal(self, message: str, **attrs: Any) -> str:
        return self._emit_immediate("fatal", message, **attrs)

    def _emit_immediate(self, level: str, message: str, **attrs: Any) -> str:
        ctx = self.start_event(Params(event=f"log.{level}", kind="log", message=message, level=level))
        self.enrich(ctx, **attrs)
        self.finish(ctx, "success")
        return self.emit(ctx)

    def _emit_checkpoint_immediate(self, ctx: EventContext, checkpoint: dict[str, object]) -> None:
        try:
            now = datetime.now(timezone.utc)
            params = Params(
                event=f"checkpoint.{checkpoint['name']}",
                kind="checkpoint",
                message=str(checkpoint["name"]),
                level=ctx.params.level,
                service=ctx.params.service,
                version=ctx.params.version,
                environment=ctx.params.environment,
                region=ctx.params.region,
                request_id=ctx.params.request_id or ctx.request_id,
                trace_id=ctx.params.trace_id or ctx.trace_id,
                span_id=ctx.params.span_id or ctx.span_id,
            )
            snapshot = EventContext(service=ctx.service, params=params)
            snapshot.started_at = ctx.started_at
            snapshot.finished_at = now
            snapshot.event_state = EVENT_FINISHED
            snapshot.attrs = {
                key: value for key, value in checkpoint.items() if key not in {"name", "at_ms"}
            }
            payload = self._config.schema.encode(snapshot)
            if isinstance(payload, dict):
                payload.setdefault("duration_ms", checkpoint.get("at_ms", 0))
            payload = self._config.redactor(payload)
            encoded = json.dumps(payload, separators=(",", ":"), sort_keys=True)
            if self._pipeline is not None:
                self._pipeline.write_sync(encoded)
            else:
                for sink in self._config.sinks:
                    sink.write(encoded)
        except Exception as exc:
            print(f"[loxa] checkpoint emit error: {exc}", file=sys.stderr)

    def _validate(self, ctx: EventContext) -> None:
        if not self._config.strict:
            return
        payload = ctx.to_dict()
        try:
            validate_event_payload(payload, strict=True)
        except ValueError as exc:
            raise EventValidationError(str(exc)) from exc

    def _apply_duplicate_field_policy(self, ctx: EventContext) -> None:
        from .canonical import is_canonical
        policy = self._config.duplicate_policy
        keys_to_remove: list[str] = []
        for key in list(ctx.attrs.keys()):
            if not is_canonical(key):
                continue
            if policy in {CanonicalWins, FirstWins}:
                keys_to_remove.append(key)
            elif policy in {UserWins, LastWins}:
                pass  # user attr overwrites canonical — already in attrs
            elif policy == KeepBoth:
                ctx.attrs[f"attrs.{key}"] = ctx.attrs.pop(key)
            elif policy == ErrorOnDuplicate:
                raise EventValidationError(f"duplicate canonical field: {key}")
        for key in keys_to_remove:
            del ctx.attrs[key]

    def _notify_emit(self, ctx: EventContext) -> None:
        if self._metrics is not None:
            self._metrics.on_event_emitted(True)
        if self._stats_handler is not None:
            try:
                self._stats_handler.on_emit(ctx)
            except Exception:
                pass

    def _notify_emit_failure(self, ctx: EventContext) -> None:
        if self._metrics is not None:
            self._metrics.on_event_emitted(False)

    def _notify_drop(self, reason: str) -> None:
        if self._metrics is not None:
            self._metrics.on_event_dropped(reason)
        if self._stats_handler is not None:
            try:
                self._stats_handler.on_drop(reason)
            except Exception:
                pass

    def _notify_delivery_failed(self, ctx: EventContext, error: Exception) -> None:
        if self._metrics is not None:
            self._metrics.on_event_emitted(False)
        if self._stats_handler is not None:
            try:
                self._stats_handler.on_error(error)
            except Exception:
                pass
            from .config import DeliveryFailureHandler as _DFH
            if isinstance(self._stats_handler, _DFH):
                try:
                    self._stats_handler.on_delivery_failed(ctx, error)
                except Exception:
                    pass

    def _target_map(self, ctx: EventContext, group: str) -> dict[str, Any]:
        if group == "user":
            return ctx.user
        if group == "tenant":
            return ctx.tenant
        if group == "resource":
            return ctx.resource
        if group == "http":
            return ctx.http
        if group == "attrs":
            return ctx.attrs
        child = ctx.attrs.get(group)
        if not isinstance(child, dict):
            child = {}
            ctx.attrs[group] = child
        return child

    def _enforce_attr_limit(self, ctx: EventContext) -> None:
        max_attr_count = self._config.security.max_attr_count
        if max_attr_count <= 0:
            return

        buckets: list[dict[str, Any]] = [ctx.attrs, ctx.user, ctx.tenant, ctx.resource, ctx.http]
        total = sum(len(bucket) for bucket in buckets)
        if total <= max_attr_count:
            return

        remaining = max_attr_count
        truncated = False
        for bucket in buckets:
            if remaining <= 0:
                if bucket:
                    bucket.clear()
                    truncated = True
                continue
            if len(bucket) > remaining:
                kept = list(bucket.items())[:remaining]
                bucket.clear()
                bucket.update(kept)
                truncated = True
            remaining -= len(bucket)

        if truncated and "_truncated" not in ctx.attrs:
            ctx.attrs["_truncated"] = True

    @staticmethod
    def _set_path(target: dict[str, Any], key: str, value: Any, policy: str = LastWins) -> None:
        if "." not in key:
            Logger._apply_duplicate(target, key, value, policy)
            return
        parts = key.split(".")
        current = target
        for part in parts[:-1]:
            child = current.get(part)
            if not isinstance(child, dict):
                child = {}
                current[part] = child
            current = child
        Logger._apply_duplicate(current, parts[-1], value, policy)

    @staticmethod
    def _apply_duplicate(target: dict[str, Any], key: str, value: Any, policy: str) -> None:
        if key not in target:
            target[key] = value
            return
        if policy in {CanonicalWins, FirstWins}:
            return
        if policy in {UserWins, LastWins}:
            target[key] = value
            return
        if policy == KeepBoth:
            old = target[key]
            target[key] = old + [value] if isinstance(old, list) else [old, value]
            return
        if policy == ErrorOnDuplicate:
            raise ValueError(f"duplicate field: {key}")
        target[key] = value

    @staticmethod
    def _delete_path(target: dict[str, Any], key: str) -> None:
        if "." not in key:
            target.pop(key, None)
            return
        parts = key.split(".")
        current = target
        for part in parts[:-1]:
            child = current.get(part)
            if not isinstance(child, dict):
                return
            current = child
        current.pop(parts[-1], None)

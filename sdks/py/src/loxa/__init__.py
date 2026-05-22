from __future__ import annotations

from dataclasses import replace

from .core.attr import *  # exports Any, String, Int, etc.
from .core.config import *  # noqa: F403
from .core.config_options import *
from .core.context import FromContext, HasEvent, EventID, RequestIDFromContext, TraceIDFromContext, SpanIDFromContext
from .core.duplicate_policy import *
from .core.event import Attr, EventContext, Params
from .core.errors import DuplicateEmitError, EventAlreadyFinishedError, EventClosedError, EventValidationError
from .core.level import *
from .core.logger import Logger
from .core.redactor import *
from .core.sampler import *
from .core.schema import *
from .metrics import *
from .core.standard_sinks import StderrSink, RotatingFileSink, CollectorSink
from .sinks import FileSink, HTTPBatchSink, MemorySink, NoopSink, StdoutSink
from .cortex import CortexClient, GraphView, IncidentContext, Remediation, RemediationFeedback
from .core.timing import ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle

# Restore loxa.Any after star-imports shadow it with typing.Any
from .core.attr import Any as _loxa_Any  # noqa: E402
Any = _loxa_Any  # noqa: F811

# ---------------------------------------------------------------------------
# Default logger instance
# ---------------------------------------------------------------------------
_default: Logger = Logger(load_layered_config())


# ---------------------------------------------------------------------------
# Lowercase Pythonic facade — delegates to _default
# ---------------------------------------------------------------------------
def configure(config: Config) -> Logger:
    global _default
    from .core.config import new_client
    _default = new_client(config)
    return _default


def default() -> Logger:
    return _default


def new(config: Config) -> Logger:
    from .core.config import new_client
    return new_client(config)


def try_new(config: Config) -> Logger:
    return new(config)


def new_client(config: Config) -> Logger:
    from .core.config import new_client as _new_client
    return _new_client(config)


def create_loxa(service: str = "", **kwargs: Any) -> Logger:
    """Create a new Logger instance. Alias for new()."""
    cfg = load_layered_config()
    if service:
        cfg = cfg.with_service(service)
    for k, v in kwargs.items():
        cfg = getattr(cfg, f'with_{k}')(v)
    return new(cfg)


def alias(service: str) -> Logger:
    """Create a new Logger with the same config as _default but different service."""
    return _default.alias(service)


def dev(service: str = "") -> Config:
    return Config.dev(service)


def production(service: str = "") -> Config:
    return Config.production(service)


def test(service: str = "") -> Config:
    return Config.test(service)


def start_event(params: Params) -> EventContext:
    return _default.start_event(replace(params))


def start_http_event(params: Params) -> EventContext:
    return start_event(replace(params, kind=params.kind or "http", event=params.event or "http.request"))


def start_job_event(params: Params) -> EventContext:
    return start_event(replace(params, kind=params.kind or "job", event=params.event or "job.run"))


def start_queue_event(params: Params) -> EventContext:
    return start_event(replace(params, kind=params.kind or "queue", event=params.event or "queue.process"))


def start_cli_event(params: Params) -> EventContext:
    return start_event(replace(params, kind=params.kind or "cli", event=params.event or "cli.run"))


def start_cron_event(params: Params) -> EventContext:
    return start_event(replace(params, kind=params.kind or "cron", event=params.event or "cron.tick"))


def start_event_from(parent: EventContext | None, params: Params) -> EventContext:
    """Create a child event that inherits trace/span/request IDs from parent."""
    params = replace(params)
    if parent is not None and isinstance(parent, EventContext):
        if not params.request_id:
            params.request_id = parent.params.request_id or parent.request_id
        if not params.trace_id:
            params.trace_id = parent.params.trace_id or parent.trace_id
        if not params.span_id:
            params.span_id = parent.params.span_id or parent.span_id
        if not params.service:
            params.service = parent.service
    return _default.start_event(params)


def append(ctx: EventContext, *attrs: Attr, **fields: Any) -> None:
    _default.append(ctx, *attrs, **fields)


def enrich(ctx: EventContext, *attrs: Attr, **fields: Any) -> None:
    _default.enrich(ctx, *attrs, **fields)


def set(ctx: EventContext, *attrs: Attr, **fields: Any) -> None:
    _default.set(ctx, *attrs, **fields)


def merge(ctx: EventContext, group: str, *attrs: Attr, **fields: Any) -> None:
    _default.merge(ctx, group, *attrs, **fields)


def delete(ctx: EventContext, *keys: str) -> None:
    _default.delete(ctx, *keys)


def get(ctx: EventContext, key: str) -> Any:
    return _default.get(ctx, key)


def get_group(ctx: EventContext, name: str) -> dict[str, Any] | None:
    return _default.get_group(ctx, name)


def checkpoint(ctx: EventContext, name: str, *attrs: Attr, **fields: Any) -> None:
    _default.checkpoint(ctx, name, *attrs, **fields)


def process(ctx: EventContext, name: str, *attrs: Attr, **fields: Any) -> "ProcessHandle":
    return ctx.start_process(name, **fields)


def start_timer(ctx: EventContext, name: str, *attrs: Attr, **fields: Any) -> "TimerHandle":
    return ctx.start_timer(name, **fields)


def start_group(ctx: EventContext, name: str, *attrs: Attr, **fields: Any) -> "GroupHandle":
    return ctx.start_group(name, **fields)


def stopwatch() -> "StopwatchHandle":
    from .core.timing import StopwatchHandle
    return StopwatchHandle()


def finish(ctx: EventContext, outcome: str, *attrs: Attr, **fields: Any) -> None:
    _default.finish(ctx, outcome, *attrs, **fields)


def finish_error(ctx: EventContext, error: Exception, *attrs: Attr, **fields: Any) -> None:
    _default.finish_error(ctx, error, *attrs, **fields)


def emit(ctx: EventContext) -> str:
    return _default.emit(ctx)


def emit_event(event: EventContext) -> str:
    return _default.emit(event)


def flush(timeout: float | None = None) -> None:
    _default.flush()


def shutdown(timeout: float | None = None) -> None:
    _default.shutdown()


def debug(message: str, **attrs: Any) -> str:
    return _default.debug(message, **attrs)


def info(message: str, **attrs: Any) -> str:
    return _default.info(message, **attrs)


def warn(message: str, **attrs: Any) -> str:
    return _default.warn(message, **attrs)


def error(message: str, **attrs: Any) -> str:
    return _default.error(message, **attrs)


def fatal(message: str, **attrs: Any) -> str:
    return _default.fatal(message, **attrs)


# ---------------------------------------------------------------------------
# Go-style uppercase aliases — import loxa as logger; logger.Enrich(...)
# ---------------------------------------------------------------------------
Configure = configure
Default = default
New = new
TryNew = try_new
NewClient = new_client
Dev = dev
Production = production
Test = test

StartEvent = start_event_from  # backward compat: StartEvent(parent_ctx, params)
StartHTTPEvent = lambda ctx, params: start_event_from(ctx, replace(params, kind="http"))
StartJobEvent = lambda ctx, params: start_event_from(ctx, replace(params, kind="job"))
StartQueueEvent = lambda ctx, params: start_event_from(ctx, replace(params, kind="queue"))
StartCLIEvent = lambda ctx, params: start_event_from(ctx, replace(params, kind="cli"))
StartCronEvent = lambda ctx, params: start_event_from(ctx, replace(params, kind="cron"))

Append = append
Enrich = enrich
Set = set
Merge = merge
Delete = delete
Get = get
GetGroup = get_group
Checkpoint = checkpoint
Process = process
StartTimer = start_timer
StartGroup = start_group
Stopwatch = stopwatch
Finish = finish
FinishError = finish_error
Emit = emit
EmitEvent = emit_event
CreateLoxa = create_loxa
Alias = alias
Flush = flush
Shutdown = shutdown

Debug = debug
Info = info
Warn = warn
Error = error
Fatal = fatal


# ---------------------------------------------------------------------------
# Attribute helpers — canonical LOXA keys
# ---------------------------------------------------------------------------
def UserID(value: str) -> Attr:
    return String("user.id", value)


def TenantID(value: str) -> Attr:
    return String("tenant.id", value)


def WorkspaceID(value: str) -> Attr:
    return String("tenant.workspace_id", value)


def OrganizationID(value: str) -> Attr:
    return String("tenant.organization_id", value)


def SessionID(value: str) -> Attr:
    return String("session.id", value)


def RequestID(value: str) -> Attr:
    return String("request_id", value)


def TraceID(value: str) -> Attr:
    return String("trace_id", value)


def SpanID(value: str) -> Attr:
    return String("span_id", value)


def FeatureFlag(name: str, value: Any) -> Attr:
    return Any(f"feature.{name}", value)


def FeatureFlagBool(name: str, value: bool) -> Attr:
    return Bool(f"feature.{name}", value)


def Experiment(name: str, variant: str) -> Attr:
    return String(f"experiment.{name}", variant)


def OrderID(value: str) -> Attr:
    return String("order.id", value)


def CartID(value: str) -> Attr:
    return String("cart.id", value)


def ProductID(value: str) -> Attr:
    return String("product.id", value)


def CustomerID(value: str) -> Attr:
    return String("customer.id", value)


def Plan(value: str) -> Attr:
    return String("customer.plan", value)


def Currency(value: str) -> Attr:
    return String("payment.currency", value)


def Amount(value: Any) -> Attr:
    return Any("payment.amount", value)


def Country(value: str) -> Attr:
    return String("geo.country", value)


def Device(value: str) -> Attr:
    return String("device.name", value)


def Platform(value: str) -> Attr:
    return String("device.platform", value)


def AppVersion(value: str) -> Attr:
    return String("app.version", value)


def ErrorType(value: str) -> Attr:
    return String("error.type", value)


def ErrorCode(value: str) -> Attr:
    return String("error.code", value)


def ErrorMessage(value: str) -> Attr:
    return String("error.message", value)


def ErrorStack(value: str) -> Attr:
    return String("error.stack", value)


def Retryable(value: bool) -> Attr:
    return Bool("error.retryable", value)


# ---------------------------------------------------------------------------
# Schema, sampler, redactor factories
# ---------------------------------------------------------------------------
def OTelSchema():
    return OTelLogSchema()


def CustomSchema(fn):
    return custom_schema(fn)


def SampleAll():
    return sample_all()


def SampleNone():
    return sample_none()


def SampleRandom(rate: float):
    return sample_random(rate)


def SampleErrors():
    return sample_errors()


def SampleSlowRequests(duration):
    return sample_slow_requests(duration)


def SampleStatusCodes(*codes: int):
    return sample_status_codes(*codes)


def SampleRoutes(*routes: str):
    return sample_routes(*routes)


def SampleUsers(*ids: str):
    return sample_users(*ids)


def SampleTenants(*ids: str):
    return sample_tenants(*ids)


def SampleFeatureFlag(name: str, value):
    return sample_feature_flag(name, value)


def SampleByHeader(header: str, value: str):
    return sample_by_header(header, value)


def AnySampler(*samplers):
    return any_sampler(*samplers)


def AllSampler(*samplers):
    return all_sampler(*samplers)


def NotSampler(sampler):
    return not_sampler(sampler)


def SampleRateLimited(rate: float, window: float = 1.0):
    return sample_rate_limited(rate, window)


def DefaultRedactor():
    return default_redactor()


def RedactKeys(*keys: str):
    return redact_keys(*keys)


def HashKeys(*keys: str):
    return hash_keys(*keys)


def MaskKeys(*keys: str, prefix: int = 2, suffix: int = 2):
    return mask_keys(*keys, prefix=prefix, suffix=suffix)


def DropKeys(*keys: str):
    return drop_keys(*keys)


def RedactPatterns(*patterns: str):
    return redact_patterns(*patterns)


def ComposeRedactors(*redactors):
    return compose_redactors(*redactors)


def NewMetricsCollector(namespace: str = "loxa_sdk", *, buffer_capacity: int = 0):
    return MetricsCollector(namespace=namespace, buffer_capacity=buffer_capacity)


def RenderPrometheus(metrics):
    return metrics.render_prometheus()


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------
__all__ = [
    # Versions
    "LOXA_SPEC_VERSION", "LOXA_INGEST_API_VERSION", "LOXA_EVENT_VERSION",
    # Errors
    "DuplicateEmitError", "EventAlreadyFinishedError", "EventClosedError", "EventValidationError",
    # Core types
    "Attr", "EventContext", "Params", "Logger", "Config", "AsyncConfig", "SecurityConfig", "FieldNamingConfig",
    # Policies
    "CanonicalWins", "UserWins", "FirstWins", "LastWins", "KeepBoth", "ErrorOnDuplicate",
    "ExpandDotKeys", "PreserveDotKeys", "SnakeCaseKeys", "CamelCaseKeys",
    # Levels
    "LevelDebug", "LevelInfo", "LevelWarn", "LevelError", "LevelFatal", "ParseLevel",
    # Lowercase facade
    "configure", "default", "new", "dev", "production", "test",
    "start_event", "start_http_event", "start_job_event", "start_queue_event", "start_cli_event", "start_cron_event",
    "start_event_from",
    "append", "enrich", "set", "merge", "delete", "get", "get_group",
    "checkpoint", "finish", "finish_error", "emit", "emit_event",
    "flush", "shutdown",
    "debug", "info", "warn", "error", "fatal",
    # Uppercase aliases
    "Configure", "Default", "New", "Dev", "Production", "Test",
    "TryNew", "NewClient",
    "StartEvent", "StartHTTPEvent", "StartJobEvent", "StartQueueEvent", "StartCLIEvent", "StartCronEvent",
    "Append", "Enrich", "Set", "Merge", "Delete", "Get", "GetGroup",
    "Checkpoint", "Finish", "FinishError", "Emit", "EmitEvent",
    "Flush", "Shutdown",
    "Debug", "Info", "Warn", "Error", "Fatal",
    # Attr constructors
    "String", "Int", "Int64", "Uint64", "Float64", "Bool", "Time", "Duration", "Any", "Null", "Group",
    "SensitiveString", "HashString", "MarkSensitive",
    # Canonical helpers
    "UserID", "TenantID", "WorkspaceID", "OrganizationID", "SessionID", "RequestID", "TraceID", "SpanID",
    "FeatureFlag", "FeatureFlagBool", "Experiment",
    "OrderID", "CartID", "ProductID", "CustomerID", "Plan", "Currency", "Amount", "Country", "Device", "Platform", "AppVersion",
    "ErrorType", "ErrorCode", "ErrorMessage", "ErrorStack", "Retryable",
    # Schema
    "Schema", "SchemaFunc", "EventView", "DefaultSchema", "FlatSchema", "NestedSchema", "ECSchema", "OTelLogSchema", "OTelSchema", "DatadogSchema", "CustomSchema",
    # Sampler
    "SampleAll", "SampleNone", "SampleRandom", "SampleErrors", "SampleSlowRequests", "SampleStatusCodes", "SampleRoutes", "SampleUsers", "SampleTenants", "SampleFeatureFlag", "SampleByHeader", "SampleRateLimited", "AnySampler", "AllSampler", "NotSampler",
    "sample_rate_limited",
    # Redactor
    "DefaultRedactor", "RedactKeys", "RedactPatterns", "HashKeys", "MaskKeys", "DropKeys", "ComposeRedactors",
    # Metrics
    "MetricsCollector", "MetricsSnapshot", "NewMetricsCollector", "RenderPrometheus",
    # Sinks
    "StdoutSink", "StderrSink", "FileSink", "RotatingFileSink", "MemorySink", "NoopSink", "CollectorSink", "HTTPBatchSink",
    # Config options
    "WithService", "WithVersion", "WithEnvironment", "WithSink", "WithSampler", "WithRedactor", "WithMetrics", "WithSchema", "WithEventSchema", "WithAsync", "WithCollectorEndpoint", "WithDuplicatePolicy", "WithStatsHandler", "WithDeploymentID", "WithIncludeHost", "WithPanicRecovery", "WithExitOnFatal",
    # Context helpers
    "FromContext", "HasEvent", "EventID", "RequestIDFromContext", "TraceIDFromContext", "SpanIDFromContext",
    # Cortex
    "CortexClient", "IncidentContext", "GraphView", "Remediation", "RemediationFeedback",
]

from __future__ import annotations

from collections.abc import Callable
from dataclasses import replace

from .core import (  # noqa: F403
    String, Int, Int64, Uint64, Float64, Bool, Time, Duration,
    Null, Group, SensitiveString, HashString, MarkSensitive,
    CanonicalWins, UserWins, FirstWins, LastWins, KeepBoth, ErrorOnDuplicate,
    ExpandDotKeys, PreserveDotKeys, SnakeCaseKeys, CamelCaseKeys,
    AsyncConfig, SecurityConfig, FieldNamingConfig, Config,
    LOXA_EVENT_VERSION, LOXA_INGEST_API_VERSION, LOXA_SPEC_VERSION,
    load_layered_config, new_client as _new_client_core,
    WithService, WithVersion, WithEnvironment, WithSink, WithSampler,
    WithRedactor, WithMetrics, WithSchema, WithEventSchema, WithAsync,
    WithCollectorEndpoint, WithDuplicatePolicy, WithStatsHandler,
    WithDeploymentID, WithIncludeHost, WithPanicRecovery, WithExitOnFatal,
    WithRelease, WithNamespace, WithApiKey, WithOtelBridge, WithRetry,
    WithTimeout, WithQueueSize, WithLogger,
    Disabled, FromEnv,
    with_service, with_version, with_environment, with_sink, with_sampler,
    with_redactor, with_metrics, with_schema, with_event_schema, with_async,
    with_collector_endpoint, with_duplicate_policy, with_stats_handler,
    with_deployment_id, with_include_host, with_panic_recovery,
    with_exit_on_fatal, with_release, with_namespace, with_api_key,
    with_otel_bridge, with_retry, with_timeout, with_queue_size, with_logger,
    disabled, from_env,
    StatsHandler, DeliveryFailureHandler,
    FromContext, HasEvent, EventID, RequestIDFromContext, TraceIDFromContext, SpanIDFromContext,
    Attr, Params, EventContext, Logger,
    DuplicateEmitError, EventAlreadyFinishedError, EventClosedError, EventValidationError,
    LevelDebug, LevelInfo, LevelNotice, LevelWarn, LevelError, LevelFatal, ParseLevel,
    # domain helpers (snake_case)
    payment_id, subscription_id, invoice_id, job_id, message_id,
    correlation_id, commit_sha, release,
    money, percent, bytes_attr, http_status, status_code, error_code,
    bucket, tags, masked, url, email_hash, ip_hash, region,
    checkout_cart_item_count, checkout_cart_total, checkout_payment_method,
    checkout_status, payment_provider, payment_method, payment_intent_id,
    payment_failure_code, payment_retry_attempt,
    billing_plan, billing_subscription_id, billing_invoice_id,
    billing_amount, billing_interval,
    agent_name, agent_provider, agent_model, agent_run_type,
    agent_tool_name, agent_tool_outcome, agent_input_tokens,
    agent_output_tokens, agent_cost,
    rag_index, rag_embedding_model, rag_chunks_retrieved, rag_top_score,
    rag_query_hash, rag_citation_count, rag_retrieval_latency,
    # PascalCase aliases
    PaymentID, SubscriptionID, InvoiceID, JobID, MessageID, CorrelationID,
    CommitSHA, Release, Money, Percent, Bytes, HTTPStatus, StatusCode,
    Bucket, Tags, Masked, URL, EmailHash, IPHash, Region,
    CheckoutCartItemCount, CheckoutCartTotal, CheckoutPaymentMethod,
    CheckoutStatus, PaymentProvider, PaymentMethod, PaymentIntentID,
    PaymentFailureCode, PaymentRetryAttempt,
    BillingPlan, BillingSubscriptionID, BillingInvoiceID, BillingAmount,
    BillingInterval, AgentName, AgentProvider, AgentModel, AgentRunType,
    AgentToolName, AgentToolOutcome, AgentInputTokens, AgentOutputTokens,
    AgentCost, RAGIndex, RAGEmbeddingModel, RAGChunksRetrieved,
    RAGTopScore, RAGQueryHash, RAGCitationCount, RAGRetrievalLatency,
    default_redactor, redact_keys, hash_keys, mask_keys, drop_keys,
    redact_patterns, compose_redactors,
    sample_all, sample_none, sample_random, sample_errors,
    sample_slow_requests, sample_status_codes, sample_routes,
    sample_users, sample_tenants, sample_feature_flag, sample_by_header,
    any_sampler, all_sampler, not_sampler, sample_rate_limited,
    sample_by_event, sample_by_outcome, should_sample,
    allow_fields, block_fields,
    SampleByEvent, SampleByOutcome, ShouldSample, AllowFields, BlockFields,
    Schema, SchemaFunc, EventView, DefaultSchema, FlatSchema, NestedSchema,
    ECSchema, OTelLogSchema, DatadogSchema, CallableSchema, custom_schema,
    StderrSink, RotatingFileSink, CollectorSink,
)

from .sinks import (
    FileSink, HTTPBatchSink, MemorySink, NoopSink, StdoutSink,
    MultiSink, multi_sink, drain, pause, resume, queue_size, health, otlp_sink,
    MultiSinkFactory, Drain, Pause, Resume, QueueSize, Health, OTLPSink,
)
from .core.http_client import CollectorClient
from .cortex import CortexClient, GraphView, IncidentContext, Remediation, RemediationFeedback
from .core.timing import (
    ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle,
    with_process, with_group, with_timer, finish_group_error, measure, step, phase, span,
)
from .testkit import (  # noqa: F403
    TestLogger, Capture, AssertEvent, AssertRedacted, AssertHasCheckpoint,
    DecodeEvents, CapturingLogger, expect_event, expect_attr,
    snapshot_event, mock_sink, fake_clock, set_id_generator,
)

# Re-export AttrAny as loxa.Any (hides typing.Any at top level)
from .core import AttrAny as _loxa_Any, MetricsCollector, MetricsSnapshot
Any = _loxa_Any

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
    return _new_client_core(config)


def create_loxa(service: str = "", **kwargs: Any) -> Logger:
    """Create a new Logger instance. Alias for new()."""
    cfg = load_layered_config()
    if service:
        cfg = cfg.with_service(service)
    for k, v in kwargs.items():
        cfg = getattr(cfg, f'with_{k}')(v)
    return new(cfg)


def alias(name: str) -> Logger:
    """Create a same-config Logger that emits loxa.alias metadata."""
    return _default.alias(name)


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


def notice(message: str, **attrs: Any) -> str:
    return _default.notice(message, **attrs)


def event(name: str, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def track(name: str, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def audit(name: str, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def security(name: str, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="warn", message=name))
    _default.enrich(ctx, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def metric(name: str, value: Any, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, metric_value=value, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def count(name: str, value: int = 1, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, metric_kind="count", count=value, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def gauge(name: str, value: float, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, metric_kind="gauge", gauge=value, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def histogram(name: str, value: float, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="event", level="info", message=name))
    _default.enrich(ctx, metric_kind="histogram", histogram_value=value, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


def breadcrumb(name: str, **attrs: Any) -> str:
    ctx = _default.start_event(Params(event=name, kind="checkpoint", level="debug", message=name))
    _default.enrich(ctx, **attrs)
    _default.finish(ctx, "success")
    return _default.emit(ctx)


# ---------------------------------------------------------------------------
# Lifecycle extras
# ---------------------------------------------------------------------------
def drop(ctx: EventContext, reason: str = "dropped", *attrs: Attr, **named: Any) -> None:
    _default.drop(ctx, reason, *attrs, **named)


def cancel(ctx: EventContext, reason: str = "cancelled", *attrs: Attr, **named: Any) -> None:
    _default.cancel(ctx, reason, *attrs, **named)


def abandon(ctx: EventContext, reason: str = "abandoned", *attrs: Attr, **named: Any) -> None:
    _default.abandon(ctx, reason, *attrs, **named)


def retry(ctx: EventContext, *attrs: Attr, **named: Any) -> None:
    _default.retry(ctx, *attrs, **named)


def partial(ctx: EventContext, reason: str = "partial", *attrs: Attr, **named: Any) -> None:
    _default.partial(ctx, reason, *attrs, **named)


def clone_event(ctx: EventContext) -> EventContext:
    return _default.clone_event(ctx)


def link_event(parent: EventContext, child: EventContext) -> None:
    _default.link_event(parent, child)


def current_event() -> EventContext | None:
    return _default.current_event()


def bind_event(ctx: EventContext) -> Logger:
    return _default.bind_event(ctx)


def wrap(ctx: EventContext, fn: Callable[..., Any], *args: Any, **kwargs: Any) -> Any:
    return _default.wrap(ctx, fn, *args, **kwargs)


# ---------------------------------------------------------------------------
# Go-style uppercase aliases — import loxa as logger; logger.Enrich(...)
# ---------------------------------------------------------------------------
Configure = configure
Default = default
New = new
TryNew = try_new
NewClient = _new_client_core
Dev = dev
Production = production
Test = test

StartEvent = start_event_from  # backward compat: StartEvent(parent_ctx, params)
def StartHTTPEvent(ctx, params):
    return start_event_from(ctx, replace(params, kind="http"))
def StartJobEvent(ctx, params):
    return start_event_from(ctx, replace(params, kind="job"))
def StartQueueEvent(ctx, params):
    return start_event_from(ctx, replace(params, kind="queue"))
def StartCLIEvent(ctx, params):
    return start_event_from(ctx, replace(params, kind="cli"))
def StartCronEvent(ctx, params):
    return start_event_from(ctx, replace(params, kind="cron"))

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
Notice = notice
Warn = warn
Error = error
Fatal = fatal

Event = event
Track = track
Audit = audit
Security = security
Metric = metric
Count = count
Gauge = gauge
Histogram = histogram
Breadcrumb = breadcrumb

Drop = drop
Cancel = cancel
Abandon = abandon
Retry = retry
Partial = partial
CloneEvent = clone_event
LinkEvent = link_event
CurrentEvent = current_event
BindEvent = bind_event
Wrap = wrap


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
    "LevelDebug", "LevelInfo", "LevelNotice", "LevelWarn", "LevelError", "LevelFatal", "ParseLevel",
    # Lowercase facade
    "configure", "default", "new", "create_loxa", "alias", "dev", "production", "test",
    "start_event", "start_http_event", "start_job_event", "start_queue_event", "start_cli_event", "start_cron_event",
    "start_event_from",
    "append", "enrich", "set", "merge", "delete", "get", "get_group",
    "checkpoint", "finish", "finish_error", "emit", "emit_event",
    "flush", "shutdown",
    "debug", "info", "notice", "warn", "error", "fatal",
    "event", "track", "audit", "security", "metric", "count", "gauge", "histogram", "breadcrumb",
    "drop", "cancel", "abandon", "retry", "partial",
    "clone_event", "link_event", "current_event", "bind_event", "wrap",
    # Uppercase aliases
    "Configure", "Default", "New", "Dev", "Production", "Test",
    "TryNew", "NewClient", "CreateLoxa", "Alias",
    "StartEvent", "StartHTTPEvent", "StartJobEvent", "StartQueueEvent", "StartCLIEvent", "StartCronEvent",
    "Append", "Enrich", "Set", "Merge", "Delete", "Get", "GetGroup",
    "Checkpoint", "Finish", "FinishError", "Emit", "EmitEvent",
    "Flush", "Shutdown",
    "Debug", "Info", "Notice", "Warn", "Error", "Fatal",
    "Event", "Track", "Audit", "Security", "Metric", "Count", "Gauge", "Histogram", "Breadcrumb",
    "Drop", "Cancel", "Abandon", "Retry", "Partial",
    "CloneEvent", "LinkEvent", "CurrentEvent", "BindEvent", "Wrap",
    # Attr constructors
    "String", "Int", "Int64", "Uint64", "Float64", "Bool", "Time", "Duration", "Any", "Null", "Group",
    "SensitiveString", "HashString", "MarkSensitive",
    # Canonical helpers
    "UserID", "TenantID", "WorkspaceID", "OrganizationID", "SessionID", "RequestID", "TraceID", "SpanID",
    "FeatureFlag", "FeatureFlagBool", "Experiment",
    "OrderID", "CartID", "ProductID", "CustomerID", "Plan", "Currency", "Amount", "Country", "Device", "Platform", "AppVersion",
    "ErrorType", "ErrorCode", "ErrorMessage", "ErrorStack", "Retryable",
    # Domain helpers
    "payment_id", "subscription_id", "invoice_id", "job_id", "message_id", "correlation_id",
    "commit_sha", "release",
    "PaymentID", "SubscriptionID", "InvoiceID", "JobID", "MessageID", "CorrelationID",
    "CommitSHA", "Release",
    "money", "percent", "bytes_attr", "http_status", "status_code", "error_code",
    "bucket", "tags", "masked", "url", "email_hash", "ip_hash", "region",
    "Money", "Percent", "Bytes", "HTTPStatus", "StatusCode", "ErrorCode",
    "Bucket", "Tags", "Masked", "URL", "EmailHash", "IPHash", "Region",
    # Domain packs
    "checkout_cart_item_count", "checkout_cart_total", "checkout_payment_method", "checkout_status",
    "payment_provider", "payment_method", "payment_intent_id", "payment_failure_code", "payment_retry_attempt",
    "billing_plan", "billing_subscription_id", "billing_invoice_id", "billing_amount", "billing_interval",
    "agent_name", "agent_provider", "agent_model", "agent_run_type", "agent_tool_name", "agent_tool_outcome",
    "agent_input_tokens", "agent_output_tokens", "agent_cost",
    "rag_index", "rag_embedding_model", "rag_chunks_retrieved", "rag_top_score", "rag_query_hash",
    "rag_citation_count", "rag_retrieval_latency",
    "CheckoutCartItemCount", "CheckoutCartTotal", "CheckoutPaymentMethod", "CheckoutStatus",
    "PaymentProvider", "PaymentMethod", "PaymentIntentID", "PaymentFailureCode", "PaymentRetryAttempt",
    "BillingPlan", "BillingSubscriptionID", "BillingInvoiceID", "BillingAmount", "BillingInterval",
    "AgentName", "AgentProvider", "AgentModel", "AgentRunType", "AgentToolName", "AgentToolOutcome",
    "AgentInputTokens", "AgentOutputTokens", "AgentCost",
    "RAGIndex", "RAGEmbeddingModel", "RAGChunksRetrieved", "RAGTopScore", "RAGQueryHash",
    "RAGCitationCount", "RAGRetrievalLatency",
    # Schema
    "Schema", "SchemaFunc", "EventView", "DefaultSchema", "FlatSchema", "NestedSchema", "ECSchema", "OTelLogSchema", "OTelSchema", "DatadogSchema", "CustomSchema",
    # Sampler
    "SampleAll", "SampleNone", "SampleRandom", "SampleErrors", "SampleSlowRequests", "SampleStatusCodes", "SampleRoutes", "SampleUsers", "SampleTenants", "SampleFeatureFlag", "SampleByHeader", "SampleRateLimited", "AnySampler", "AllSampler", "NotSampler",
    "sample_rate_limited",
    "SampleByEvent", "SampleByOutcome", "ShouldSample", "AllowFields", "BlockFields",
    "sample_by_event", "sample_by_outcome", "should_sample", "allow_fields", "block_fields",
    # Redactor
    "DefaultRedactor", "RedactKeys", "RedactPatterns", "HashKeys", "MaskKeys", "DropKeys", "ComposeRedactors",
    # Metrics
    "MetricsCollector", "MetricsSnapshot", "NewMetricsCollector", "RenderPrometheus",
    # Sinks
    "StdoutSink", "StderrSink", "FileSink", "RotatingFileSink", "MemorySink", "NoopSink", "CollectorSink", "HTTPBatchSink",
    "MultiSink", "multi_sink", "MultiSinkFactory", "otlp_sink", "OTLPSink",
    "drain", "Drain", "pause", "Pause", "resume", "Resume", "queue_size", "QueueSize", "health", "Health",
    # Config options
    "WithService", "WithVersion", "WithEnvironment", "WithSink", "WithSampler", "WithRedactor", "WithMetrics", "WithSchema", "WithEventSchema", "WithAsync", "WithCollectorEndpoint", "WithDuplicatePolicy", "WithStatsHandler", "WithDeploymentID", "WithIncludeHost", "WithPanicRecovery", "WithExitOnFatal",
    "WithRelease", "WithNamespace", "WithApiKey", "WithOtelBridge", "WithRetry", "WithTimeout", "WithQueueSize", "WithLogger",
    "with_service", "with_version", "with_environment", "with_sink", "with_sampler", "with_redactor",
    "with_metrics", "with_schema", "with_event_schema", "with_async", "with_collector_endpoint",
    "with_duplicate_policy", "with_stats_handler", "with_deployment_id", "with_include_host",
    "with_panic_recovery", "with_exit_on_fatal",
    "with_release", "with_namespace", "with_api_key", "with_otel_bridge", "with_retry", "with_timeout", "with_queue_size", "with_logger",
    "Disabled", "disabled", "FromEnv", "from_env",
    # Timing
    "ProcessHandle", "TimerHandle", "GroupHandle", "StopwatchHandle",
    "with_process", "with_group", "with_timer", "finish_group_error",
    "measure", "step", "phase", "span",
    # Context helpers
    "FromContext", "HasEvent", "EventID", "RequestIDFromContext", "TraceIDFromContext", "SpanIDFromContext",
    # Testkit
    "TestLogger", "Capture", "AssertEvent", "AssertRedacted", "AssertHasCheckpoint", "DecodeEvents", "CapturingLogger",
    "expect_event", "expect_attr", "snapshot_event", "mock_sink", "fake_clock", "set_id_generator",
    # Collector
    "CollectorClient",
    # Cortex
    "CortexClient", "IncidentContext", "GraphView", "Remediation", "RemediationFeedback",
]

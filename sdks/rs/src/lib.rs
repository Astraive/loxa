#![allow(non_snake_case, non_upper_case_globals)]

mod config;
pub mod dsn;
mod errors;
mod event;
mod generated;
mod logger;
mod metrics;
mod redaction;
mod sampling;
mod schema;
mod sink;

pub mod core;
pub mod cortex;
pub mod integrations;
pub mod internal;
pub mod loxa;
pub mod middleware;
pub mod packages;
pub mod sinks;
pub mod storagepath;
pub mod testkit;
pub mod utils;

use serde_json::Value;
use std::sync::{Arc, OnceLock, RwLock, RwLockReadGuard, RwLockWriteGuard};

pub use crate::core::client::{
    extract_http_headers, inject_http_headers, CollectorHttpClient, HTTPClient, HTTPRequest,
    HTTPResponse,
};
pub use config::{
    AsyncConfig, BackpressurePolicy, Config, DeliveryFailureHandler, MemorySinkStore,
    RedactorConfig, SamplerConfig, SchemaConfig, SecurityConfig, SinkConfig, StatsHandler,
};
pub use cortex::{CortexClient, GraphView, IncidentContext, Remediation, RemediationFeedback};
pub use errors::LoxaError;
pub use event::{
    Attr, ContextCarrier, ContextSource, EventContext, GroupHandle, Params, ProcessHandle,
    StopwatchHandle, TimerHandle,
};
pub use generated::spec_contract::{
    LOXA_EVENT_VERSION, LOXA_INGEST_API_VERSION, LOXA_SPEC_VERSION,
};
// Logger is intentionally NOT re-exported. Use loxa::default(),
// loxa::create_loxa(), or loxa::alias() instead.
use logger::Logger;
pub use metrics::{MetricsCollector, MetricsSnapshot};
pub use schema::{DefaultSchemaType, EventView, Schema, SchemaFunc};

pub const LevelDebug: &str = "debug";
pub const LevelInfo: &str = "info";
pub const LevelWarn: &str = "warn";
pub const LevelError: &str = "error";
pub const LevelNotice: &str = "notice";
pub const LevelFatal: &str = "fatal";

pub const CanonicalWins: &str = "canonical_wins";
pub const UserWins: &str = "user_wins";
pub const FirstWins: &str = "first_wins";
pub const LastWins: &str = "last_wins";
pub const KeepBoth: &str = "keep_both";
pub const ErrorOnDuplicate: &str = "error_on_duplicate";
pub const AttrsWin: &str = UserWins;

pub const ExpandDotKeys: &str = "expand_dot_keys";
pub const PreserveDotKeys: &str = "preserve_dot_keys";
pub const SnakeCaseKeys: &str = "snake_case_keys";
pub const CamelCaseKeys: &str = "camel_case_keys";

// --- Logger factories ---

pub fn New(config: Config) -> Logger {
    Logger::new(config)
}

pub fn NewWith(options: Vec<core::options::ConfigOption>) -> Logger {
    Logger::new(core::options::apply(Config::base(), options))
}

pub fn TryNew(config: Config) -> Result<Logger, LoxaError> {
    Logger::try_new(config)
}

pub fn NewClient(config: Config) -> Logger {
    Logger::new(config)
}

pub fn Configure(config: Config) -> Logger {
    Logger::new(config)
}

/// Install a logger as the global default and return it.
pub fn configure(config: Config) -> Result<Logger, LoxaError> {
    let logger = Logger::try_new(config)?;
    let lock = GLOBAL_LOGGER.get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))));
    let mut guard = write_global_logger(lock);
    // Shut down the previous logger before replacing it
    let _ = guard.shutdown();
    *guard = logger.clone();
    std::mem::drop(guard);
    Ok(logger)
}

pub fn reset() -> Result<Logger, LoxaError> {
    configure(Config::dev("loxa"))
}

pub fn Reset() -> Result<Logger, LoxaError> {
    reset()
}

/// Return the global default logger. Always succeeds (dev default if not configured).
pub fn default() -> Logger {
    let lock = GLOBAL_LOGGER.get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))));
    read_global_logger(lock).clone()
}

pub fn Default(service: impl Into<String>) -> Logger {
    Logger::new(Config::base().with_service(service))
}

pub fn Dev(service: impl Into<String>) -> Logger {
    Logger::new(Config::dev(service))
}

pub fn Production(service: impl Into<String>) -> Logger {
    Logger::new(Config::production(service))
}

pub fn Test(service: impl Into<String>) -> Logger {
    Logger::new(Config::test(service))
}

// --- Event starters ---

pub fn StartEvent(parent: Option<&dyn ContextSource>, params: Params) -> EventContext {
    let params = if let Some(parent) = parent {
        parent.inherit_params(params)
    } else {
        params
    };
    default_logger().start_event(params)
}

pub fn StartHTTPEvent(method: &str, path: &str) -> Params {
    core::event::params_from_http(method, path, None)
}

pub fn StartJobEvent(job: impl Into<String>) -> Params {
    Params::new(job).with_kind("job")
}

pub fn StartQueueEvent(queue: impl Into<String>) -> Params {
    Params::new(queue).with_kind("queue")
}

pub fn StartCLIEvent(command: impl Into<String>) -> Params {
    Params::new(command).with_kind("cli")
}

pub fn StartCronEvent(cron: impl Into<String>) -> Params {
    Params::new(cron).with_kind("cron")
}

// --- Event manipulation (delegated to Logger) ---
// These are implemented as free functions that use the global logger.

static GLOBAL_LOGGER: OnceLock<RwLock<Logger>> = OnceLock::new();

fn default_logger() -> Logger {
    let lock = GLOBAL_LOGGER.get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))));
    read_global_logger(lock).clone()
}

pub fn set_global_logger(logger: Logger) {
    let lock = GLOBAL_LOGGER.get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))));
    *write_global_logger(lock) = logger;
}

fn read_global_logger(lock: &RwLock<Logger>) -> RwLockReadGuard<'_, Logger> {
    match lock.read() {
        Ok(guard) => guard,
        Err(poisoned) => poisoned.into_inner(),
    }
}

fn write_global_logger(lock: &RwLock<Logger>) -> RwLockWriteGuard<'_, Logger> {
    match lock.write() {
        Ok(guard) => guard,
        Err(poisoned) => poisoned.into_inner(),
    }
}

pub fn Append(event: &mut EventContext, attr: Attr) {
    default_logger().append(event, attr);
}

pub fn Enrich(event: &mut EventContext, attrs: impl IntoIterator<Item = Attr>) {
    let logger = default_logger();
    for attr in attrs {
        logger.append(event, attr);
    }
}

pub fn Set(event: &mut EventContext, key: impl Into<String>, value: impl Into<Value>) {
    default_logger().append(event, Attr::new(key, value));
}

pub fn Merge(event: &mut EventContext, map: serde_json::Map<String, Value>) {
    let logger = default_logger();
    for (key, value) in map {
        logger.append(event, Attr::new(key, value));
    }
}

pub fn Delete(event: &mut EventContext, key: &str) {
    event.attrs.remove(key);
}

pub fn Get<'a>(event: &'a EventContext, key: &str) -> Option<&'a Value> {
    event.attrs.get(key)
}

pub fn GetGroup<'a>(event: &'a EventContext, group: &str) -> Option<&'a Value> {
    event.attrs.get(group)
}

pub fn Checkpoint(event: &mut EventContext, name: impl Into<String>) {
    event.checkpoint(&name.into());
}

pub fn CheckpointWithAttrs(event: &mut EventContext, name: impl Into<String>, attrs: &[Attr]) {
    event.checkpoint_with_attrs(&name.into(), attrs);
}

pub fn Finish(event: &mut EventContext) {
    let _ = default_logger().finish(event, "success");
}

pub fn FinishError(event: &mut EventContext, err: impl std::fmt::Display) {
    let _ = default_logger().finish_error(event, err.to_string());
}

pub fn Emit(event: &mut EventContext) -> Result<(), LoxaError> {
    default_logger().emit(event).map(|_| ())
}

pub fn EmitEvent(event: &mut EventContext) -> Result<(), LoxaError> {
    Emit(event)
}

pub fn Flush() {
    let _ = default_logger().flush();
}

pub fn Shutdown() {
    let _ = default_logger().shutdown();
}

/// Create a new Logger instance with the given config.
pub fn create_loxa(config: Config) -> Logger {
    Logger::new(config)
}

/// Create a same-config Logger that emits loxa.alias metadata.
pub fn alias(name: impl Into<String>) -> Logger {
    default_logger().alias(name)
}

pub fn CreateLoxa(config: Config) -> Logger {
    create_loxa(config)
}

pub fn Alias(service: impl Into<String>) -> Logger {
    alias(service)
}

// --- Convenience loggers ---

pub fn Debug(message: impl Into<String>) {
    let _ = default_logger().debug(message);
}

pub fn Info(message: impl Into<String>) {
    let _ = default_logger().info(message);
}

pub fn Warn(message: impl Into<String>) {
    let _ = default_logger().warn(message);
}

pub fn Error(message: impl Into<String>) {
    let _ = default_logger().error(message);
}

pub fn Fatal(message: impl Into<String>) -> ! {
    let _ = default_logger().fatal(message);
    Flush();
    std::process::exit(1)
}

/// Emit a fatal-level event without exiting the process.
pub fn fatal_event(message: impl Into<String>) -> Result<String, LoxaError> {
    default_logger().fatal(message)
}

// --- Context helpers ---

pub fn FromContext(ctx: &EventContext) -> Option<&EventContext> {
    core::event::from_context(ctx)
}

pub fn HasEvent(ctx: &EventContext) -> bool {
    core::event::has_event(ctx)
}

pub fn EventID(ctx: &EventContext) -> Option<String> {
    Some(ctx.event_id.clone())
}

pub fn RequestIDFromContext(ctx: &EventContext) -> Option<String> {
    Some(ctx.request_id.clone())
}

pub fn TraceIDFromContext(ctx: &EventContext) -> Option<String> {
    ctx.trace_id.clone()
}

pub fn SpanIDFromContext(ctx: &EventContext) -> Option<String> {
    ctx.span_id.clone()
}

pub fn InjectHTTPHeaders(request: &mut HTTPRequest, event: &EventContext) {
    inject_http_headers(request, event);
}

pub fn InjectHTTPHeadersFromCarrier(
    carrier: &ContextCarrier,
    headers: &mut std::collections::BTreeMap<String, String>,
) {
    if let Some(traceparent) = carrier.traceparent() {
        headers.insert("traceparent".to_string(), traceparent);
    }
    if let Some(tracestate) = &carrier.tracestate {
        headers.insert("tracestate".to_string(), tracestate.clone());
    }
    if let Some(request_id) = &carrier.request_id {
        headers.insert("x-request-id".to_string(), request_id.clone());
    }
    if !carrier.baggage.is_empty() {
        let parts: Vec<String> = carrier
            .baggage
            .iter()
            .map(|(k, v)| format!("{k}={v}"))
            .collect();
        headers.insert("baggage".to_string(), parts.join(","));
    }
}

pub fn ExtractHTTPHeaders(
    headers: &std::collections::BTreeMap<String, String>,
) -> (Option<String>, Option<String>, Option<String>) {
    extract_http_headers(headers)
}

// --- Attribute constructors ---

pub fn String(key: impl Into<String>, value: impl Into<std::string::String>) -> Attr {
    Attr::new(key, serde_json::Value::String(value.into()))
}

pub fn Int(key: impl Into<String>, value: i64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn Int64(key: impl Into<String>, value: i64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn Uint64(key: impl Into<String>, value: u64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn Float64(key: impl Into<String>, value: f64) -> Attr {
    Attr::new(
        key,
        serde_json::Number::from_f64(value)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

/// PascalCase alias for Float64 — cross-SDK parity.
pub fn Float(key: impl Into<String>, value: f64) -> Attr {
    Float64(key, value)
}

pub fn Bool(key: impl Into<String>, value: bool) -> Attr {
    Attr::new(key, Value::Bool(value))
}

pub fn Time(key: impl Into<String>, value: time::OffsetDateTime) -> Attr {
    let formatted = value
        .format(&time::format_description::well_known::Rfc3339)
        .unwrap_or_default();
    Attr::new(key, Value::String(formatted))
}

pub fn Duration(key: impl Into<String>, value: std::time::Duration) -> Attr {
    Attr::new(key, Value::Number((value.as_millis() as u64).into()))
}

pub fn Any(key: impl Into<String>, value: impl serde::Serialize) -> Attr {
    let v = serde_json::to_value(value).unwrap_or(Value::Null);
    Attr::new(key, v)
}

pub fn Null(key: impl Into<String>) -> Attr {
    Attr::new(key, Value::Null)
}

pub fn Group(name: impl Into<String>, attrs: impl IntoIterator<Item = Attr>) -> Attr {
    core::event::group(name, attrs)
}

pub fn SensitiveString(key: impl Into<String>, value: impl Into<String>) -> Attr {
    core::event::sensitive_string(key, value)
}

pub fn MarkSensitive(attr: Attr) -> Attr {
    attr.sensitive()
}

pub fn HashString(key: impl Into<String>, value: impl Into<String>) -> Attr {
    core::event::hash_string(key, value)
}

// --- Domain attribute constructors ---

pub fn UserID(id: impl Into<String>) -> Attr {
    Attr::new("user.id", Value::String(id.into()))
}

pub fn TenantID(id: impl Into<String>) -> Attr {
    Attr::new("tenant.id", Value::String(id.into()))
}

pub fn RequestID(id: impl Into<String>) -> Attr {
    Attr::new("request_id", Value::String(id.into()))
}

pub fn TraceID(id: impl Into<String>) -> Attr {
    Attr::new("trace_id", Value::String(id.into()))
}

pub fn SpanID(id: impl Into<String>) -> Attr {
    Attr::new("span_id", Value::String(id.into()))
}

pub fn ServiceName(name: impl Into<String>) -> Attr {
    Attr::new("service", Value::String(name.into()))
}

pub fn Environment(env: impl Into<String>) -> Attr {
    Attr::new("environment", Value::String(env.into()))
}

pub fn Region(region: impl Into<String>) -> Attr {
    Attr::new("region", Value::String(region.into()))
}

pub fn Version(version: impl Into<String>) -> Attr {
    Attr::new("version", Value::String(version.into()))
}

pub fn ErrorAttr(error: impl std::fmt::Display) -> Attr {
    Attr::new("error.message", Value::String(error.to_string()))
}

pub fn StatusCode(code: u16) -> Attr {
    Attr::new("status_code", Value::Number(code.into()))
}

pub fn Method(method: impl Into<String>) -> Attr {
    Attr::new("method", Value::String(method.into()))
}

pub fn Path(path: impl Into<String>) -> Attr {
    Attr::new("path", Value::String(path.into()))
}

pub fn Route(route: impl Into<String>) -> Attr {
    Attr::new("route", Value::String(route.into()))
}

pub fn Message(msg: impl Into<String>) -> Attr {
    Attr::new("message", Value::String(msg.into()))
}

// --- Additional domain attribute constructors ---

pub fn WorkspaceID(id: impl Into<String>) -> Attr {
    Attr::new("workspace.id", Value::String(id.into()))
}

pub fn OrganizationID(id: impl Into<String>) -> Attr {
    Attr::new("organization.id", Value::String(id.into()))
}

pub fn SessionID(id: impl Into<String>) -> Attr {
    Attr::new("session.id", Value::String(id.into()))
}

pub fn FeatureFlag(name: impl Into<String>, value: impl Into<Value>) -> Attr {
    Attr::new(format!("feature.{}", name.into()), value.into())
}

pub fn FeatureFlagBool(name: impl Into<String>, value: bool) -> Attr {
    Attr::new(format!("feature.{}", name.into()), Value::Bool(value))
}

pub fn Experiment(name: impl Into<String>, variant: impl Into<String>) -> Attr {
    Attr::new(
        format!("experiment.{}", name.into()),
        Value::String(variant.into()),
    )
}

pub fn OrderID(id: impl Into<String>) -> Attr {
    Attr::new("order.id", Value::String(id.into()))
}

pub fn CartID(id: impl Into<String>) -> Attr {
    Attr::new("cart.id", Value::String(id.into()))
}

pub fn ProductID(id: impl Into<String>) -> Attr {
    Attr::new("product.id", Value::String(id.into()))
}

pub fn CustomerID(id: impl Into<String>) -> Attr {
    Attr::new("customer.id", Value::String(id.into()))
}

pub fn Plan(name: impl Into<String>) -> Attr {
    Attr::new("customer.plan", Value::String(name.into()))
}

pub fn Currency(code: impl Into<String>) -> Attr {
    Attr::new("payment.currency", Value::String(code.into()))
}

pub fn Amount(value: f64) -> Attr {
    Attr::new(
        "payment.amount",
        serde_json::Number::from_f64(value)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn Country(code: impl Into<String>) -> Attr {
    Attr::new("geo.country", Value::String(code.into()))
}

pub fn Device(name: impl Into<String>) -> Attr {
    Attr::new("device.name", Value::String(name.into()))
}

pub fn Platform(name: impl Into<String>) -> Attr {
    Attr::new("device.platform", Value::String(name.into()))
}

pub fn AppVersion(version: impl Into<String>) -> Attr {
    Attr::new("app.version", Value::String(version.into()))
}

pub fn ErrorType(typ: impl Into<String>) -> Attr {
    Attr::new("error.type", Value::String(typ.into()))
}

pub fn ErrorCode(code: impl Into<String>) -> Attr {
    Attr::new("error.code", Value::String(code.into()))
}

pub fn ErrorMessage(msg: impl Into<String>) -> Attr {
    Attr::new("error.message", Value::String(msg.into()))
}

pub fn ErrorStack(stack: impl Into<String>) -> Attr {
    Attr::new("error.stack", Value::String(stack.into()))
}

pub fn Retryable(value: bool) -> Attr {
    Attr::new("error.retryable", Value::Bool(value))
}

// --- Domain helpers ---

pub fn PaymentID(id: impl Into<String>) -> Attr {
    Attr::new("payment.id", Value::String(id.into()))
}

pub fn SubscriptionID(id: impl Into<String>) -> Attr {
    Attr::new("subscription.id", Value::String(id.into()))
}

pub fn InvoiceID(id: impl Into<String>) -> Attr {
    Attr::new("invoice.id", Value::String(id.into()))
}

pub fn JobID(id: impl Into<String>) -> Attr {
    Attr::new("job.id", Value::String(id.into()))
}

pub fn MessageID(id: impl Into<String>) -> Attr {
    Attr::new("message.id", Value::String(id.into()))
}

pub fn CorrelationID(id: impl Into<String>) -> Attr {
    Attr::new("correlation.id", Value::String(id.into()))
}

pub fn CommitSHA(sha: impl Into<String>) -> Attr {
    Attr::new("commit.sha", Value::String(sha.into()))
}

pub fn Release(name: impl Into<String>) -> Attr {
    Attr::new("release", Value::String(name.into()))
}

pub fn Money(amount: f64) -> Attr {
    Attr::new(
        "money",
        serde_json::Number::from_f64(amount)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn Percent(value: f64) -> Attr {
    Attr::new(
        "percent",
        serde_json::Number::from_f64(value)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn Bytes(value: u64) -> Attr {
    Attr::new("bytes", Value::Number(value.into()))
}

pub fn HTTPStatus(code: u16) -> Attr {
    Attr::new("http.status_code", Value::Number(code.into()))
}

pub fn Bucket(name: impl Into<String>) -> Attr {
    Attr::new("bucket", Value::String(name.into()))
}

pub fn Tags(values: Vec<impl Into<String>>) -> Attr {
    let arr: Vec<Value> = values
        .into_iter()
        .map(|v| Value::String(v.into()))
        .collect();
    Attr::new("tags", Value::Array(arr))
}

pub fn Masked(value: impl Into<String>) -> Attr {
    Attr::new(value.into(), Value::String("[REDACTED]".to_string())).sensitive()
}

pub fn List<S: serde::Serialize>(key: impl Into<String>, values: Vec<S>) -> Attr {
    let arr: Vec<Value> = values
        .into_iter()
        .map(|v| serde_json::to_value(v).unwrap_or(Value::Null))
        .collect();
    Attr::new(key.into(), Value::Array(arr))
}

pub fn Map(key: impl Into<String>, value: serde_json::Map<String, Value>) -> Attr {
    Attr::new(key.into(), Value::Object(value))
}

pub fn Enum<E: Into<String>>(
    key: impl Into<String>,
    value: impl Into<String>,
    _allowed: Vec<E>,
) -> Attr {
    Attr::new(key.into(), Value::String(value.into()))
}

pub fn ID(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Attr::new(key.into(), Value::String(value.into()))
}

pub fn Hash(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Attr::new(key.into(), Value::String(value.into())).hash_value()
}

pub fn Redacted(key: impl Into<String>) -> Attr {
    Attr::new(key.into(), Value::String("[REDACTED]".to_string()))
}

pub fn AccountID(id: impl Into<String>) -> Attr {
    Attr::new("account.id", Value::String(id.into()))
}

pub fn URL(url: impl Into<String>) -> Attr {
    Attr::new("url", Value::String(url.into()))
}

pub fn DeploymentID(id: impl Into<String>) -> Attr {
    Attr::new("deployment.id", Value::String(id.into()))
}

pub fn HTTPRoute(route: impl Into<String>) -> Attr {
    Attr::new("http.route", Value::String(route.into()))
}

pub fn HTTPMethod(method: impl Into<String>) -> Attr {
    Attr::new("http.method", Value::String(method.into().to_uppercase()))
}

pub fn HTTPPath(path: impl Into<String>) -> Attr {
    Attr::new("http.path", Value::String(path.into()))
}

pub fn HTTPUserAgent(ua: impl Into<String>) -> Attr {
    let mut value = ua.into();
    if value.len() > 512 {
        value.truncate(512);
    }
    Attr::new("http.user_agent", Value::String(value))
}

pub fn HTTPReferer(referer: impl Into<String>) -> Attr {
    let value = referer.into().split('?').next().unwrap_or("").to_string();
    Attr::new("http.referer", Value::String(value))
}

pub fn HTTPRequest(method: impl Into<String>, path: impl Into<String>) -> Attr {
    Attr::new(
        "http.request",
        serde_json::json!({
            "method": method.into(),
            "path": path.into(),
        }),
    )
}

pub fn HTTPResponse(status_code: u16) -> Attr {
    Attr::new(
        "http.response",
        serde_json::json!({ "status_code": status_code }),
    )
}

pub fn EmailHash(email: impl Into<String>) -> Attr {
    Attr::new("email.hash", Value::String(email.into())).hash_value()
}

pub fn IPHash(ip: impl Into<String>) -> Attr {
    Attr::new("ip.hash", Value::String(ip.into())).hash_value()
}

// --- Domain pack: checkout ---

pub fn CheckoutCartItemCount(count: u32) -> Attr {
    Attr::new("checkout.cart_item_count", Value::Number(count.into()))
}

pub fn CheckoutCartTotal(total: f64) -> Attr {
    Attr::new(
        "checkout.cart_total",
        serde_json::Number::from_f64(total)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn CheckoutPaymentMethod(method: impl Into<String>) -> Attr {
    Attr::new("checkout.payment_method", Value::String(method.into()))
}

pub fn CheckoutStatus(status: impl Into<String>) -> Attr {
    Attr::new("checkout.status", Value::String(status.into()))
}

// --- Domain pack: payment ---

pub fn PaymentProvider(provider: impl Into<String>) -> Attr {
    Attr::new("payment.provider", Value::String(provider.into()))
}

pub fn PaymentMethod(method: impl Into<String>) -> Attr {
    Attr::new("payment.method", Value::String(method.into()))
}

pub fn PaymentIntentID(id: impl Into<String>) -> Attr {
    Attr::new("payment.intent_id", Value::String(id.into()))
}

pub fn PaymentFailureCode(code: impl Into<String>) -> Attr {
    Attr::new("payment.failure_code", Value::String(code.into()))
}

pub fn PaymentRetryAttempt(attempt: u32) -> Attr {
    Attr::new("payment.retry_attempt", Value::Number(attempt.into()))
}

// --- Domain pack: billing ---

pub fn BillingPlan(plan: impl Into<String>) -> Attr {
    Attr::new("billing.plan", Value::String(plan.into()))
}

pub fn BillingSubscriptionID(id: impl Into<String>) -> Attr {
    Attr::new("billing.subscription_id", Value::String(id.into()))
}

pub fn BillingInvoiceID(id: impl Into<String>) -> Attr {
    Attr::new("billing.invoice_id", Value::String(id.into()))
}

pub fn BillingAmount(amount: f64) -> Attr {
    Attr::new(
        "billing.amount",
        serde_json::Number::from_f64(amount)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn BillingInterval(interval: impl Into<String>) -> Attr {
    Attr::new("billing.interval", Value::String(interval.into()))
}

// --- Domain pack: agent ---

pub fn AgentName(name: impl Into<String>) -> Attr {
    Attr::new("agent.name", Value::String(name.into()))
}

pub fn AgentProvider(provider: impl Into<String>) -> Attr {
    Attr::new("agent.provider", Value::String(provider.into()))
}

pub fn AgentModel(model: impl Into<String>) -> Attr {
    Attr::new("agent.model", Value::String(model.into()))
}

pub fn AgentRunType(run_type: impl Into<String>) -> Attr {
    Attr::new("agent.run_type", Value::String(run_type.into()))
}

pub fn AgentToolName(name: impl Into<String>) -> Attr {
    Attr::new("agent.tool_name", Value::String(name.into()))
}

pub fn AgentToolOutcome(outcome: impl Into<String>) -> Attr {
    Attr::new("agent.tool_outcome", Value::String(outcome.into()))
}

pub fn AgentInputTokens(tokens: u64) -> Attr {
    Attr::new("agent.input_tokens", Value::Number(tokens.into()))
}

pub fn AgentOutputTokens(tokens: u64) -> Attr {
    Attr::new("agent.output_tokens", Value::Number(tokens.into()))
}

pub fn AgentCost(cost: f64) -> Attr {
    Attr::new(
        "agent.cost",
        serde_json::Number::from_f64(cost)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

// --- Domain pack: RAG ---

pub fn RAGIndex(index: impl Into<String>) -> Attr {
    Attr::new("rag.index", Value::String(index.into()))
}

pub fn RAGEmbeddingModel(model: impl Into<String>) -> Attr {
    Attr::new("rag.embedding_model", Value::String(model.into()))
}

pub fn RAGChunksRetrieved(count: u32) -> Attr {
    Attr::new("rag.chunks_retrieved", Value::Number(count.into()))
}

pub fn RAGTopScore(score: f64) -> Attr {
    Attr::new(
        "rag.top_score",
        serde_json::Number::from_f64(score)
            .map(Value::Number)
            .unwrap_or(Value::Null),
    )
}

pub fn RAGQueryHash(hash: impl Into<String>) -> Attr {
    Attr::new("rag.query_hash", Value::String(hash.into()))
}

pub fn RAGCitationCount(count: u32) -> Attr {
    Attr::new("rag.citation_count", Value::Number(count.into()))
}

pub fn RAGRetrievalLatency(latency_ms: u64) -> Attr {
    Attr::new("rag.retrieval_latency", Value::Number(latency_ms.into()))
}

// --- Lifecycle extras ---

pub fn Drop(event: &mut EventContext, reason: impl Into<String>) {
    event.outcome = Some("dropped".to_string());
    event.partial = true;
    event.partial_reason = Some(reason.into());
}

pub fn Cancel(event: &mut EventContext) {
    let _ = event.finish("cancelled");
}

pub fn Abandon(event: &mut EventContext) {
    let _ = event.finish("abandoned");
}

pub fn Retry(event: &mut EventContext) {
    let _ = event.finish("retried");
}

pub fn Partial(event: &mut EventContext, reason: impl Into<String>) {
    let _ = event.finish("partial");
    event.partial = true;
    event.partial_reason = Some(reason.into());
}

pub fn CloneEvent(event: &EventContext) -> EventContext {
    event.clone()
}

pub fn LinkEvent(event: &mut EventContext, linked_id: impl Into<String>) {
    let mut link = serde_json::Map::new();
    link.insert("event_id".to_string(), Value::String(linked_id.into()));
    if event.error.is_none() {
        event.error = Some(serde_json::Map::new());
    }
}

thread_local! {
    static CURRENT_EVENT: std::cell::RefCell<Option<EventContext>> = const { std::cell::RefCell::new(None) };
}

pub(crate) fn set_current_event(ctx: Option<EventContext>) {
    CURRENT_EVENT.with(|cell| *cell.borrow_mut() = ctx);
}

pub fn CurrentEvent() -> Option<EventContext> {
    CURRENT_EVENT.with(|cell| cell.borrow().clone())
}

pub fn BindEvent(_logger: &Logger, event: &EventContext) -> EventContext {
    let ctx = event.clone();
    set_current_event(Some(ctx.clone()));
    ctx
}

pub fn Wrap(event: &mut EventContext, f: impl FnOnce(&mut EventContext)) {
    f(event)
}

pub fn RunEvent(params: Params, f: impl FnOnce(&mut EventContext)) -> Result<String, LoxaError> {
    let logger = default_logger();
    let mut ctx = logger.start_event(params);
    f(&mut ctx);
    let _ = logger.finish(&mut ctx, "success");
    logger.emit(&ctx)
}

pub fn Run(
    event: &mut EventContext,
    f: impl FnOnce(&mut EventContext),
) -> Result<String, LoxaError> {
    let logger = default_logger();
    f(event);
    let _ = logger.finish(event, "success");
    logger.emit(event)
}

pub fn run_event(params: Params, f: impl FnOnce(&mut EventContext)) -> Result<String, LoxaError> {
    RunEvent(params, f)
}

pub fn run(
    event: &mut EventContext,
    f: impl FnOnce(&mut EventContext),
) -> Result<String, LoxaError> {
    Run(event, f)
}

// --- from_request: extract HTTP request context ---

/// Extract safe request context from HTTP request metadata and start an event.
/// Cross-SDK parity with Go's StartHTTPEventFromRequest.
pub fn FromRequest(
    method: impl Into<String>,
    path: impl Into<String>,
    route: impl Into<String>,
    _attrs: Vec<Attr>,
) -> Params {
    Params::new("http.request")
        .with_kind("http")
        .with_method(method)
        .with_path(path)
        .with_route(route)
}

/// Lowercase alias for FromRequest.
pub fn from_request(
    method: impl Into<String>,
    path: impl Into<String>,
    route: impl Into<String>,
    attrs: Vec<Attr>,
) -> Params {
    FromRequest(method, path, route, attrs)
}

// --- max_attr_length / max_attrs / cardinality_policy ---

/// Returns a SecurityConfig with max field bytes set.
pub fn MaxAttrLength(length: usize) -> SecurityConfig {
    SecurityConfig {
        max_field_bytes: length,
        ..Default::default()
    }
}

/// Lowercase alias for MaxAttrLength.
pub fn max_attr_length(length: usize) -> SecurityConfig {
    MaxAttrLength(length)
}

/// Returns a SecurityConfig with max attr count set.
pub fn MaxAttrs(count: usize) -> SecurityConfig {
    SecurityConfig {
        max_attr_count: count,
        ..Default::default()
    }
}

/// Lowercase alias for MaxAttrs.
pub fn max_attrs(count: usize) -> SecurityConfig {
    MaxAttrs(count)
}

/// Configure cardinality policy. Returns the policy map unchanged.
pub fn CardinalityPolicy(
    policy: std::collections::HashMap<String, serde_json::Value>,
) -> std::collections::HashMap<String, serde_json::Value> {
    policy
}

/// Lowercase alias for CardinalityPolicy.
pub fn cardinality_policy(
    policy: std::collections::HashMap<String, serde_json::Value>,
) -> std::collections::HashMap<String, serde_json::Value> {
    CardinalityPolicy(policy)
}

// --- Process/Group/Timer extras ---

pub fn Process(event: &mut EventContext, name: &str) -> ProcessHandle {
    event.start_process(name)
}

pub fn StartProcess(event: &mut EventContext, name: &str) -> ProcessHandle {
    event.start_process(name)
}

pub fn FinishProcess(handle: ProcessHandle, event: &mut EventContext) {
    handle.finish(event, &[]);
}

pub fn FinishProcessError(handle: ProcessHandle, event: &mut EventContext, message: &str) {
    handle.finish(event, &[Attr::new("error", message)]);
}

pub fn GroupProcess(event: &mut EventContext, name: &str) -> GroupHandle {
    event.start_group(name)
}

pub fn StartGroup(event: &mut EventContext, name: &str) -> GroupHandle {
    event.start_group(name)
}

pub fn FinishGroup(handle: GroupHandle, event: &mut EventContext) {
    handle.finish(event, &[]);
}

pub fn Timer(event: &mut EventContext, name: &str) -> TimerHandle {
    event.start_timer(name)
}

pub fn StartTimer(event: &mut EventContext, name: &str) -> TimerHandle {
    event.start_timer(name)
}

pub fn StopTimer(handle: TimerHandle, event: &mut EventContext) {
    handle.stop(event, &[]);
}

pub fn Stopwatch() -> StopwatchHandle {
    StopwatchHandle::new()
}

pub fn process(event: &mut EventContext, name: &str) -> ProcessHandle {
    Process(event, name)
}

pub fn start_process(event: &mut EventContext, name: &str) -> ProcessHandle {
    StartProcess(event, name)
}

pub fn finish_process(handle: ProcessHandle, event: &mut EventContext) {
    FinishProcess(handle, event)
}

pub fn finish_process_error(handle: ProcessHandle, event: &mut EventContext, message: &str) {
    FinishProcessError(handle, event, message)
}

pub fn group(event: &mut EventContext, name: &str) -> GroupHandle {
    event.start_group(name)
}

pub fn start_group(event: &mut EventContext, name: &str) -> GroupHandle {
    StartGroup(event, name)
}

pub fn finish_group(handle: GroupHandle, event: &mut EventContext) {
    FinishGroup(handle, event)
}

pub fn timer(event: &mut EventContext, name: &str) -> TimerHandle {
    Timer(event, name)
}

pub fn start_timer(event: &mut EventContext, name: &str) -> TimerHandle {
    StartTimer(event, name)
}

pub fn stop_timer(handle: TimerHandle, event: &mut EventContext) {
    StopTimer(handle, event)
}

pub fn stopwatch() -> StopwatchHandle {
    Stopwatch()
}

pub fn WithProcess(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(ProcessHandle, &mut EventContext),
) {
    let handle = event.start_process(name);
    f(handle, event);
}

pub fn WithGroup(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(GroupHandle, &mut EventContext),
) {
    let handle = event.start_group(name);
    f(handle, event);
}

pub fn WithTimer(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(TimerHandle, &mut EventContext),
) {
    let handle = event.start_timer(name);
    f(handle, event);
}

pub fn FinishGroupError(handle: GroupHandle, event: &mut EventContext, message: &str) {
    handle.finish(event, &[Attr::new("error", message)]);
}

pub fn Measure(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    let timer = event.start_timer(name);
    f(event);
    timer.stop(event, &[]);
}

pub fn Step(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    let process = event.start_process(name);
    f(event);
    process.finish(event, &[]);
}

pub fn Phase(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    let group = event.start_group(name);
    f(event);
    group.finish(event, &[]);
}

pub fn Span(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    Measure(event, name, f);
}

// --- Logging helpers ---

pub fn Notice(message: impl Into<String>) {
    let _ = default_logger().notice(message);
}

pub fn Event(name: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(name))
}

pub fn Track(name: impl Into<String>, attrs: &[Attr]) {
    let mut ctx = default_logger().start_event(Params::new(name).with_kind("track"));
    for attr in attrs {
        ctx.append_attr(attr.clone());
    }
    let _ = ctx.finish("success");
    let _ = default_logger().emit(&ctx);
}

pub fn Audit(name: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(name).with_kind("audit"))
}

pub fn Security(name: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(name).with_kind("security"))
}

pub fn Metric(name: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(name).with_kind("metric"))
}

pub fn Count(name: impl Into<String>, value: u64) {
    let mut event = Metric(name);
    event.append_attr(Attr::new("count", Value::Number(value.into())));
    let _ = event.finish("success");
    let _ = default_logger().emit(&event);
}

pub fn Gauge(name: impl Into<String>, value: f64) {
    let mut event = Metric(name);
    let num = serde_json::Number::from_f64(value).unwrap_or(serde_json::Number::from(0));
    event.append_attr(Attr::new("gauge", Value::Number(num)));
    let _ = event.finish("success");
    let _ = default_logger().emit(&event);
}

pub fn Histogram(name: impl Into<String>, value: f64) {
    let mut event = Metric(name);
    let num = serde_json::Number::from_f64(value).unwrap_or(serde_json::Number::from(0));
    event.append_attr(Attr::new("histogram", Value::Number(num)));
    let _ = event.finish("success");
    let _ = default_logger().emit(&event);
}

pub fn Breadcrumb(message: impl Into<String>) {
    let _ = default_logger().breadcrumb(message);
}

// --- Config extras ---

pub fn DisabledConfig() -> Config {
    Config::disabled()
}

pub fn FromEnv() -> Config {
    Config::from_env()
}

// --- Sink extras ---

pub fn MultiSink(sinks: &[SinkConfig]) -> Vec<SinkConfig> {
    sinks.to_vec()
}

pub fn OtlpSink(endpoint: impl Into<String>) -> SinkConfig {
    SinkConfig::HttpBatch {
        endpoint: endpoint.into(),
        api_key: None,
        timeout_ms: 2_000,
        max_batch_bytes: 256 * 1024,
        max_retries: 3,
        enable_compression: true,
        ndjson: false,
    }
}

pub fn Drain(sink: &SinkConfig) {
    let _ = crate::sink::flush_sink(sink);
}

pub fn Pause(sink: &SinkConfig) {
    crate::sinks::pause(sink);
}

pub fn Resume(sink: &SinkConfig) {
    crate::sinks::resume(sink);
}

pub fn QueueSize(sink: &SinkConfig) -> usize {
    crate::sinks::queue_size(sink)
}

pub fn Health(sink: &SinkConfig) -> bool {
    crate::sinks::health(sink)
}

// --- Sampling/Policy extras ---

pub fn SampleByEvent(f: impl Fn(&EventContext) -> bool + Send + Sync + 'static) -> SamplerConfig {
    SamplerConfig::Custom(Arc::new(f))
}

pub fn SampleByOutcome(outcomes: &[&str]) -> SamplerConfig {
    let outcomes: Vec<String> = outcomes.iter().map(|s| s.to_string()).collect();
    SamplerConfig::Custom(Arc::new(move |event: &EventContext| {
        event
            .outcome
            .as_deref()
            .is_some_and(|o| outcomes.iter().any(|w| w == o))
    }))
}

pub fn ShouldSample(event: &EventContext, sampler: &SamplerConfig) -> bool {
    sampling::should_sample(event, sampler)
}

pub fn AllowFields(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::AllowKeys(keys.iter().map(|k| k.to_string()).collect())
}

pub fn BlockFields(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::Keys(keys.iter().map(|k| k.to_string()).collect())
}

// --- Testing extras ---

pub fn ExpectEvent(logger: &Logger, name: &str, f: impl FnOnce(&EventContext)) {
    crate::testkit::helpers::expect_event(logger, name, f);
}

pub fn ExpectAttr(event: &EventContext, key: &str, expected: &Value) -> bool {
    event.attrs.get(key) == Some(expected)
}

pub fn snapshot_event(event: &EventContext) -> String {
    SnapshotEvent(event)
}

pub fn mock_sink() -> SinkConfig {
    MockSink()
}

pub fn assert_event(encoded: &str, key: &str, expected: &str) {
    crate::testkit::helpers::assert_event(encoded, key, expected);
}

pub fn assert_redacted(encoded: &str, key: &str) {
    crate::testkit::helpers::assert_redacted(encoded, key);
}

pub fn sanitize_event(value: Value, ctx: &EventContext) -> Value {
    SanitizeEvent(value, ctx)
}

pub fn SnapshotEvent(event: &EventContext) -> String {
    serde_json::to_string(event).unwrap_or_default()
}

pub fn MockSink() -> SinkConfig {
    SinkConfig::Memory(MemorySinkStore::new())
}

pub fn FakeClock(unix_ms: u128) {
    crate::internal::clock::freeze_at(unix_ms);
}

pub fn SetIDGenerator(f: fn() -> String) {
    crate::internal::core::uuidv7::set_id_generator(Box::new(f));
}

pub fn Testkit(service: &str) -> (Logger, MemorySinkStore) {
    crate::testkit::helpers::testkit(service)
}

pub fn SanitizeEvent(value: Value, ctx: &EventContext) -> Value {
    crate::event::apply_sensitive_to_value(value, ctx)
}

/// Validate an event map against the Loxa spec contract.
/// Returns Ok(()) if valid, Err with a list of error strings if not.
pub fn ValidateEvent(event: &Value) -> Result<(), Vec<String>> {
    let mut errors: Vec<String> = Vec::new();
    if let Some(obj) = event.as_object() {
        let has_event_id = obj.contains_key("event_id") || obj.contains_key("eventId");
        if !has_event_id {
            errors.push("missing required field: event_id".into());
        }
        if !obj.contains_key("timestamp") {
            errors.push("missing required field: timestamp".into());
        }
        if !obj.contains_key("service") {
            errors.push("missing required field: service".into());
        }
        if !obj.contains_key("event") {
            errors.push("missing required field: event".into());
        }
    } else {
        errors.push("event must be a JSON object".into());
    }
    if errors.is_empty() {
        Ok(())
    } else {
        Err(errors)
    }
}

pub fn validate_event(event: &Value) -> Result<(), Vec<String>> {
    ValidateEvent(event)
}

/// Normalize event field names from dashed/snake_case aliases to canonical camelCase.
pub fn NormalizeEvent(mut event: Value) -> Value {
    let aliases: Vec<(&str, &str)> = vec![
        ("event_id", "eventId"),
        ("schema_version", "schemaVersion"),
        ("event_version", "eventVersion"),
        ("started_at", "startedAt"),
        ("started_at_ms", "startedAtMs"),
        ("finished_at", "finishedAt"),
        ("finished_at_ms", "finishedAtMs"),
        ("duration_ms", "durationMs"),
        ("status_code", "statusCode"),
        ("deployment_id", "deploymentId"),
        ("user_id", "userId"),
        ("tenant_id", "tenantId"),
        ("session_id", "sessionId"),
        ("request_id", "requestId"),
        ("correlation_id", "correlationId"),
        ("trace_id", "traceId"),
        ("span_id", "spanId"),
        ("incident_id", "incidentId"),
        ("error_message", "errorMessage"),
        ("error_stack", "errorStack"),
        ("error_type", "errorType"),
        ("error_code", "errorCode"),
        ("order_id", "orderId"),
        ("cart_id", "cartId"),
        ("payment_id", "paymentId"),
        ("subscription_id", "subscriptionId"),
        ("invoice_id", "invoiceId"),
        ("job_id", "jobId"),
        ("message_id", "messageId"),
    ];
    if let Some(obj) = event.as_object_mut() {
        for (alias, canonical) in aliases {
            if let Some(val) = obj.remove(alias) {
                obj.entry(canonical.to_string()).or_insert(val);
            }
        }
    }
    event
}

pub fn normalize_event(event: Value) -> Value {
    NormalizeEvent(event)
}

pub fn Capture(f: impl FnOnce(&Logger)) -> Vec<String> {
    crate::testkit::helpers::capture(f)
}

pub fn ResetForTest() {
    crate::testkit::helpers::reset_for_test()
}

pub fn LastEvent(store: &MemorySinkStore) -> Option<String> {
    store.events().last().cloned()
}

pub fn Events(store: &MemorySinkStore) -> Vec<String> {
    store.events()
}

pub fn ClearEvents(store: &MemorySinkStore) {
    store.clear()
}

/// Compare an event snapshot against a golden file. Creates the file if it doesn't exist.
/// Returns `true` if the event matches the golden file (or file was newly created).
pub fn GoldenTest(path: impl Into<String>, snapshot: &str) -> bool {
    let p = std::path::PathBuf::from(path.into());
    if !p.exists() {
        if let Some(parent) = p.parent() {
            let _ = std::fs::create_dir_all(parent);
        }
        let _ = std::fs::write(&p, format!("{}\n", snapshot));
        return true;
    }
    match std::fs::read_to_string(&p) {
        Ok(expected) => expected.trim() == snapshot.trim(),
        Err(_) => false,
    }
}

pub fn ConformanceSuite() -> serde_json::Value {
    serde_json::json!({"name": "loxa-rs-conformance", "status": "available"})
}

pub fn SetClock(unix_ms: u128) {
    FakeClock(unix_ms)
}

pub fn testkit() -> (Logger, MemorySinkStore) {
    Testkit("test")
}

pub fn last_event(store: &MemorySinkStore) -> Option<String> {
    LastEvent(store)
}

pub fn events(store: &MemorySinkStore) -> Vec<String> {
    Events(store)
}

pub fn clear_events(store: &MemorySinkStore) {
    ClearEvents(store)
}

pub fn golden_test(path: impl Into<String>, snapshot: &str) -> bool {
    GoldenTest(path, snapshot)
}

pub fn conformance_suite() -> serde_json::Value {
    ConformanceSuite()
}

pub fn reset_for_test() {
    ResetForTest()
}

pub fn set_clock(unix_ms: u128) {
    SetClock(unix_ms)
}

// --- Schema constructors ---

pub fn DefaultSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::Default)
}

pub fn FlatSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::Flat)
}

pub fn NestedSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::Nested)
}

pub fn ECSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::ECS)
}

pub fn OTelSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::OTel)
}

pub fn OTelLogSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::OTel)
}

pub fn DatadogSchema() -> schema::DefaultSchemaType {
    schema::DefaultSchemaType(SchemaConfig::Datadog)
}

pub fn CustomSchema(func: schema::SchemaFunc) -> schema::SchemaFunc {
    func
}

pub fn custom_schema(schema: impl schema::Schema + Send + Sync + 'static) -> SchemaConfig {
    SchemaConfig::Custom(std::sync::Arc::new(schema))
}

// --- Encoder constructors ---

pub fn JSONEncoder() -> fn(&Value) -> Result<String, serde_json::Error> {
    serde_json::to_string
}

pub fn PrettyJSONEncoder() -> fn(&Value) -> Result<String, serde_json::Error> {
    serde_json::to_string_pretty
}

// --- Sink constructors ---

pub fn StdoutSink() -> SinkConfig {
    SinkConfig::Stdout
}

pub fn StderrSink() -> SinkConfig {
    SinkConfig::Stderr
}

pub fn FileSink(path: impl Into<String>) -> SinkConfig {
    SinkConfig::File(path.into())
}

pub fn RotatingFileSink(path: impl Into<String>) -> SinkConfig {
    SinkConfig::File(path.into())
}

pub fn MemorySink() -> SinkConfig {
    SinkConfig::Memory(MemorySinkStore::new())
}

pub fn NoopSink() -> SinkConfig {
    SinkConfig::Noop
}

pub fn CollectorSink() -> SinkConfig {
    CollectorSinkWithEndpoint("http://127.0.0.1:9308/events")
}

pub fn CollectorSinkWithEndpoint(endpoint: impl Into<String>) -> SinkConfig {
    SinkConfig::HttpBatch {
        endpoint: endpoint.into(),
        api_key: None,
        timeout_ms: 2_000,
        max_batch_bytes: 256 * 1024,
        max_retries: 3,
        enable_compression: true,
        ndjson: false,
    }
}

pub fn HttpBatchSink(endpoint: impl Into<String>) -> SinkConfig {
    CollectorSinkWithEndpoint(endpoint)
}

pub fn HTTPBatchSink(endpoint: impl Into<String>) -> SinkConfig {
    CollectorSinkWithEndpoint(endpoint)
}

pub fn KafkaSink(endpoint: impl Into<String>, _topic: impl Into<String>) -> SinkConfig {
    SinkConfig::HttpBatch {
        endpoint: endpoint.into(),
        api_key: None,
        timeout_ms: 2_000,
        max_batch_bytes: 256 * 1024,
        max_retries: 3,
        enable_compression: true,
        ndjson: false,
    }
}

pub fn kafka_sink(endpoint: impl Into<String>, topic: impl Into<String>) -> SinkConfig {
    KafkaSink(endpoint, topic)
}

// --- Sampler constructors ---

pub fn SampleAll() -> SamplerConfig {
    SamplerConfig::All
}

pub fn SampleNone() -> SamplerConfig {
    SamplerConfig::None
}

pub fn SampleRandom(rate: f64) -> SamplerConfig {
    SamplerConfig::SampleRandom(rate)
}

pub fn SampleRate(rate: f64) -> SamplerConfig {
    SamplerConfig::SampleRandom(rate)
}

pub fn SampleErrors() -> SamplerConfig {
    SamplerConfig::Errors
}

pub fn SampleSlowRequests(ms: u64) -> SamplerConfig {
    SamplerConfig::SlowRequests(ms as u128)
}

pub fn SampleStatusCodes(codes: &[u16]) -> SamplerConfig {
    SamplerConfig::StatusCodes(codes.to_vec())
}

pub fn SampleRoutes(routes: &[&str]) -> SamplerConfig {
    SamplerConfig::Routes(routes.iter().map(|s| s.to_string()).collect())
}

pub fn SampleUsers(ids: &[&str]) -> SamplerConfig {
    SamplerConfig::Users(ids.iter().map(|s| s.to_string()).collect())
}

pub fn SampleTenants(ids: &[&str]) -> SamplerConfig {
    SamplerConfig::Tenants(ids.iter().map(|s| s.to_string()).collect())
}

pub fn SampleFeatureFlag(name: &str, value: &Value) -> SamplerConfig {
    SamplerConfig::FeatureFlag(name.to_string(), value.clone())
}

pub fn AnySampler(samplers: &[SamplerConfig]) -> SamplerConfig {
    SamplerConfig::Any(samplers.to_vec())
}

pub fn AllSampler(samplers: &[SamplerConfig]) -> SamplerConfig {
    SamplerConfig::AllOf(samplers.to_vec())
}

pub fn NotSampler(sampler: SamplerConfig) -> SamplerConfig {
    SamplerConfig::Not(Box::new(sampler))
}

pub fn SampleByHeader(header: &str, value: &str) -> SamplerConfig {
    SamplerConfig::SampleByHeader(header.to_string(), value.to_string())
}

pub fn SampleRateLimited(rate: f64, window: std::time::Duration) -> SamplerConfig {
    SamplerConfig::SampleRateLimited(rate, window)
}

pub fn sample_rate_limited(rate: f64, window: std::time::Duration) -> SamplerConfig {
    SampleRateLimited(rate, window)
}

// --- Redactor constructors ---

pub fn DefaultRedactor() -> RedactorConfig {
    RedactorConfig::Default
}

pub fn Redact(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::Keys(keys.iter().map(|s| s.to_string()).collect())
}

pub fn RedactKeys(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::Keys(keys.iter().map(|s| s.to_string()).collect())
}

pub fn HashKeys(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::HashKeys(keys.iter().map(|k| k.to_string()).collect())
}

pub fn DropKeys(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::DropKeys(keys.iter().map(|k| k.to_string()).collect())
}

pub fn MaskKeys(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::MaskKeys(keys.iter().map(|k| k.to_string()).collect())
}

pub fn ComposeRedactors(redactors: &[RedactorConfig]) -> RedactorConfig {
    RedactorConfig::Compose(redactors.to_vec())
}

// --- Config builder shortcuts ---

pub fn WithService(service: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_service(service)
}

pub fn WithAlias(alias: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_alias(alias)
}

pub fn WithVersion(version: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_version(version)
}

pub fn WithEnvironment(environment: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_environment(environment)
}

pub fn WithRegion(region: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_region(region)
}

pub fn WithSink(sink: SinkConfig) -> core::options::ConfigOption {
    core::options::with_sink(sink)
}

pub fn WithSampler(sampler: SamplerConfig) -> core::options::ConfigOption {
    core::options::with_sampler(sampler)
}

pub fn WithRedactor(redactor: RedactorConfig) -> core::options::ConfigOption {
    core::options::with_redactor(redactor)
}

pub fn WithSchema(schema: SchemaConfig) -> core::options::ConfigOption {
    core::options::with_schema(schema)
}

pub fn WithEventSchema(schema: SchemaConfig) -> core::options::ConfigOption {
    core::options::with_schema(schema)
}

pub fn WithCollectorEndpoint(endpoint: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_collector_endpoint(endpoint)
}

pub fn WithDuplicatePolicy(policy: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_duplicate_policy(policy)
}

pub fn WithAsync(enabled: bool) -> core::options::ConfigOption {
    core::options::with_async(enabled)
}

pub fn WithStatsHandler(
    handler: std::sync::Arc<dyn config::StatsHandler>,
) -> core::options::ConfigOption {
    Box::new(move |cfg| cfg.with_stats_handler(handler))
}

pub fn WithDeploymentID(deployment_id: impl Into<String>) -> core::options::ConfigOption {
    let deployment_id = deployment_id.into();
    Box::new(move |cfg| cfg.with_deployment_id(deployment_id))
}

pub fn WithIncludeHost(include_host: bool) -> core::options::ConfigOption {
    Box::new(move |cfg| cfg.with_include_host(include_host))
}

pub fn WithPanicRecovery(panic_recovery: bool) -> core::options::ConfigOption {
    Box::new(move |cfg| cfg.with_panic_recovery(panic_recovery))
}

pub fn WithBatchSize(size: usize) -> core::options::ConfigOption {
    core::options::with_batch_size(size)
}

pub fn WithFlushInterval(ms: u64) -> core::options::ConfigOption {
    core::options::with_flush_interval(ms)
}

pub fn WithRetry(max_retries: u32) -> core::options::ConfigOption {
    core::options::with_retry(max_retries)
}

pub fn WithLogger(logger: Logger) -> core::options::ConfigOption {
    Box::new(move |cfg| cfg.with_logger(logger))
}

pub fn WithApiKey(api_key: impl Into<String>) -> core::options::ConfigOption {
    core::options::with_api_key(api_key)
}

pub fn RedactPatterns(patterns: &[&str]) -> RedactorConfig {
    RedactorConfig::Patterns(patterns.iter().map(|p| p.to_string()).collect())
}

pub fn render_prometheus(collector: &MetricsCollector) -> String {
    collector.render_prometheus("loxa")
}

pub fn RenderPrometheus(collector: &MetricsCollector) -> String {
    render_prometheus(collector)
}

// =============================================================================
// Lowercase snake_case aliases (Rust-idiomatic API)
// =============================================================================

// --- Logger factories ---

pub fn new_logger(config: Config) -> Logger {
    Logger::new(config)
}

pub fn new_with(options: Vec<core::options::ConfigOption>) -> Logger {
    Logger::new(core::options::apply(Config::base(), options))
}

pub fn try_new_logger(config: Config) -> Result<Logger, LoxaError> {
    Logger::try_new(config)
}

pub fn dev(service: impl Into<String>) -> Logger {
    Logger::new(Config::dev(service))
}

pub fn production(service: impl Into<String>) -> Logger {
    Logger::new(Config::production(service))
}

pub fn test(service: impl Into<String>) -> Logger {
    Logger::new(Config::test(service))
}

// --- Event starters ---

pub fn start_event(parent: Option<&dyn ContextSource>, params: Params) -> EventContext {
    StartEvent(parent, params)
}

pub fn start_http_event(method: &str, path: &str) -> EventContext {
    let params = core::event::params_from_http(method, path, None);
    default_logger().start_event(params)
}

pub fn start_job_event(job: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(job).with_kind("job"))
}

pub fn start_queue_event(queue: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(queue).with_kind("queue"))
}

pub fn start_cli_event(command: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(command).with_kind("cli"))
}

pub fn start_cron_event(cron: impl Into<String>) -> EventContext {
    default_logger().start_event(Params::new(cron).with_kind("cron"))
}

// --- Event manipulation ---

pub fn append(event: &mut EventContext, attr: Attr) {
    Append(event, attr)
}

pub fn enrich(event: &mut EventContext, attrs: impl IntoIterator<Item = Attr>) {
    Enrich(event, attrs)
}

pub fn set(event: &mut EventContext, key: impl Into<String>, value: impl Into<Value>) {
    Set(event, key, value)
}

pub fn merge(event: &mut EventContext, map: serde_json::Map<String, Value>) {
    Merge(event, map)
}

pub fn delete(event: &mut EventContext, key: &str) {
    Delete(event, key)
}

pub fn get<'a>(event: &'a EventContext, key: &str) -> Option<&'a Value> {
    Get(event, key)
}

pub fn get_group<'a>(event: &'a EventContext, group: &str) -> Option<&'a Value> {
    GetGroup(event, group)
}

pub fn checkpoint(event: &mut EventContext, name: impl Into<String>) {
    Checkpoint(event, name)
}

pub fn checkpoint_with_attrs(event: &mut EventContext, name: impl Into<String>, attrs: &[Attr]) {
    CheckpointWithAttrs(event, name, attrs)
}

pub fn finish(event: &mut EventContext) {
    Finish(event)
}

pub fn finish_error(event: &mut EventContext, err: impl std::fmt::Display) {
    FinishError(event, err)
}

pub fn emit(event: &mut EventContext) -> Result<(), LoxaError> {
    Emit(event)
}

pub fn emit_event(event: &mut EventContext) -> Result<(), LoxaError> {
    Emit(event)
}

pub fn flush() {
    Flush()
}

pub fn shutdown() {
    Shutdown()
}

// --- Convenience loggers ---

pub fn debug(message: impl Into<String>) {
    Debug(message)
}

pub fn info(message: impl Into<String>) {
    Info(message)
}

pub fn warn(message: impl Into<String>) {
    Warn(message)
}

pub fn error(message: impl Into<String>) {
    Error(message)
}

pub fn fatal(message: impl Into<String>) -> ! {
    Fatal(message)
}

// --- Generic attribute constructors ---

pub fn string(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Attr::new(key, Value::String(value.into()))
}

pub fn int(key: impl Into<String>, value: i64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn int64(key: impl Into<String>, value: i64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn uint64(key: impl Into<String>, value: u64) -> Attr {
    Attr::new(key, Value::Number(value.into()))
}

pub fn float(key: impl Into<String>, value: f64) -> Attr {
    Float64(key, value)
}

pub fn float64(key: impl Into<String>, value: f64) -> Attr {
    Float64(key, value)
}

pub fn bool(key: impl Into<String>, value: bool) -> Attr {
    Bool(key, value)
}

pub fn time(key: impl Into<String>, value: time::OffsetDateTime) -> Attr {
    Time(key, value)
}

pub fn duration(key: impl Into<String>, value: std::time::Duration) -> Attr {
    Duration(key, value)
}

pub fn any(key: impl Into<String>, value: impl serde::Serialize) -> Attr {
    Any(key, value)
}

/// Lowercase alias for Any — cross-SDK parity with Go/JS/Python `json()`.
pub fn json(key: impl Into<String>, value: impl serde::Serialize) -> Attr {
    Any(key, value)
}

pub fn null(key: impl Into<String>) -> Attr {
    Null(key)
}

pub fn group_attr(name: impl Into<String>, attrs: Vec<Attr>) -> Attr {
    Group(name, attrs)
}

pub fn sensitive_string(key: impl Into<String>, value: impl Into<String>) -> Attr {
    SensitiveString(key, value)
}

pub fn mark_sensitive(attr: Attr) -> Attr {
    MarkSensitive(attr)
}

pub fn hash_string(key: impl Into<String>, value: impl Into<String>) -> Attr {
    HashString(key, value)
}

// --- Domain attribute constructors ---

pub fn user_id(id: impl Into<String>) -> Attr {
    UserID(id)
}

pub fn tenant_id(id: impl Into<String>) -> Attr {
    TenantID(id)
}

pub fn request_id(id: impl Into<String>) -> Attr {
    RequestID(id)
}

pub fn trace_id(id: impl Into<String>) -> Attr {
    TraceID(id)
}

pub fn span_id(id: impl Into<String>) -> Attr {
    SpanID(id)
}

pub fn service_name(name: impl Into<String>) -> Attr {
    ServiceName(name)
}

pub fn environment(env: impl Into<String>) -> Attr {
    Environment(env)
}

pub fn region(region: impl Into<String>) -> Attr {
    Region(region)
}

pub fn version(version: impl Into<String>) -> Attr {
    Version(version)
}

pub fn error_attr(error: impl std::fmt::Display) -> Attr {
    ErrorAttr(error)
}

pub fn status_code(code: u16) -> Attr {
    StatusCode(code)
}

pub fn method(method: impl Into<String>) -> Attr {
    Method(method)
}

pub fn path(path: impl Into<String>) -> Attr {
    Path(path)
}

pub fn route(route: impl Into<String>) -> Attr {
    Route(route)
}

pub fn message(msg: impl Into<String>) -> Attr {
    Message(msg)
}

pub fn workspace_id(id: impl Into<String>) -> Attr {
    WorkspaceID(id)
}

pub fn organization_id(id: impl Into<String>) -> Attr {
    OrganizationID(id)
}

pub fn session_id(id: impl Into<String>) -> Attr {
    SessionID(id)
}

pub fn feature_flag(name: impl Into<String>, value: impl Into<Value>) -> Attr {
    FeatureFlag(name, value)
}

pub fn feature_flag_bool(name: impl Into<String>, value: bool) -> Attr {
    FeatureFlagBool(name, value)
}

pub fn experiment(name: impl Into<String>, variant: impl Into<String>) -> Attr {
    Experiment(name, variant)
}

pub fn order_id(id: impl Into<String>) -> Attr {
    OrderID(id)
}

pub fn cart_id(id: impl Into<String>) -> Attr {
    CartID(id)
}

pub fn product_id(id: impl Into<String>) -> Attr {
    ProductID(id)
}

pub fn customer_id(id: impl Into<String>) -> Attr {
    CustomerID(id)
}

pub fn plan(name: impl Into<String>) -> Attr {
    Plan(name)
}

pub fn currency(code: impl Into<String>) -> Attr {
    Currency(code)
}

pub fn amount(value: f64) -> Attr {
    Amount(value)
}

pub fn country(code: impl Into<String>) -> Attr {
    Country(code)
}

pub fn device(name: impl Into<String>) -> Attr {
    Device(name)
}

pub fn platform(name: impl Into<String>) -> Attr {
    Platform(name)
}

pub fn app_version(version: impl Into<String>) -> Attr {
    AppVersion(version)
}

pub fn error_type(typ: impl Into<String>) -> Attr {
    ErrorType(typ)
}

pub fn error_code(code: impl Into<String>) -> Attr {
    ErrorCode(code)
}

pub fn error_message(msg: impl Into<String>) -> Attr {
    ErrorMessage(msg)
}

pub fn error_stack(stack: impl Into<String>) -> Attr {
    ErrorStack(stack)
}

pub fn retryable(value: bool) -> Attr {
    Retryable(value)
}

// --- Schema constructors ---

pub fn default_schema() -> schema::DefaultSchemaType {
    DefaultSchema()
}

pub fn flat_schema() -> schema::DefaultSchemaType {
    FlatSchema()
}

pub fn nested_schema() -> schema::DefaultSchemaType {
    NestedSchema()
}

pub fn otel_schema() -> schema::DefaultSchemaType {
    OTelSchema()
}

pub fn otel_log_schema() -> schema::DefaultSchemaType {
    OTelLogSchema()
}

pub fn ec_schema() -> schema::DefaultSchemaType {
    ECSchema()
}

pub fn datadog_schema() -> schema::DefaultSchemaType {
    DatadogSchema()
}

// --- Sink constructors ---

pub fn stdout_sink() -> SinkConfig {
    StdoutSink()
}

pub fn stderr_sink() -> SinkConfig {
    StderrSink()
}

pub fn file_sink(path: impl Into<String>) -> SinkConfig {
    FileSink(path)
}

pub fn memory_sink() -> SinkConfig {
    MemorySink()
}

pub fn noop_sink() -> SinkConfig {
    NoopSink()
}

pub fn http_batch_sink(endpoint: impl Into<String>) -> SinkConfig {
    HttpBatchSink(endpoint)
}

// --- Sampler constructors ---

pub fn sample_all() -> SamplerConfig {
    SampleAll()
}

pub fn sample_none() -> SamplerConfig {
    SampleNone()
}

pub fn sample_random(rate: f64) -> SamplerConfig {
    SampleRandom(rate)
}

pub fn sample_rate(rate: f64) -> SamplerConfig {
    SampleRate(rate)
}

pub fn sample_errors() -> SamplerConfig {
    SampleErrors()
}

pub fn sample_slow_requests(threshold_ms: u64) -> SamplerConfig {
    SampleSlowRequests(threshold_ms)
}

pub fn sample_status_codes(codes: &[u16]) -> SamplerConfig {
    SampleStatusCodes(codes)
}

pub fn sample_routes(routes: &[&str]) -> SamplerConfig {
    SampleRoutes(routes)
}

pub fn sample_users(users: &[&str]) -> SamplerConfig {
    SampleUsers(users)
}

pub fn sample_tenants(tenants: &[&str]) -> SamplerConfig {
    SampleTenants(tenants)
}

pub fn sample_feature_flag(name: &str, value: &Value) -> SamplerConfig {
    SampleFeatureFlag(name, value)
}

pub fn any_sampler(samplers: &[SamplerConfig]) -> SamplerConfig {
    AnySampler(samplers)
}

pub fn all_sampler(samplers: &[SamplerConfig]) -> SamplerConfig {
    AllSampler(samplers)
}

pub fn not_sampler(sampler: SamplerConfig) -> SamplerConfig {
    NotSampler(sampler)
}

// --- Redactor constructors ---

pub fn default_redactor() -> RedactorConfig {
    DefaultRedactor()
}

pub fn redact(keys: &[&str]) -> RedactorConfig {
    Redact(keys)
}

pub fn redact_keys(keys: &[&str]) -> RedactorConfig {
    RedactKeys(keys)
}

pub fn drop_keys(keys: &[&str]) -> RedactorConfig {
    DropKeys(keys)
}

pub fn mask_keys(keys: &[&str]) -> RedactorConfig {
    MaskKeys(keys)
}

pub fn compose_redactors(redactors: &[RedactorConfig]) -> RedactorConfig {
    ComposeRedactors(redactors)
}

pub fn redact_patterns(patterns: &[&str]) -> RedactorConfig {
    RedactPatterns(patterns)
}

// --- Context helpers ---

pub fn from_context(ctx: &EventContext) -> Option<&EventContext> {
    FromContext(ctx)
}

pub fn event_id(ctx: &EventContext) -> Option<String> {
    EventID(ctx)
}

pub fn request_id_from_context(ctx: &EventContext) -> Option<String> {
    RequestIDFromContext(ctx)
}

pub fn trace_id_from_context(ctx: &EventContext) -> Option<String> {
    TraceIDFromContext(ctx)
}

pub fn span_id_from_context(ctx: &EventContext) -> Option<String> {
    SpanIDFromContext(ctx)
}

pub fn has_event(ctx: &EventContext) -> bool {
    HasEvent(ctx)
}

// --- Domain helper aliases ---

pub fn payment_id(id: impl Into<String>) -> Attr {
    PaymentID(id)
}
pub fn subscription_id(id: impl Into<String>) -> Attr {
    SubscriptionID(id)
}
pub fn invoice_id(id: impl Into<String>) -> Attr {
    InvoiceID(id)
}
pub fn job_id(id: impl Into<String>) -> Attr {
    JobID(id)
}
pub fn message_id(id: impl Into<String>) -> Attr {
    MessageID(id)
}
pub fn correlation_id(id: impl Into<String>) -> Attr {
    CorrelationID(id)
}
pub fn commit_sha(sha: impl Into<String>) -> Attr {
    CommitSHA(sha)
}
pub fn release(name: impl Into<String>) -> Attr {
    Release(name)
}
pub fn money(amount: f64) -> Attr {
    Money(amount)
}
pub fn percent(value: f64) -> Attr {
    Percent(value)
}
pub fn bytes(value: u64) -> Attr {
    Bytes(value)
}
pub fn http_status(code: u16) -> Attr {
    HTTPStatus(code)
}
pub fn bucket(name: impl Into<String>) -> Attr {
    Bucket(name)
}
pub fn tags(values: Vec<impl Into<String>>) -> Attr {
    Tags(values)
}
pub fn masked(value: impl Into<String>) -> Attr {
    Masked(value)
}

pub fn list<S: serde::Serialize>(key: impl Into<String>, values: Vec<S>) -> Attr {
    List(key, values)
}

pub fn map(key: impl Into<String>, value: serde_json::Map<String, Value>) -> Attr {
    Map(key, value)
}

pub fn enum_<E: Into<String>>(
    key: impl Into<String>,
    value: impl Into<String>,
    allowed: Vec<E>,
) -> Attr {
    Enum(key, value, allowed)
}

pub fn id(key: impl Into<String>, value: impl Into<String>) -> Attr {
    ID(key, value)
}

pub fn hash(key: impl Into<String>, value: impl Into<String>) -> Attr {
    Hash(key, value)
}

pub fn redacted(key: impl Into<String>) -> Attr {
    Redacted(key)
}

pub fn account_id(id: impl Into<String>) -> Attr {
    AccountID(id)
}

pub fn deployment_id(id: impl Into<String>) -> Attr {
    DeploymentID(id)
}

pub fn http_route(route: impl Into<String>) -> Attr {
    HTTPRoute(route)
}

pub fn http_method(method: impl Into<String>) -> Attr {
    HTTPMethod(method)
}

pub fn http_path(path: impl Into<String>) -> Attr {
    HTTPPath(path)
}

pub fn http_user_agent(ua: impl Into<String>) -> Attr {
    HTTPUserAgent(ua)
}

pub fn http_referer(referer: impl Into<String>) -> Attr {
    HTTPReferer(referer)
}

pub fn http_request(method: impl Into<String>, path: impl Into<String>) -> Attr {
    HTTPRequest(method, path)
}

pub fn http_response(status_code: u16) -> Attr {
    HTTPResponse(status_code)
}

pub fn url(url: impl Into<String>) -> Attr {
    URL(url)
}
pub fn email_hash(email: impl Into<String>) -> Attr {
    EmailHash(email)
}
pub fn ip_hash(ip: impl Into<String>) -> Attr {
    IPHash(ip)
}

// --- Domain pack aliases ---

pub fn checkout_cart_item_count(count: u32) -> Attr {
    CheckoutCartItemCount(count)
}
pub fn checkout_cart_total(total: f64) -> Attr {
    CheckoutCartTotal(total)
}
pub fn checkout_payment_method(method: impl Into<String>) -> Attr {
    CheckoutPaymentMethod(method)
}
pub fn checkout_status(status: impl Into<String>) -> Attr {
    CheckoutStatus(status)
}
pub fn payment_provider(provider: impl Into<String>) -> Attr {
    PaymentProvider(provider)
}
pub fn payment_method(method: impl Into<String>) -> Attr {
    PaymentMethod(method)
}
pub fn payment_intent_id(id: impl Into<String>) -> Attr {
    PaymentIntentID(id)
}
pub fn payment_failure_code(code: impl Into<String>) -> Attr {
    PaymentFailureCode(code)
}
pub fn payment_retry_attempt(attempt: u32) -> Attr {
    PaymentRetryAttempt(attempt)
}
pub fn billing_plan(plan: impl Into<String>) -> Attr {
    BillingPlan(plan)
}
pub fn billing_subscription_id(id: impl Into<String>) -> Attr {
    BillingSubscriptionID(id)
}
pub fn billing_invoice_id(id: impl Into<String>) -> Attr {
    BillingInvoiceID(id)
}
pub fn billing_amount(amount: f64) -> Attr {
    BillingAmount(amount)
}
pub fn billing_interval(interval: impl Into<String>) -> Attr {
    BillingInterval(interval)
}
pub fn agent_name(name: impl Into<String>) -> Attr {
    AgentName(name)
}
pub fn agent_provider(provider: impl Into<String>) -> Attr {
    AgentProvider(provider)
}
pub fn agent_model(model: impl Into<String>) -> Attr {
    AgentModel(model)
}
pub fn agent_run_type(run_type: impl Into<String>) -> Attr {
    AgentRunType(run_type)
}
pub fn agent_tool_name(name: impl Into<String>) -> Attr {
    AgentToolName(name)
}
pub fn agent_tool_outcome(outcome: impl Into<String>) -> Attr {
    AgentToolOutcome(outcome)
}
pub fn agent_input_tokens(tokens: u64) -> Attr {
    AgentInputTokens(tokens)
}
pub fn agent_output_tokens(tokens: u64) -> Attr {
    AgentOutputTokens(tokens)
}
pub fn agent_cost(cost: f64) -> Attr {
    AgentCost(cost)
}
pub fn rag_index(index: impl Into<String>) -> Attr {
    RAGIndex(index)
}
pub fn rag_embedding_model(model: impl Into<String>) -> Attr {
    RAGEmbeddingModel(model)
}
pub fn rag_chunks_retrieved(count: u32) -> Attr {
    RAGChunksRetrieved(count)
}
pub fn rag_top_score(score: f64) -> Attr {
    RAGTopScore(score)
}
pub fn rag_query_hash(hash: impl Into<String>) -> Attr {
    RAGQueryHash(hash)
}
pub fn rag_citation_count(count: u32) -> Attr {
    RAGCitationCount(count)
}
pub fn rag_retrieval_latency(latency_ms: u64) -> Attr {
    RAGRetrievalLatency(latency_ms)
}

// --- Lifecycle extras ---

pub fn drop(event: &mut EventContext, reason: impl Into<String>) {
    Drop(event, reason)
}
pub fn cancel(event: &mut EventContext) {
    Cancel(event)
}
pub fn abandon(event: &mut EventContext) {
    Abandon(event)
}
pub fn retry(event: &mut EventContext) {
    Retry(event)
}
pub fn partial(event: &mut EventContext, reason: impl Into<String>) {
    Partial(event, reason)
}
pub fn clone_event(event: &EventContext) -> EventContext {
    CloneEvent(event)
}
pub fn link_event(event: &mut EventContext, linked_id: impl Into<String>) {
    LinkEvent(event, linked_id)
}
pub fn current_event() -> Option<EventContext> {
    CurrentEvent()
}
pub fn bind_event(logger: &Logger, event: &EventContext) -> EventContext {
    BindEvent(logger, event)
}
pub fn wrap(event: &mut EventContext, f: impl FnOnce(&mut EventContext)) {
    Wrap(event, f)
}

// --- Process/Group/Timer extras ---

pub fn with_process(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(ProcessHandle, &mut EventContext),
) {
    WithProcess(event, name, f)
}
pub fn with_group(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(GroupHandle, &mut EventContext),
) {
    WithGroup(event, name, f)
}
pub fn with_timer(
    event: &mut EventContext,
    name: &str,
    f: impl FnOnce(TimerHandle, &mut EventContext),
) {
    WithTimer(event, name, f)
}
pub fn finish_group_error(handle: GroupHandle, event: &mut EventContext, message: &str) {
    FinishGroupError(handle, event, message)
}
pub fn measure(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    Measure(event, name, f)
}
pub fn step(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    Step(event, name, f)
}
pub fn phase(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    Phase(event, name, f)
}
pub fn span(event: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
    Span(event, name, f)
}

// --- Logging helpers ---

pub fn notice(message: impl Into<String>) {
    Notice(message)
}
pub fn event(name: impl Into<String>) -> EventContext {
    Event(name)
}
pub fn track(name: impl Into<String>, attrs: &[Attr]) {
    Track(name, attrs)
}
pub fn audit(name: impl Into<String>) -> EventContext {
    Audit(name)
}
pub fn security(name: impl Into<String>) -> EventContext {
    Security(name)
}
pub fn metric(name: impl Into<String>) -> EventContext {
    Metric(name)
}
pub fn count(name: impl Into<String>, value: u64) {
    Count(name, value)
}
pub fn gauge(name: impl Into<String>, value: f64) {
    Gauge(name, value)
}
pub fn histogram(name: impl Into<String>, value: f64) {
    Histogram(name, value)
}
pub fn breadcrumb(message: impl Into<String>) {
    Breadcrumb(message)
}

// --- Config extras ---

pub fn disabled_config() -> Config {
    DisabledConfig()
}
pub fn from_env() -> Config {
    FromEnv()
}

// --- Sink extras ---

pub fn multi_sink(sinks: &[SinkConfig]) -> Vec<SinkConfig> {
    MultiSink(sinks)
}
pub fn otlp_sink(endpoint: impl Into<String>) -> SinkConfig {
    OtlpSink(endpoint)
}
pub fn drain(sink: &SinkConfig) {
    Drain(sink)
}
pub fn pause(sink: &SinkConfig) {
    Pause(sink)
}
pub fn resume(sink: &SinkConfig) {
    Resume(sink)
}
pub fn queue_size(sink: &SinkConfig) -> usize {
    QueueSize(sink)
}
pub fn health(sink: &SinkConfig) -> bool {
    Health(sink)
}

// --- Sampling/Policy extras ---

pub fn sample_by_event(f: impl Fn(&EventContext) -> bool + Send + Sync + 'static) -> SamplerConfig {
    SampleByEvent(f)
}
pub fn sample_by_outcome(outcomes: &[&str]) -> SamplerConfig {
    SampleByOutcome(outcomes)
}
pub fn should_sample(event: &EventContext, sampler: &SamplerConfig) -> bool {
    ShouldSample(event, sampler)
}
pub fn allow_fields(keys: &[&str]) -> RedactorConfig {
    AllowFields(keys)
}
pub fn block_fields(keys: &[&str]) -> RedactorConfig {
    BlockFields(keys)
}

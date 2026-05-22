#![allow(non_snake_case, non_upper_case_globals)]

mod config;
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
use std::sync::{OnceLock, RwLock};

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
pub use logger::Logger;
pub use metrics::{MetricsCollector, MetricsSnapshot};
pub use schema::{DefaultSchemaType, EventView, Schema, SchemaFunc};

pub const LevelDebug: &str = "debug";
pub const LevelInfo: &str = "info";
pub const LevelWarn: &str = "warn";
pub const LevelError: &str = "error";
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
    *lock.write().unwrap() = logger.clone();
    Ok(logger)
}

/// Return the global default logger. Always succeeds (dev default if not configured).
pub fn default() -> Logger {
    GLOBAL_LOGGER
        .get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))))
        .read()
        .unwrap()
        .clone()
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
    GLOBAL_LOGGER
        .get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))))
        .read()
        .unwrap()
        .clone()
}

pub fn set_global_logger(logger: Logger) {
    let lock = GLOBAL_LOGGER.get_or_init(|| RwLock::new(Logger::new(Config::dev("loxa"))));
    *lock.write().unwrap() = logger;
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
    let _ = Flush();
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

pub fn HasEvent(_: &EventContext) -> bool {
    true
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
    CollectorSinkWithEndpoint("http://127.0.0.1:9090/v1/events")
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

// --- Redactor constructors ---

pub fn DefaultRedactor() -> RedactorConfig {
    RedactorConfig::Default
}

pub fn RedactKeys(keys: &[&str]) -> RedactorConfig {
    RedactorConfig::Keys(keys.iter().map(|k| k.to_string()).collect())
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

pub fn null(key: impl Into<String>) -> Attr {
    Null(key)
}

pub fn group(name: impl Into<String>, attrs: Vec<Attr>) -> Attr {
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

pub fn redact_keys(keys: &[&str]) -> RedactorConfig {
    RedactKeys(keys)
}

pub fn hash_keys(keys: &[&str]) -> RedactorConfig {
    HashKeys(keys)
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

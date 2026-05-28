use crate::{
    errors::LoxaError,
    generated::spec_contract,
    internal::core::{duplicate::resolve_duplicate, duplicate_policy::DuplicatePolicy},
    internal::queue::ByteBatcher,
    metrics::MetricsCollector,
    redaction, sampling, schema, sink, Attr, Config, EventContext, Params,
};
use serde_json::Map;
use serde_json::Value;
use std::sync::mpsc::{self, Receiver, SyncSender};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Instant;

const DEFAULT_ASYNC_QUEUE_SIZE: usize = 8192;
const DEFAULT_ASYNC_MAX_BATCH_BYTES: usize = 256 * 1024;

pub struct Logger {
    config: Config,
    async_runtime: Option<Arc<AsyncRuntime>>,
    metrics: Arc<MetricsCollector>,
}

impl Clone for Logger {
    fn clone(&self) -> Self {
        Self {
            config: self.config.clone(),
            async_runtime: self.async_runtime.clone(),
            metrics: self.metrics.clone(),
        }
    }
}

impl std::fmt::Debug for Logger {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Logger")
            .field("config", &self.config)
            .field("async_enabled", &self.async_runtime.is_some())
            .field("metrics_enabled", &true)
            .finish()
    }
}

impl Logger {
    pub fn new(config: Config) -> Self {
        Self::try_new(config).expect("invalid loxa config")
    }

    pub fn try_new(config: Config) -> Result<Self, LoxaError> {
        let mut config = config;
        config.validate_result()?;
        install_default_collector_sink(&mut config);
        let async_runtime = if config.async_enabled {
            Some(Arc::new(AsyncRuntime::new(config.sinks.clone())?))
        } else {
            None
        };
        Ok(Self {
            config,
            async_runtime,
            metrics: Arc::new(MetricsCollector::new()),
        })
    }

    /// Returns the names of the configured sinks (e.g. ["HttpBatch"], ["Stdout"], ["File", "HttpBatch"]).
    pub fn sink_names(&self) -> Vec<String> {
        self.config
            .sinks
            .iter()
            .map(|s| match s {
                crate::SinkConfig::Stdout => "Stdout".to_string(),
                crate::SinkConfig::Stderr => "Stderr".to_string(),
                crate::SinkConfig::File(_) => "File".to_string(),
                crate::SinkConfig::Memory(_) => "Memory".to_string(),
                crate::SinkConfig::Noop => "Noop".to_string(),
                crate::SinkConfig::HttpBatch { .. } => "HttpBatch".to_string(),
            })
            .collect()
    }

    /// Returns the configured collector endpoint, or empty string if not set.
    pub fn collector_endpoint(&self) -> &str {
        &self.config.collector_endpoint
    }

    /// Returns a reference to the logger configuration.
    pub fn config(&self) -> &Config {
        &self.config
    }

    /// Create an immutable child logger that preserves config and emits loxa.alias.
    pub fn alias(&self, name: impl Into<String>) -> Logger {
        let mut cfg = self.config.clone();
        cfg.alias = name.into();
        Logger::new(cfg)
    }

    pub fn start_event(&self, params: Params) -> EventContext {
        self.metrics.record_event_created();
        let mut params = params;
        if params.service.is_none() && !self.config.service.is_empty() {
            params.service = Some(self.config.service.clone());
        }
        if params.version.is_none() && !self.config.version.is_empty() {
            params.version = Some(self.config.version.clone());
        }
        if params.environment.is_none() && !self.config.environment.is_empty() {
            params.environment = Some(self.config.environment.clone());
        }
        if params.region.is_none() && !self.config.region.is_empty() {
            params.region = Some(self.config.region.clone());
        }
        if params.deployment_id.is_none() && !self.config.deployment_id.is_empty() {
            params.deployment_id = Some(self.config.deployment_id.clone());
        }
        let mut ctx = EventContext::new(self.config.service.clone(), params);
        if !self.config.alias.is_empty() {
            ctx.append_attr(Attr::new("loxa.alias", self.config.alias.clone()));
        }
        if self.config.include_host && ctx.host.is_none() {
            let host = std::env::var("HOSTNAME").unwrap_or_else(|_| {
                hostname::get()
                    .map(|h| h.to_string_lossy().to_string())
                    .unwrap_or_default()
            });
            if !host.is_empty() {
                ctx.host = Some(host);
            }
        }
        ctx
    }

    pub fn enrich<V: Into<Value>>(&self, ctx: &mut EventContext, key: impl Into<String>, value: V) {
        self.apply_attr(ctx, Attr::new(key, value));
    }

    pub fn append(&self, ctx: &mut EventContext, attr: Attr) {
        self.apply_attr(ctx, attr);
    }

    pub fn set<V: Into<Value>>(&self, ctx: &mut EventContext, key: impl Into<String>, value: V) {
        self.enrich(ctx, key, value);
    }

    pub fn merge<V: Into<Value>>(
        &self,
        ctx: &mut EventContext,
        group: &str,
        key: impl Into<String>,
        value: V,
    ) {
        let _ = ctx.ensure_mutable();
        ctx.mark_active_if_created();
        let target = match group {
            "http" => &mut ctx.http,
            "user" => &mut ctx.user,
            "tenant" => &mut ctx.tenant,
            "resource" => &mut ctx.resource,
            _ => &mut ctx.attrs,
        };
        if let Err(err) = apply_path(
            target,
            &key.into(),
            value.into(),
            DuplicatePolicy::parse(&self.config.duplicate_policy),
        ) {
            ctx.note_error(err);
        }
    }

    pub fn delete(&self, ctx: &mut EventContext, key: &str) {
        ctx.delete_attr(key);
    }

    pub fn checkpoint(&self, ctx: &mut EventContext, name: impl Into<String>) {
        ctx.checkpoint(&name.into());
    }

    pub fn checkpoint_with_attrs(
        &self,
        ctx: &mut EventContext,
        name: impl Into<String>,
        attrs: &[Attr],
    ) {
        ctx.checkpoint_with_attrs(&name.into(), attrs);
    }

    pub fn finish(
        &self,
        ctx: &mut EventContext,
        outcome: impl Into<String>,
    ) -> Result<(), LoxaError> {
        let result = ctx.finish(outcome);
        if result.is_ok() {
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn finish_error(
        &self,
        ctx: &mut EventContext,
        message: impl Into<String>,
    ) -> Result<(), LoxaError> {
        let result = ctx.finish_error(message);
        if result.is_ok() {
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn emit(&self, ctx: &EventContext) -> Result<String, LoxaError> {
        crate::set_current_event(Some(ctx.clone()));
        let result = if self.config.panic_recovery {
            let result =
                std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| self.emit_inner(ctx)));
            match result {
                Ok(inner) => inner,
                Err(panic) => {
                    let msg = panic
                        .downcast_ref::<&str>()
                        .map(|s| s.to_string())
                        .or_else(|| panic.downcast_ref::<String>().cloned())
                        .unwrap_or_else(|| "panic in emit".to_string());
                    self.metrics.record_event_dropped("panic");
                    Err(LoxaError::Transport(msg))
                }
            }
        } else {
            self.emit_inner(ctx)
        };
        crate::set_current_event(None);
        result
    }

    fn emit_inner(&self, ctx: &EventContext) -> Result<String, LoxaError> {
        let started = Instant::now();
        match ctx.lifecycle_state().as_str() {
            crate::event::EVENT_EMITTED => {
                self.metrics.record_event_emitted(true);
                return Ok(ctx.cached_emitted_payload().unwrap_or_default());
            }
            crate::event::EVENT_EMITTING
            | crate::event::EVENT_DELIVERY_FAILED
            | crate::event::EVENT_FAILED_VALIDATION => {
                self.metrics.record_event_dropped("closed");
                self.notify_drop("closed");
                return Err(LoxaError::EventClosed {
                    event_id: ctx.event_id.clone(),
                    state: ctx.lifecycle_state(),
                });
            }
            _ => {}
        }
        if let Err(err) = self.validate(ctx) {
            ctx.mark_validation_failed();
            self.metrics.record_event_dropped("validation");
            self.notify_drop("validation");
            return Err(err);
        }
        if !sampling::should_sample(ctx, &self.config.sampler) {
            ctx.mark_delivery_accepted(Some(""));
            self.metrics.record_event_dropped("sampled");
            self.notify_drop("sampled");
            return Ok(String::new());
        }
        ctx.begin_emit()?;
        let payload = match &self.config.schema {
            crate::SchemaConfig::Custom(schema) => {
                let encoded = schema.encode(ctx);
                let encoded = crate::event::apply_sensitive_to_value(encoded, ctx);
                redaction::redact(encoded, &self.config.redactor)
            }
            other => {
                let raw = serde_json::to_value(ctx)
                    .unwrap_or(serde_json::Value::Object(serde_json::Map::new()));
                let raw = crate::event::apply_sensitive_to_value(raw, ctx);
                let redacted = redaction::redact(raw, &self.config.redactor);
                schema::encode_schema_from_value(redacted, other)
            }
        };
        if self.config.strict && schema_supports_strict_validation(&self.config.schema) {
            spec_contract::validate_event_value(&payload, true).map_err(LoxaError::Validation)?;
        }
        let encoded = serde_json::to_string(&payload)?;
        if encoded.len() > self.config.max_event_bytes {
            ctx.mark_validation_failed();
            self.metrics.record_event_dropped("max_event_bytes");
            self.notify_drop("max_event_bytes");
            return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                None,
                "max_event_bytes_exceeded",
                "event exceeds max_event_bytes".to_string(),
            )));
        }
        if let Some(runtime) = &self.async_runtime {
            runtime.enqueue(encoded.clone()).map_err(|err| {
                self.metrics.record_backpressure();
                self.metrics.record_event_dropped("backpressure");
                self.notify_drop("backpressure");
                LoxaError::Transport(format!("delivery_failed for {}: {err}", ctx.event_id))
            })?;
        } else {
            for configured_sink in &self.config.sinks {
                if let Err(err) = sink::write_sink(configured_sink, &encoded) {
                    ctx.mark_delivery_failed();
                    self.metrics.record_event_dropped("transport");
                    self.notify_drop("transport");
                    let err_msg = format!("delivery_failed for {}: {err}", ctx.event_id);
                    self.notify_delivery_failed(ctx, &err_msg);
                    return Err(LoxaError::Transport(err_msg));
                }
            }
        }
        ctx.mark_delivery_accepted(Some(&encoded));
        self.metrics.record_event_emitted(true);
        self.metrics.observe_emit_duration(started.elapsed());
        self.notify_emit(ctx);
        Ok(encoded)
    }

    pub fn flush(&self) -> Result<(), LoxaError> {
        if let Some(runtime) = &self.async_runtime {
            runtime.flush().map_err(LoxaError::Transport)?;
        }
        for configured_sink in &self.config.sinks {
            sink::flush_sink(configured_sink)
                .map_err(|err| LoxaError::Transport(format!("flush failed: {err}")))?;
        }
        Ok(())
    }

    pub fn shutdown(&self) -> Result<(), LoxaError> {
        if let Some(runtime) = &self.async_runtime {
            runtime.shutdown().map_err(LoxaError::Transport)?;
        } else {
            self.flush()?;
        }
        for configured_sink in &self.config.sinks {
            sink::close_sink(configured_sink)
                .map_err(|err| LoxaError::Transport(format!("shutdown failed: {err}")))?;
        }
        Ok(())
    }

    pub fn metrics(&self) -> Arc<MetricsCollector> {
        self.metrics.clone()
    }

    pub fn debug(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("debug", message)
    }

    pub fn info(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("info", message)
    }

    pub fn warn(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("warn", message)
    }

    pub fn error(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("error", message)
    }

    pub fn fatal(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("fatal", message)
    }

    pub fn notice(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        self.emit_immediate("notice", message)
    }

    pub fn breadcrumb(&self, message: impl Into<String>) -> Result<String, LoxaError> {
        let mut ctx = self.start_event(
            Params::new("log.breadcrumb")
                .with_kind("log")
                .with_message(message),
        );
        ctx.level = "debug".to_string();
        let _ = ctx.finish("success");
        self.emit(&ctx)
    }

    pub fn fatal_exit(&self, message: impl Into<String>) -> ! {
        // Emit and flush before exiting, logging any errors to stderr
        if let Err(err) = self.emit_immediate("fatal", message) {
            eprintln!("loxa: fatal event emit failed: {err}");
        }
        if let Err(err) = self.flush() {
            eprintln!("loxa: flush on fatal exit failed: {err}");
        }
        std::process::exit(1)
    }

    pub fn drop_event(
        &self,
        ctx: &mut EventContext,
        reason: impl Into<String>,
    ) -> Result<(), LoxaError> {
        ctx.outcome = Some("dropped".to_string());
        ctx.partial = true;
        ctx.partial_reason = Some(reason.into());
        Ok(())
    }

    pub fn cancel(&self, ctx: &mut EventContext) -> Result<(), LoxaError> {
        let result = ctx.finish("cancelled");
        if result.is_ok() {
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn abandon(&self, ctx: &mut EventContext) -> Result<(), LoxaError> {
        let result = ctx.finish("abandoned");
        if result.is_ok() {
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn retry(&self, ctx: &mut EventContext) -> Result<(), LoxaError> {
        let result = ctx.finish("retried");
        if result.is_ok() {
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn partial(
        &self,
        ctx: &mut EventContext,
        reason: impl Into<String>,
    ) -> Result<(), LoxaError> {
        let result = ctx.finish("partial");
        if result.is_ok() {
            ctx.partial = true;
            ctx.partial_reason = Some(reason.into());
            self.metrics.record_event_finished();
        }
        result
    }

    pub fn clone_event(&self, ctx: &EventContext) -> EventContext {
        ctx.clone()
    }

    pub fn link_event(&self, ctx: &mut EventContext, linked_id: impl Into<String>) {
        let mut link = serde_json::Map::new();
        link.insert(
            "event_id".to_string(),
            serde_json::Value::String(linked_id.into()),
        );
        if ctx.error.is_none() {
            ctx.error = Some(serde_json::Map::new());
        }
    }

    pub fn bind_event(&self, ctx: &EventContext) -> EventContext {
        ctx.clone()
    }

    pub fn wrap(&self, ctx: &mut EventContext, f: impl FnOnce(&mut EventContext)) {
        f(ctx);
    }

    pub fn with_process(
        &self,
        ctx: &mut EventContext,
        name: &str,
        f: impl FnOnce(&mut EventContext),
    ) {
        let process = ctx.start_process(name);
        f(ctx);
        process.finish(ctx, &[]);
    }

    pub fn with_group(
        &self,
        ctx: &mut EventContext,
        name: &str,
        f: impl FnOnce(&mut EventContext),
    ) {
        let group = ctx.start_group(name);
        f(ctx);
        group.finish(ctx, &[]);
    }

    pub fn with_timer(
        &self,
        ctx: &mut EventContext,
        name: &str,
        f: impl FnOnce(&mut EventContext),
    ) {
        let timer = ctx.start_timer(name);
        f(ctx);
        timer.stop(ctx, &[]);
    }

    pub fn measure(&self, ctx: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
        self.with_timer(ctx, name, f);
    }

    pub fn step(&self, ctx: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
        self.with_process(ctx, name, f);
    }

    pub fn phase(&self, ctx: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
        self.with_group(ctx, name, f);
    }

    pub fn span(&self, ctx: &mut EventContext, name: &str, f: impl FnOnce(&mut EventContext)) {
        self.with_timer(ctx, name, f);
    }

    fn emit_immediate(&self, level: &str, message: impl Into<String>) -> Result<String, LoxaError> {
        let mut ctx = self.start_event(
            Params::new(format!("log.{level}"))
                .with_kind("log")
                .with_message(message),
        );
        ctx.level = level.to_string();
        let _ = ctx.finish("success");
        self.emit(&ctx)
    }

    fn validate(&self, ctx: &EventContext) -> Result<(), LoxaError> {
        if !self.config.strict {
            if let Some(error) = ctx.pending_error() {
                return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                    None,
                    "pending_error",
                    error,
                )));
            }
            return Ok(());
        }
        if let Some(error) = ctx.pending_error() {
            return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                None,
                "pending_error",
                error,
            )));
        }
        if ctx.service.is_empty() {
            return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                Some("service"),
                "required",
                "strict mode requires a non-empty service".to_string(),
            )));
        }
        if ctx.event.is_empty() {
            return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                Some("event"),
                "required",
                "strict mode requires a non-empty event".to_string(),
            )));
        }
        if ctx.kind.is_empty() {
            return Err(LoxaError::Validation(crate::errors::ValidationError::new(
                Some("kind"),
                "required",
                "strict mode requires a non-empty kind".to_string(),
            )));
        }
        Ok(())
    }

    fn notify_emit(&self, ctx: &EventContext) {
        if let Some(handler) = &self.config.stats_handler {
            handler.on_emit(ctx);
        }
    }

    fn notify_drop(&self, reason: &str) {
        if let Some(handler) = &self.config.stats_handler {
            handler.on_drop(reason);
        }
    }

    fn notify_delivery_failed(&self, ctx: &EventContext, error: &str) {
        if let Some(handler) = &self.config.stats_handler {
            handler.on_delivery_failed(ctx, error);
        }
    }
}

impl Logger {
    fn apply_attr(&self, ctx: &mut EventContext, attr: Attr) {
        let _ = ctx.ensure_mutable();
        ctx.mark_active_if_created();
        if attr.sensitive {
            ctx.sensitive_keys.push(attr.key.clone());
        }
        if attr.hash_value {
            ctx.hash_keys.push(attr.key.clone());
        }
        let target = if let Some(key) = attr.key.strip_prefix("user.") {
            (&mut ctx.user, key.to_string())
        } else if let Some(key) = attr.key.strip_prefix("tenant.") {
            (&mut ctx.tenant, key.to_string())
        } else if let Some(key) = attr.key.strip_prefix("http.") {
            (&mut ctx.http, key.to_string())
        } else if let Some(key) = attr.key.strip_prefix("resource.") {
            (&mut ctx.resource, key.to_string())
        } else {
            (&mut ctx.attrs, attr.key.clone())
        };
        if let Err(err) = apply_path(
            target.0,
            &target.1,
            attr.value,
            DuplicatePolicy::parse(&self.config.duplicate_policy),
        ) {
            ctx.note_error(err);
        }
    }
}

enum AsyncCommand {
    Event(String),
    Flush(SyncSender<Result<(), String>>),
    Shutdown(SyncSender<Result<(), String>>),
}

struct AsyncRuntime {
    tx: SyncSender<AsyncCommand>,
    worker: Mutex<Option<JoinHandle<()>>>,
}

impl AsyncRuntime {
    fn new(sinks: Vec<crate::SinkConfig>) -> Result<Self, LoxaError> {
        let (tx, rx) = mpsc::sync_channel(DEFAULT_ASYNC_QUEUE_SIZE);
        let worker = thread::spawn(move || run_async_worker(rx, sinks));
        Ok(Self {
            tx,
            worker: Mutex::new(Some(worker)),
        })
    }

    fn enqueue(&self, encoded: String) -> Result<(), String> {
        self.tx
            .send(AsyncCommand::Event(encoded))
            .map_err(|err| format!("async queue send failed: {err}"))
    }

    fn flush(&self) -> Result<(), String> {
        let (ack_tx, ack_rx) = mpsc::sync_channel(1);
        self.tx
            .send(AsyncCommand::Flush(ack_tx))
            .map_err(|err| format!("async flush send failed: {err}"))?;
        ack_rx
            .recv()
            .map_err(|err| format!("async flush ack failed: {err}"))?
    }

    fn shutdown(&self) -> Result<(), String> {
        let (ack_tx, ack_rx) = mpsc::sync_channel(1);
        self.tx
            .send(AsyncCommand::Shutdown(ack_tx))
            .map_err(|err| format!("async shutdown send failed: {err}"))?;
        let result = ack_rx
            .recv()
            .map_err(|err| format!("async shutdown ack failed: {err}"))?;
        if let Ok(mut worker) = self.worker.lock() {
            if let Some(handle) = worker.take() {
                let _ = handle.join();
            }
        }
        result
    }
}

fn run_async_worker(rx: Receiver<AsyncCommand>, sinks: Vec<crate::SinkConfig>) {
    let mut batcher = ByteBatcher::new(DEFAULT_ASYNC_MAX_BATCH_BYTES);
    while let Ok(command) = rx.recv() {
        match command {
            AsyncCommand::Event(encoded) => {
                if let Some(flushed) = batcher.push(encoded) {
                    let _ = flush_batch(&sinks, &flushed);
                }
            }
            AsyncCommand::Flush(reply) => {
                let result =
                    flush_batch(&sinks, &batcher.drain()).and_then(|_| flush_sinks(&sinks));
                let _ = reply.send(result);
            }
            AsyncCommand::Shutdown(reply) => {
                let result = flush_batch(&sinks, &batcher.drain())
                    .and_then(|_| flush_sinks(&sinks))
                    .and_then(|_| close_sinks(&sinks));
                let _ = reply.send(result);
                break;
            }
        }
    }
}

fn flush_batch(sinks: &[crate::SinkConfig], encoded_events: &[String]) -> Result<(), String> {
    if encoded_events.is_empty() {
        return Ok(());
    }
    for configured_sink in sinks {
        sink::write_batch_sink(configured_sink, encoded_events)
            .map_err(|err| format!("async delivery failed: {err}"))?;
    }
    Ok(())
}

fn flush_sinks(sinks: &[crate::SinkConfig]) -> Result<(), String> {
    for configured_sink in sinks {
        sink::flush_sink(configured_sink).map_err(|err| format!("flush failed: {err}"))?;
    }
    Ok(())
}

fn close_sinks(sinks: &[crate::SinkConfig]) -> Result<(), String> {
    for configured_sink in sinks {
        sink::close_sink(configured_sink).map_err(|err| format!("close failed: {err}"))?;
    }
    Ok(())
}

fn install_default_collector_sink(config: &mut Config) {
    let endpoint = config.collector_endpoint.trim();
    if endpoint.is_empty()
        || config.sinks.iter().any(is_http_batch_sink)
        || config.environment == "test"
    {
        return;
    }

    let http_batch = crate::SinkConfig::HttpBatch {
        endpoint: endpoint.to_string(),
        api_key: None,
        timeout_ms: 2_000,
        max_batch_bytes: 256 * 1024,
        max_retries: 3,
        enable_compression: true,
        ndjson: false,
    };

    if config.sinks.is_empty() || config.sinks.iter().all(is_default_terminal_sink) {
        // All sinks are default terminal sinks (Stdout/Stderr) or empty — replace with HttpBatch
        config.sinks = vec![http_batch];
    } else {
        // Explicit non-default sinks exist — append HttpBatch alongside them
        config.sinks.push(http_batch);
    }
}

fn schema_supports_strict_validation(schema: &crate::SchemaConfig) -> bool {
    matches!(
        schema,
        crate::SchemaConfig::Default | crate::SchemaConfig::Nested
    )
}

fn apply_path(
    target: &mut Map<String, Value>,
    key: &str,
    value: Value,
    policy: DuplicatePolicy,
) -> Result<(), String> {
    if !key.contains('.') {
        if let Some(existing) = target.get(key).cloned() {
            let resolved = resolve_duplicate(existing, value, policy)?;
            target.insert(key.to_string(), resolved);
        } else {
            target.insert(key.to_string(), value);
        }
        return Ok(());
    }

    let mut current = target;
    let mut parts = key.split('.').peekable();
    while let Some(part) = parts.next() {
        if parts.peek().is_none() {
            if let Some(existing) = current.get(part).cloned() {
                let resolved = resolve_duplicate(existing, value, policy)?;
                current.insert(part.to_string(), resolved);
            } else {
                current.insert(part.to_string(), value);
            }
            return Ok(());
        }
        let entry = current
            .entry(part.to_string())
            .or_insert_with(|| Value::Object(Map::new()));
        if !entry.is_object() {
            *entry = Value::Object(Map::new());
        }
        current = entry.as_object_mut().expect("object");
    }
    Ok(())
}

fn is_http_batch_sink(sink: &crate::SinkConfig) -> bool {
    matches!(sink, crate::SinkConfig::HttpBatch { .. })
}

fn is_default_terminal_sink(sink: &crate::SinkConfig) -> bool {
    matches!(sink, crate::SinkConfig::Stdout | crate::SinkConfig::Stderr)
}

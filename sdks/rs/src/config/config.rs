use serde::Deserialize;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use super::async_config::AsyncConfig;
use super::security::SecurityConfig;

/// Receives logger pipeline telemetry callbacks.
pub trait StatsHandler: Send + Sync {
    fn on_emit(&self, event: &crate::EventContext);
    fn on_drop(&self, reason: &str);
    fn on_error(&self, error: &str);
    /// Called when delivery fails. Default implementation delegates to on_error.
    fn on_delivery_failed(&self, _event: &crate::EventContext, error: &str) {
        self.on_error(error);
    }
    /// Called when collector returns ack/nack response. Default is no-op.
    fn on_collector_ack(
        &self,
        _acks: &[serde_json::Value],
        _errors: &[serde_json::Value],
        _request_id: &str,
        _deduped: u32,
    ) {
    }
}

/// Stable-v1 alias for handlers that care about explicit delivery failures.
pub trait DeliveryFailureHandler: StatsHandler {}

#[derive(Clone)]
pub struct Config {
    pub service: String,
    pub alias: String,
    pub version: String,
    pub environment: String,
    pub region: String,
    pub level: String,
    pub strict: bool,
    pub async_enabled: bool,
    pub collector_endpoint: String,
    pub api_key: String,
    pub duplicate_policy: String,
    pub max_event_bytes: usize,
    pub sampler: SamplerConfig,
    pub redactor: RedactorConfig,
    pub schema: SchemaConfig,
    pub sinks: Vec<SinkConfig>,
    pub stats_handler: Option<Arc<dyn StatsHandler>>,
    pub deployment_id: String,
    pub include_host: bool,
    pub panic_recovery: bool,
    pub security: SecurityConfig,
    pub async_config: AsyncConfig,
    pub timeout_ms: u64,
}

impl std::fmt::Debug for Config {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Config")
            .field("service", &self.service)
            .field("alias", &self.alias)
            .field("version", &self.version)
            .field("environment", &self.environment)
            .field("region", &self.region)
            .field("level", &self.level)
            .field("strict", &self.strict)
            .field("async_enabled", &self.async_enabled)
            .field("collector_endpoint", &self.collector_endpoint)
            .field("duplicate_policy", &self.duplicate_policy)
            .field("max_event_bytes", &self.max_event_bytes)
            .field("sinks", &self.sinks)
            .field(
                "stats_handler",
                &self.stats_handler.as_ref().map(|_| "Some(StatsHandler)"),
            )
            .field("deployment_id", &self.deployment_id)
            .field("include_host", &self.include_host)
            .field("panic_recovery", &self.panic_recovery)
            .field("security", &self.security)
            .field("async_config", &self.async_config)
            .finish()
    }
}

#[derive(Clone, Debug, Default, Deserialize)]
pub(crate) struct FileConfig {
    pub(crate) service: Option<String>,
    pub(crate) version: Option<String>,
    pub(crate) environment: Option<String>,
    pub(crate) region: Option<String>,
    pub(crate) level: Option<String>,
    pub(crate) strict: Option<bool>,
    pub(crate) async_enabled: Option<bool>,
    pub(crate) collector_endpoint: Option<String>,
    pub(crate) api_key: Option<String>,
    pub(crate) duplicate_policy: Option<String>,
    pub(crate) max_event_bytes: Option<usize>,
}

#[derive(Clone, Debug, Default)]
pub struct MemorySinkStore {
    pub(crate) events: std::sync::Arc<std::sync::Mutex<Vec<String>>>,
}

impl MemorySinkStore {
    pub fn new() -> Self {
        Self::default()
    }
    pub fn events(&self) -> Vec<String> {
        self.events.lock().unwrap().clone()
    }
    pub fn len(&self) -> usize {
        self.events.lock().unwrap().len()
    }
    pub fn is_empty(&self) -> bool {
        self.events.lock().unwrap().is_empty()
    }
    pub fn clear(&self) {
        self.events.lock().unwrap().clear();
    }
}

#[derive(Clone, Debug)]
pub enum SinkConfig {
    Stdout,
    Stderr,
    File(String),
    Memory(MemorySinkStore),
    Noop,
    HttpBatch {
        endpoint: String,
        api_key: Option<String>,
        timeout_ms: u64,
        max_batch_bytes: usize,
        max_retries: u32,
        enable_compression: bool,
        ndjson: bool,
    },
}

#[derive(Clone)]
pub enum SamplerConfig {
    All,
    None,
    Any(Vec<SamplerConfig>),
    AllOf(Vec<SamplerConfig>),
    Not(Box<SamplerConfig>),
    Errors,
    SlowRequests(u128),
    StatusCodes(Vec<u16>),
    Routes(Vec<String>),
    Users(Vec<String>),
    Tenants(Vec<String>),
    FeatureFlag(String, serde_json::Value),
    SampleRandom(f64),
    SampleRateLimited(f64, std::time::Duration),
    SampleByHeader(String, String),
    Custom(std::sync::Arc<dyn Fn(&crate::EventContext) -> bool + Send + Sync>),
}

impl std::fmt::Debug for SamplerConfig {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::All => write!(f, "All"),
            Self::None => write!(f, "None"),
            Self::Any(s) => f.debug_tuple("Any").field(s).finish(),
            Self::AllOf(s) => f.debug_tuple("AllOf").field(s).finish(),
            Self::Not(s) => f.debug_tuple("Not").field(s).finish(),
            Self::Errors => write!(f, "Errors"),
            Self::SlowRequests(ms) => f.debug_tuple("SlowRequests").field(ms).finish(),
            Self::StatusCodes(c) => f.debug_tuple("StatusCodes").field(c).finish(),
            Self::Routes(r) => f.debug_tuple("Routes").field(r).finish(),
            Self::Users(u) => f.debug_tuple("Users").field(u).finish(),
            Self::Tenants(t) => f.debug_tuple("Tenants").field(t).finish(),
            Self::FeatureFlag(n, v) => f.debug_tuple("FeatureFlag").field(n).field(v).finish(),
            Self::SampleRandom(r) => f.debug_tuple("SampleRandom").field(r).finish(),
            Self::SampleRateLimited(r, w) => f
                .debug_tuple("SampleRateLimited")
                .field(r)
                .field(w)
                .finish(),
            Self::SampleByHeader(h, v) => {
                f.debug_tuple("SampleByHeader").field(h).field(v).finish()
            }
            Self::Custom(_) => write!(f, "Custom(<fn>)"),
        }
    }
}

#[derive(Clone, Debug)]
pub enum RedactorConfig {
    Default,
    Keys(Vec<String>),
    HashKeys(Vec<String>),
    DropKeys(Vec<String>),
    MaskKeys(Vec<String>),
    AllowKeys(Vec<String>),
    Patterns(Vec<String>),
    Compose(Vec<RedactorConfig>),
    None,
}

#[derive(Clone)]
pub enum SchemaConfig {
    Default,
    Flat,
    Nested,
    OTel,
    ECS,
    Datadog,
    Custom(std::sync::Arc<dyn crate::schema::Schema + Send + Sync>),
}

impl std::fmt::Debug for SchemaConfig {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Default => write!(f, "Default"),
            Self::Flat => write!(f, "Flat"),
            Self::Nested => write!(f, "Nested"),
            Self::OTel => write!(f, "OTel"),
            Self::ECS => write!(f, "ECS"),
            Self::Datadog => write!(f, "Datadog"),
            Self::Custom(_) => write!(f, "Custom(...)"),
        }
    }
}

impl Config {
    pub fn with_service(mut self, service: impl Into<String>) -> Self {
        self.service = service.into();
        self
    }

    pub fn with_alias(mut self, alias: impl Into<String>) -> Self {
        self.alias = alias.into();
        self
    }

    pub fn with_version(mut self, version: impl Into<String>) -> Self {
        self.version = version.into();
        self
    }

    pub fn with_environment(mut self, environment: impl Into<String>) -> Self {
        self.environment = environment.into();
        self
    }

    pub fn with_region(mut self, region: impl Into<String>) -> Self {
        self.region = region.into();
        self
    }

    pub fn with_sink(mut self, sink: SinkConfig) -> Self {
        self.sinks.push(sink);
        self
    }

    pub fn with_sampler(mut self, sampler: SamplerConfig) -> Self {
        self.sampler = sampler;
        self
    }

    pub fn with_redactor(mut self, redactor: RedactorConfig) -> Self {
        self.redactor = redactor;
        self
    }

    pub fn with_schema(mut self, schema: SchemaConfig) -> Self {
        self.schema = schema;
        self
    }

    pub fn with_collector_endpoint(mut self, endpoint: impl Into<String>) -> Self {
        self.collector_endpoint = endpoint.into();
        self
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = api_key.into();
        self
    }

    pub fn with_duplicate_policy(mut self, policy: impl Into<String>) -> Self {
        self.duplicate_policy = policy.into();
        self
    }

    pub fn with_async(mut self, enabled: bool) -> Self {
        self.async_enabled = enabled;
        self
    }

    pub fn with_stats_handler(mut self, handler: Arc<dyn StatsHandler>) -> Self {
        self.stats_handler = Some(handler);
        self
    }

    pub fn with_deployment_id(mut self, deployment_id: impl Into<String>) -> Self {
        self.deployment_id = deployment_id.into();
        self
    }

    pub fn with_include_host(mut self, include_host: bool) -> Self {
        self.include_host = include_host;
        self
    }

    pub fn with_panic_recovery(mut self, panic_recovery: bool) -> Self {
        self.panic_recovery = panic_recovery;
        self
    }

    pub fn with_security(mut self, security: SecurityConfig) -> Self {
        self.security = security;
        self
    }

    pub fn with_async_config(mut self, async_config: AsyncConfig) -> Self {
        self.async_config = async_config;
        self
    }

    pub fn with_release(mut self, release: impl Into<String>) -> Self {
        self.version = release.into();
        self
    }

    pub fn with_namespace(mut self, namespace: impl Into<String>) -> Self {
        self.service = format!("{}.{}", namespace.into(), self.service);
        self
    }

    pub fn with_otel_bridge(mut self, enabled: bool) -> Self {
        if enabled {
            self.async_enabled = true;
        }
        self
    }

    pub fn with_timeout(mut self, timeout_ms: u64) -> Self {
        self.timeout_ms = timeout_ms;
        self
    }

    pub fn with_queue_size(mut self, size: usize) -> Self {
        self.async_config.queue_size = size;
        self
    }

    pub fn with_batch_size(mut self, size: usize) -> Self {
        self.async_config.batch_size = size;
        self
    }

    pub fn with_flush_interval_ms(mut self, ms: u64) -> Self {
        self.async_config.flush_interval_ms = ms;
        self
    }

    pub fn with_retry(mut self, max_retries: u32) -> Self {
        self.async_config.max_retries = max_retries;
        self
    }

    pub fn with_logger(self, _logger: crate::Logger) -> Self {
        // In Rust, Logger IS the Config consumer — it cannot be stored inside Config
        // without a cycle. This is by design. Use StatsHandler for diagnostics.
        self
    }

    pub fn disabled() -> Self {
        Self::base()
    }

    pub fn from_env() -> Self {
        let file_cfg = super::config::load_layered_file_config().unwrap_or_default();
        file_cfg.apply(Self::base())
    }

    pub fn validate(&self) {
        if let Err(err) = self.validate_result() {
            panic!("{err}");
        }
    }

    pub fn validate_result(&self) -> Result<(), crate::errors::LoxaError> {
        if !matches!(
            self.level.as_str(),
            "debug" | "info" | "warn" | "error" | "fatal"
        ) {
            return Err(crate::errors::LoxaError::Validation(
                crate::errors::ValidationError::new(
                    None,
                    "unsupported_level",
                    "unsupported level".to_string(),
                ),
            ));
        }
        if self.max_event_bytes == 0 {
            return Err(crate::errors::LoxaError::Validation(
                crate::errors::ValidationError::new(
                    None,
                    "invalid_max_event_bytes",
                    "max_event_bytes must be positive".to_string(),
                ),
            ));
        }
        if self.strict && self.service.is_empty() {
            return Err(crate::errors::LoxaError::Validation(
                crate::errors::ValidationError::new(
                    Some("service"),
                    "required",
                    "strict mode requires service".to_string(),
                ),
            ));
        }
        Ok(())
    }
}

impl FileConfig {
    pub(crate) fn apply(self, mut cfg: Config) -> Config {
        if let Some(value) = self.service {
            cfg.service = value;
        }
        if let Some(value) = self.version {
            cfg.version = value;
        }
        if let Some(value) = self.environment {
            cfg.environment = value;
        }
        if let Some(value) = self.region {
            cfg.region = value;
        }
        if let Some(value) = self.level {
            cfg.level = value;
        }
        if let Some(value) = self.strict {
            cfg.strict = value;
        }
        if let Some(value) = self.async_enabled {
            cfg.async_enabled = value;
        }
        if let Some(value) = self.collector_endpoint {
            cfg.collector_endpoint = value;
        }
        if let Some(value) = self.api_key {
            cfg.api_key = value;
        }
        if let Some(value) = self.duplicate_policy {
            cfg.duplicate_policy = value;
        }
        if let Some(value) = self.max_event_bytes {
            cfg.max_event_bytes = value;
        }
        cfg
    }
}

pub(crate) fn load_layered_file_config() -> Result<FileConfig, std::io::Error> {
    let mut cfg = match find_defaults_config_file() {
        Ok(path) => load_file_config(path)?,
        Err(_) => FileConfig::default(),
    };
    if let Some(path) = find_user_config_file() {
        cfg = overlay_file_config(cfg, load_file_config(path)?);
    }
    cfg = overlay_file_config(cfg, super::env::load_env_config());
    Ok(cfg)
}

/// Create a Logger with 4-layer config precedence: defaults -> file -> env -> code.
#[allow(dead_code)]
pub fn new_client(code_config: Config) -> Result<crate::Logger, crate::errors::LoxaError> {
    // Step 1: Start with hardcoded defaults
    let base = Config::base();

    // Step 2: Load file config (defaults YAML + user YAML + env vars)
    let file_cfg = load_layered_file_config().unwrap_or_default();
    let mut merged = file_cfg.apply(base);

    // Step 3: Apply code config (highest precedence) - only non-default values
    merged = merge_code_config(merged, code_config);

    // Step 4: Install default collector sink if endpoint is set
    if !merged.collector_endpoint.is_empty()
        && !merged
            .sinks
            .iter()
            .any(|s| matches!(s, SinkConfig::HttpBatch { .. }))
    {
        merged.sinks = vec![SinkConfig::HttpBatch {
            endpoint: merged.collector_endpoint.clone(),
            api_key: if !merged.api_key.is_empty() {
                Some(merged.api_key.clone())
            } else {
                std::env::var("LOXA_API_KEY")
                    .ok()
                    .filter(|s| !s.is_empty())
                    .or_else(|| {
                        std::env::var("LOXA_COLLECTOR_API_KEY")
                            .ok()
                            .filter(|s| !s.is_empty())
                    })
            },
            timeout_ms: 2_000,
            max_batch_bytes: 256 * 1024,
            max_retries: 3,
            enable_compression: true,
            ndjson: false,
        }];
    }

    // Step 5: Validate and create logger
    merged.validate_result()?;
    crate::Logger::try_new(merged)
}

#[allow(dead_code)]
fn merge_code_config(mut base: Config, code: Config) -> Config {
    // Compare against base defaults to determine what was explicitly set
    let defaults = Config::base();

    if code.service != defaults.service {
        base.service = code.service;
    }
    if code.alias != defaults.alias {
        base.alias = code.alias;
    }
    if code.version != defaults.version {
        base.version = code.version;
    }
    if code.environment != defaults.environment {
        base.environment = code.environment;
    }
    if code.region != defaults.region {
        base.region = code.region;
    }
    if code.level != defaults.level {
        base.level = code.level;
    }
    if code.strict != defaults.strict {
        base.strict = code.strict;
    }
    if code.async_enabled != defaults.async_enabled {
        base.async_enabled = code.async_enabled;
    }
    if code.collector_endpoint != defaults.collector_endpoint {
        base.collector_endpoint = code.collector_endpoint;
    }
    if code.api_key != defaults.api_key {
        base.api_key = code.api_key;
    }
    if code.duplicate_policy != defaults.duplicate_policy {
        base.duplicate_policy = code.duplicate_policy;
    }
    if code.max_event_bytes != defaults.max_event_bytes {
        base.max_event_bytes = code.max_event_bytes;
    }
    // For sinks, if code config has non-default sinks, use them
    if !(code.sinks.is_empty()
        || code.sinks.len() == 1 && matches!(&code.sinks[0], SinkConfig::Stdout))
    {
        base.sinks = code.sinks;
    }
    if code.stats_handler.is_some() {
        base.stats_handler = code.stats_handler;
    }
    if !code.deployment_id.is_empty() {
        base.deployment_id = code.deployment_id;
    }
    if code.include_host {
        base.include_host = code.include_host;
    }
    if code.panic_recovery {
        base.panic_recovery = code.panic_recovery;
    }
    base
}

fn overlay_file_config(mut base: FileConfig, override_cfg: FileConfig) -> FileConfig {
    if override_cfg.service.is_some() {
        base.service = override_cfg.service;
    }
    if override_cfg.version.is_some() {
        base.version = override_cfg.version;
    }
    if override_cfg.environment.is_some() {
        base.environment = override_cfg.environment;
    }
    if override_cfg.region.is_some() {
        base.region = override_cfg.region;
    }
    if override_cfg.level.is_some() {
        base.level = override_cfg.level;
    }
    if override_cfg.strict.is_some() {
        base.strict = override_cfg.strict;
    }
    if override_cfg.async_enabled.is_some() {
        base.async_enabled = override_cfg.async_enabled;
    }
    if override_cfg.collector_endpoint.is_some() {
        base.collector_endpoint = override_cfg.collector_endpoint;
    }
    if override_cfg.duplicate_policy.is_some() {
        base.duplicate_policy = override_cfg.duplicate_policy;
    }
    if override_cfg.max_event_bytes.is_some() {
        base.max_event_bytes = override_cfg.max_event_bytes;
    }
    base
}

fn load_file_config(path: impl AsRef<Path>) -> Result<FileConfig, std::io::Error> {
    let raw = fs::read_to_string(path)?;
    let parsed = serde_yaml::from_str::<FileConfig>(&raw).unwrap_or_default();
    Ok(parsed)
}

fn find_defaults_config_file() -> Result<PathBuf, std::io::Error> {
    if let Ok(value) = std::env::var("LOXA_RS_DEFAULTS") {
        let trimmed = value.trim();
        if !trimmed.is_empty() {
            return Ok(PathBuf::from(trimmed));
        }
    }
    let cwd = std::env::current_dir()?;
    Ok([
        cwd.join("loxa-rs.defaults.yaml"),
        cwd.join("../loxa-rs.defaults.yaml"),
        cwd.join("../../loxa-rs.defaults.yaml"),
    ]
    .into_iter()
    .find(|candidate| candidate.exists())
    .unwrap_or_else(|| cwd.join("loxa-rs.defaults.yaml")))
}

fn find_user_config_file() -> Option<PathBuf> {
    if let Ok(value) = std::env::var("LOXA_RS_CONFIG") {
        let trimmed = value.trim();
        if !trimmed.is_empty() {
            return Some(PathBuf::from(trimmed));
        }
    }
    let cwd = std::env::current_dir().ok()?;
    [cwd.join(".loxa-rs.yaml"), cwd.join("loxa.yaml")]
        .into_iter()
        .find(|candidate| candidate.exists())
}

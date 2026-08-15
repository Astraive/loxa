use super::async_config::AsyncConfig;
use super::config::{Config, RedactorConfig, SamplerConfig, SchemaConfig, SinkConfig};
use super::security::SecurityConfig;

impl Config {
    pub fn dev(service: impl Into<String>) -> Self {
        let mut cfg = super::config::load_layered_file_config()
            .unwrap_or_default()
            .apply(Self::base());
        cfg.service = service.into();
        cfg.environment = "development".to_string();
        cfg.level = "debug".to_string();
        cfg.strict = false;
        cfg.async_enabled = false;
        cfg.sinks = vec![SinkConfig::Stdout];
        apply_runtime_env(&mut cfg);
        cfg
    }

    pub fn production(service: impl Into<String>) -> Self {
        let mut cfg = super::config::load_layered_file_config()
            .unwrap_or_default()
            .apply(Self::base());
        cfg.service = service.into();
        cfg.environment = "production".to_string();
        cfg.level = "info".to_string();
        cfg.strict = true;
        cfg.async_enabled = true;
        cfg.duplicate_policy = "canonical_wins".to_string();
        cfg.sinks = vec![SinkConfig::Stdout];
        apply_runtime_env(&mut cfg);
        cfg
    }

    pub fn test(service: impl Into<String>) -> Self {
        let mut cfg = super::config::load_layered_file_config()
            .unwrap_or_default()
            .apply(Self::base());
        cfg.service = service.into();
        cfg.environment = "test".to_string();
        cfg.level = "debug".to_string();
        cfg.strict = true;
        cfg.async_enabled = false;
        cfg.sinks = Vec::new();
        apply_runtime_env(&mut cfg);
        cfg
    }

    pub(crate) fn base() -> Self {
        Self {
            service: String::new(),
            alias: String::new(),
            version: String::new(),
            environment: "development".to_string(),
            region: String::new(),
            level: "info".to_string(),
            strict: false,
            async_enabled: false,
            collector_endpoint: String::new(),
            collector_name: String::new(),
            api_key: String::new(),
            basic_username: None,
            basic_password: None,
            insecure: false,
            duplicate_policy: "canonical_wins".to_string(),
            max_event_bytes: 256 * 1024,
            sampler: SamplerConfig::All,
            redactor: RedactorConfig::Default,
            schema: SchemaConfig::Default,
            sinks: vec![SinkConfig::Stdout],
            stats_handler: None,
            deployment_id: String::new(),
            include_host: false,
            panic_recovery: false,
            security: SecurityConfig::default(),
            async_config: AsyncConfig::default(),
            timeout_ms: 2_000,
        }
    }
}

fn apply_runtime_env(cfg: &mut Config) {
    let env_cfg = super::env::load_env_config();
    if let Some(version) = env_cfg.version {
        cfg.version = version;
    }
    if let Some(region) = env_cfg.region {
        cfg.region = region;
    }
    if let Some(endpoint) = env_cfg.collector_endpoint {
        cfg.collector_endpoint = endpoint;
    }
    if let Some(policy) = env_cfg.duplicate_policy {
        cfg.duplicate_policy = policy;
    }
    if let Some(max_event_bytes) = env_cfg.max_event_bytes {
        cfg.max_event_bytes = max_event_bytes;
    }
}

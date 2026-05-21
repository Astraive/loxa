use crate::config::{Config, RedactorConfig, SamplerConfig, SchemaConfig, SinkConfig};

pub type ConfigOption = Box<dyn FnOnce(Config) -> Config + Send + 'static>;

pub fn with_service(service: impl Into<String>) -> ConfigOption {
    let service = service.into();
    Box::new(move |cfg| cfg.with_service(service))
}

pub fn with_sink(sink: SinkConfig) -> ConfigOption {
    Box::new(move |cfg| cfg.with_sink(sink))
}

pub fn with_version(version: impl Into<String>) -> ConfigOption {
    let version = version.into();
    Box::new(move |cfg| cfg.with_version(version))
}

pub fn with_environment(environment: impl Into<String>) -> ConfigOption {
    let environment = environment.into();
    Box::new(move |cfg| cfg.with_environment(environment))
}

pub fn with_region(region: impl Into<String>) -> ConfigOption {
    let region = region.into();
    Box::new(move |cfg| cfg.with_region(region))
}

pub fn with_sampler(sampler: SamplerConfig) -> ConfigOption {
    Box::new(move |cfg| cfg.with_sampler(sampler))
}

pub fn with_redactor(redactor: RedactorConfig) -> ConfigOption {
    Box::new(move |cfg| cfg.with_redactor(redactor))
}

pub fn with_schema(schema: SchemaConfig) -> ConfigOption {
    Box::new(move |cfg| cfg.with_schema(schema))
}

pub fn with_collector_endpoint(endpoint: impl Into<String>) -> ConfigOption {
    let endpoint = endpoint.into();
    Box::new(move |cfg| cfg.with_collector_endpoint(endpoint))
}

pub fn with_duplicate_policy(policy: impl Into<String>) -> ConfigOption {
    let policy = policy.into();
    Box::new(move |cfg| cfg.with_duplicate_policy(policy))
}

pub fn with_async(enabled: bool) -> ConfigOption {
    Box::new(move |cfg| cfg.with_async(enabled))
}

pub fn apply(mut cfg: Config, options: impl IntoIterator<Item = ConfigOption>) -> Config {
    for option in options {
        cfg = option(cfg);
    }
    cfg
}

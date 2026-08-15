use super::config::FileConfig;

pub(crate) fn load_env_config() -> FileConfig {
    let dsn = env_string("LOZA_DSN").and_then(|raw| super::dsn::parse(&raw).ok());
    let dsn_service = dsn
        .as_ref()
        .map(|value| value.service.clone())
        .filter(|value| !value.is_empty());
    let dsn_environment = dsn.as_ref().map(|value| value.env.clone());
    let dsn_collector_name = dsn.as_ref().map(|value| value.collector_name.clone());
    let dsn_username = dsn.as_ref().and_then(|value| value.username.clone());
    let dsn_password = dsn.as_ref().and_then(|value| value.password.clone());
    let dsn_insecure = dsn.as_ref().map(|value| !value.tls);
    FileConfig {
        service: env_string("LOZA_SERVICE_NAME").or(dsn_service),
        version: env_string("LOZA_SERVICE_VERSION"),
        environment: env_string("LOZA_ENVIRONMENT").or(dsn_environment),
        region: env_string("LOZA_REGION"),
        level: env_string("LOZA_LOG_LEVEL"),
        strict: env_bool("LOZA_STRICT"),
        async_enabled: env_bool("LOZA_ASYNC_ENABLED"),
        collector_endpoint: env_string("LOZA_COLLECTOR_URL")
            .or_else(|| env_string("LOZA_COLLECTOR_ENDPOINT"))
            .or_else(|| dsn.as_ref().map(|value| value.base_url.clone())),
        collector_name: dsn_collector_name,
        api_key: env_string("LOZA_API_KEY").or_else(|| env_string("LOZA_COLLECTOR_API_KEY")),
        basic_username: dsn_username,
        basic_password: dsn_password,
        insecure: env_bool("LOZA_INSECURE").or(dsn_insecure),
        duplicate_policy: env_string("LOZA_DUPLICATE_POLICY"),
        max_event_bytes: env_usize("LOZA_MAX_EVENT_BYTES"),
    }
}

fn env_string(name: &str) -> Option<String> {
    std::env::var(name)
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
}

fn env_bool(name: &str) -> Option<bool> {
    std::env::var(name)
        .ok()
        .and_then(|value| match value.trim().to_ascii_lowercase().as_str() {
            "1" | "true" | "yes" | "on" => Some(true),
            "0" | "false" | "no" | "off" => Some(false),
            _ => None,
        })
}

fn env_usize(name: &str) -> Option<usize> {
    std::env::var(name).ok()?.trim().parse::<usize>().ok()
}

use super::config::FileConfig;

pub(crate) fn load_env_config() -> FileConfig {
    FileConfig {
        service: env_string("LOXA_SERVICE_NAME"),
        version: env_string("LOXA_SERVICE_VERSION"),
        environment: env_string("LOXA_ENVIRONMENT"),
        region: env_string("LOXA_REGION"),
        level: env_string("LOXA_LOG_LEVEL"),
        strict: env_bool("LOXA_STRICT"),
        async_enabled: env_bool("LOXA_ASYNC_ENABLED"),
        collector_endpoint: env_string("LOXA_COLLECTOR_ENDPOINT")
            .or_else(|| env_string("LOXA_COLLECTOR_URL")),
        api_key: env_string("LOXA_API_KEY")
            .or_else(|| env_string("LOXA_COLLECTOR_API_KEY")),
        duplicate_policy: env_string("LOXA_DUPLICATE_POLICY"),
        max_event_bytes: env_usize("LOXA_MAX_EVENT_BYTES"),
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

use crate::config::MemorySinkStore;
use crate::sink;
use crate::SinkConfig;

pub fn stdout() -> SinkConfig {
    SinkConfig::Stdout
}

pub fn stderr() -> SinkConfig {
    SinkConfig::Stderr
}

pub fn memory() -> SinkConfig {
    SinkConfig::Memory(MemorySinkStore::new())
}

pub fn noop() -> SinkConfig {
    SinkConfig::Noop
}

pub fn multi_sink(sinks: &[SinkConfig]) -> Vec<SinkConfig> {
    sinks.to_vec()
}

pub fn otlp_sink(endpoint: impl Into<String>) -> SinkConfig {
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

pub fn drain(sink: &SinkConfig) {
    let _ = sink::flush_sink(sink);
}

pub fn pause(_sink: &SinkConfig) {}

pub fn resume(_sink: &SinkConfig) {}

pub fn queue_size() -> usize {
    0
}

pub fn health() -> bool {
    true
}

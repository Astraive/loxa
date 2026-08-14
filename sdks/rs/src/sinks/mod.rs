pub mod httpbatch;

pub use httpbatch::*;

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
        basic_username: None,
        basic_password: None,
        insecure: false,
        timeout_ms: 2_000,
        max_batch_bytes: 256 * 1024,
        max_retries: 3,
        enable_compression: true,
        ndjson: false,
    }
}

pub fn drain(sink_cfg: &SinkConfig) {
    let _ = sink::flush_sink(sink_cfg);
}

pub fn pause(sink_cfg: &SinkConfig) {
    sink::pause_sink(sink_cfg);
}

pub fn resume(sink_cfg: &SinkConfig) {
    sink::resume_sink(sink_cfg);
}

pub fn queue_size(sink_cfg: &SinkConfig) -> usize {
    sink::sink_queue_size(sink_cfg)
}

pub fn health(sink_cfg: &SinkConfig) -> bool {
    sink::sink_health(sink_cfg)
}

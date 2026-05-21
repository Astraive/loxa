use crate::config::MemorySinkStore;
use crate::{Config, Logger, SinkConfig};

pub fn test_logger(service: &str) -> Logger {
    Logger::new(Config::test(service).with_sink(SinkConfig::Memory(MemorySinkStore::new())))
}

pub fn assert_contains(encoded: &str, needle: &str) {
    assert!(encoded.contains(needle), "event did not contain {needle}");
}

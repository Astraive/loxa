use crate::config::MemorySinkStore;
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

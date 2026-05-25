use crate::config::MemorySinkStore;
use crate::{Config, EventContext, Logger, SinkConfig};

pub fn test_logger(service: &str) -> Logger {
    Logger::new(Config::test(service).with_sink(SinkConfig::Memory(MemorySinkStore::new())))
}

/// Spec-aligned alias for test_logger.
pub fn testkit(service: &str) -> (Logger, MemorySinkStore) {
    let store = MemorySinkStore::new();
    let logger = Logger::new(Config::test(service).with_sink(SinkConfig::Memory(store.clone())));
    (logger, store)
}

pub fn assert_contains(encoded: &str, needle: &str) {
    assert!(encoded.contains(needle), "event did not contain {needle}");
}

/// Run a closure with a temporary memory sink and return captured event JSON strings.
pub fn capture(f: impl FnOnce(&Logger)) -> Vec<String> {
    let store = MemorySinkStore::new();
    let logger = Logger::new(Config::test("capture").with_sink(SinkConfig::Memory(store.clone())));
    f(&logger);
    let _ = logger.flush();
    store.events()
}

/// Assert that a decoded event JSON contains the expected key-value pair.
pub fn assert_event(encoded: &str, key: &str, expected: &str) {
    let parsed: serde_json::Value =
        serde_json::from_str(encoded).expect("assert_event: failed to parse event JSON");
    let actual = get_nested_value(&parsed, key);
    match actual {
        Some(val) => {
            let val_str = match val {
                serde_json::Value::String(s) => s.clone(),
                other => serde_json::to_string(&other).unwrap_or_default(),
            };
            assert_eq!(
                val_str, expected,
                "assert_event: key \"{key}\" expected \"{expected}\", got \"{val_str}\""
            );
        }
        None => panic!("assert_event: key \"{key}\" not found in event"),
    }
}

/// Assert that a field value is `"[REDACTED]"`.
pub fn assert_redacted(encoded: &str, key: &str) {
    assert_event(encoded, key, "[REDACTED]");
}

/// Assert that a checkpoint with the given name exists.
pub fn assert_has_checkpoint(encoded: &str, name: &str) {
    let parsed: serde_json::Value =
        serde_json::from_str(encoded).expect("assert_has_checkpoint: failed to parse event JSON");
    if let Some(checkpoints) = parsed.get("checkpoints").and_then(|v| v.as_array()) {
        let found = checkpoints.iter().any(|cp| {
            cp.get("name")
                .and_then(|n| n.as_str())
                .map(|n| n == name)
                .unwrap_or(false)
        });
        assert!(
            found,
            "assert_has_checkpoint: checkpoint \"{name}\" not found"
        );
    } else {
        panic!("assert_has_checkpoint: no checkpoints array in event");
    }
}

pub fn expect_event(logger: &Logger, name: &str, f: impl FnOnce(&EventContext)) {
    for sink in &logger.config().sinks {
        if let SinkConfig::Memory(store) = sink {
            for encoded in store.events() {
                if let Ok(val) = serde_json::from_str::<serde_json::Value>(&encoded) {
                    if val.get("event").and_then(|v| v.as_str()) == Some(name) {
                        let ctx: EventContext = serde_json::from_value(val)
                            .expect("expect_event: failed to deserialize event context");
                        f(&ctx);
                        return;
                    }
                }
            }
        }
    }
    panic!("expect_event: no event named \"{name}\" found");
}

pub fn expect_attr(event: &EventContext, key: &str, expected: &serde_json::Value) -> bool {
    event.attrs.get(key) == Some(expected)
}

pub fn snapshot_event(event: &EventContext) -> String {
    serde_json::to_string(event).unwrap_or_default()
}

pub fn mock_sink() -> SinkConfig {
    SinkConfig::Memory(MemorySinkStore::new())
}

pub fn fake_clock(unix_ms: u128) {
    crate::internal::clock::freeze_at(unix_ms);
}

pub fn set_id_generator(f: fn() -> String) {
    crate::internal::core::uuidv7::set_id_generator(Box::new(f));
}

/// Reset all global mutable state: global logger, clock, and ID generator.
pub fn reset_for_test() {
    crate::set_global_logger(Logger::new(Config::dev("loxa")));
    crate::internal::clock::unfreeze();
    crate::internal::core::uuidv7::reset_id_generator();
}

fn get_nested_value<'a>(obj: &'a serde_json::Value, path: &str) -> Option<&'a serde_json::Value> {
    let parts: Vec<&str> = path.split('.').collect();
    let mut current = obj;
    for part in parts {
        current = current.get(part)?;
    }
    Some(current)
}

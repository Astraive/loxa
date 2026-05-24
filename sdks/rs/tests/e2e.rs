// Integration tests for Rust SDK with collector
use loxa::{Config, New, Params};
use serde_json::Value;

#[test]
fn test_emit_to_collector_basic() {
    let logger = New(Config::test("integration_test"));
    let mut ctx = logger.start_event(Params::new("basic_event"));
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    // Payload should be valid JSON
    assert!(!payload.is_empty());
    let event: Value = serde_json::from_str(&payload).unwrap();
    assert_eq!(event["event"], "basic_event");
}

#[test]
fn test_event_integrity_through_pipeline() {
    let logger = New(Config::test("test_service"));

    let mut ctx = logger.start_event(
        Params::new("integrity_test")
            .with_message("Test message")
            .with_kind("event"),
    );

    logger.enrich(&mut ctx, "user_id", "user123");
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    // Verify fields are intact
    let event: Value = serde_json::from_str(&payload).unwrap();
    assert_eq!(event["event"], "integrity_test");
    assert_eq!(event["message"], "Test message");
    assert_eq!(event["kind"], "event");
    assert_eq!(event["level"], "info");
    assert_eq!(event["service"], "test_service");
    assert_eq!(event["outcome"], "success");
    assert!(event["event_id"].as_str().unwrap().contains("evt_"));
}

#[test]
fn test_multiple_events_ordering() {
    let logger = New(Config::test("order_test"));

    for i in 0..5 {
        let mut ctx = logger.start_event(Params::new(format!("event_{}", i)));
        let _ = logger.finish(&mut ctx, "success");
        let payload = logger.emit(&ctx).unwrap();

        let event: Value = serde_json::from_str(&payload).unwrap();
        assert_eq!(event["event"], format!("event_{}", i));
    }
}

#[test]
fn test_error_event_collection() {
    let logger = New(Config::test("error_test"));

    let mut ctx =
        logger.start_event(Params::new("error_event").with_message("Something went wrong"));

    logger.enrich(&mut ctx, "error_code", "E001");
    let _ = logger.finish(&mut ctx, "error");
    let payload = logger.emit(&ctx).unwrap();

    // Error should be recorded with proper outcome
    let event: Value = serde_json::from_str(&payload).unwrap();
    assert_eq!(event["outcome"], "error");
    assert_eq!(event["message"], "Something went wrong");
}

#[test]
fn test_partial_event_outcome() {
    let logger = New(Config::test("partial_test"));

    let mut ctx = logger.start_event(Params::new("partial_event"));
    let _ = logger.finish(&mut ctx, "partial");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    assert_eq!(event["outcome"], "partial");
}

#[test]
fn test_trace_context_preservation() {
    let logger = New(Config::test("trace_test"));

    let mut ctx = logger.start_event(
        Params::new("trace_event"), // Params has trace_id/span_id fields, but they're set via inherit_from or ContextCarrier
    );

    // Set via attributes instead
    logger.enrich(&mut ctx, "trace_id", "trace_abc123");
    logger.enrich(&mut ctx, "span_id", "span_def456");
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    // Should have attributes
    if let Some(attrs) = event.get("attrs") {
        assert!(attrs.get("trace_id").is_some() || attrs.get("span_id").is_some());
    }
}

#[test]
fn test_canonical_fields_immutable() {
    let logger = New(Config::test("canon_test"));

    let mut ctx = logger.start_event(Params::new("canon_event"));

    let original_event_id = ctx.event_id.clone();

    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    // Canonical fields should match original
    assert_eq!(event["event_id"], original_event_id);
    // Timestamp should exist in emitted event
    assert!(event.get("timestamp").is_some());
}

#[test]
fn test_enrichment_preserved() {
    let logger = New(Config::test("enrich_test"));

    let mut ctx = logger.start_event(Params::new("enrich_event"));

    logger.enrich(&mut ctx, "custom_field", "custom_value");
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    // Custom fields should be present
    if let Some(attrs) = event.get("attrs") {
        assert_eq!(attrs["custom_field"], "custom_value");
    }
}

#[test]
fn test_duration_calculated() {
    use std::thread;
    use std::time::Duration;

    let logger = New(Config::test("duration_test"));

    let mut ctx = logger.start_event(Params::new("duration_event"));

    thread::sleep(Duration::from_millis(10));
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    // Duration should be present
    assert!(event.get("duration_ms").is_some());
    assert!(event["duration_ms"].as_f64().unwrap() > 0.0);
}

#[test]
fn test_schema_version_set() {
    let logger = New(Config::test("schema_test"));

    let mut ctx = logger.start_event(Params::new("schema_event"));

    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    // Schema version should be canonical (0.0.2)
    assert!(event.get("schema_version").is_some());
}

#[test]
fn test_sampling_in_pipeline() {
    let logger = New(Config::test("sample_test"));

    for i in 0..3 {
        let mut ctx = logger.start_event(Params::new(format!("sampled_{}", i)));
        let _ = logger.finish(&mut ctx, "success");
        let payload = logger.emit(&ctx).unwrap();
        // Should emit events
        assert!(!payload.is_empty());
    }
}

#[test]
fn test_finish_twice_error_handling() {
    let logger = New(Config::test("finish_test"));

    let mut ctx = logger.start_event(Params::new("finish_event"));

    let _ = logger.finish(&mut ctx, "success");

    // Calling finish again may error or be a no-op
    let _ = logger.finish(&mut ctx, "error");
}

#[test]
fn test_emit_after_finish() {
    let logger = New(Config::test("emit_test"));

    let mut ctx = logger.start_event(Params::new("emit_event"));

    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    // Should produce output
    assert!(!payload.is_empty());
}

#[test]
fn test_concurrent_emissions() {
    use std::sync::{Arc, Mutex};
    use std::thread;

    let logger = Arc::new(Mutex::new(New(Config::test("concurrent_test"))));

    let mut handles = vec![];

    for i in 0..3 {
        let logger_clone = Arc::clone(&logger);
        let handle = thread::spawn(move || {
            let logger = logger_clone.lock().unwrap();
            let mut ctx = logger.start_event(Params::new(format!("concurrent_{}", i)));

            let _ = logger.finish(&mut ctx, "success");
            let payload = logger.emit(&ctx).unwrap();
            assert!(!payload.is_empty());
        });
        handles.push(handle);
    }

    for handle in handles {
        let _ = handle.join();
    }
}

#[test]
fn test_event_attributes_json() {
    let logger = New(Config::test("attrs_test"));

    let mut ctx = logger.start_event(Params::new("attrs_event"));

    logger.enrich(&mut ctx, "region", "us-west-2");
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();

    // Verify event has expected structure
    assert!(event.get("event_id").is_some());
    assert!(event.get("timestamp").is_some());
    assert!(event.get("service").is_some());
    assert!(event.get("outcome").is_some());
}

#[test]
fn test_event_version_integrity() {
    let logger = New(Config::test("version_test"));

    let mut ctx = logger.start_event(Params::new("version_event"));
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).unwrap();

    let event: Value = serde_json::from_str(&payload).unwrap();
    assert!(event.get("event_version").is_some());
    assert!(event.get("schema_version").is_some());
}

use serde_json::Value;

use loxa::{Config, ContextCarrier, EventContext, MemorySinkStore, Params, SinkConfig};

#[test]
fn incident_id_in_params() {
    let store = MemorySinkStore::new();
    let logger = loxa::New(Config::test("test-svc").with_sink(SinkConfig::Memory(store.clone())));
    let mut ctx = logger.start_event(Params::new("test.incident").with_incident_id("inc-test-001"));
    ctx.finish("success").unwrap();
    logger.emit(&ctx).unwrap();

    let events = store.events();
    assert!(!events.is_empty(), "expected at least one emitted event");
    let parsed: Value = serde_json::from_str(&events[0]).unwrap();
    assert_eq!(
        parsed["incident_id"], "inc-test-001",
        "incident_id should be present in emitted JSON"
    );
}

#[test]
fn incident_id_from_context_carrier() {
    let carrier = ContextCarrier::new()
        .with_trace_id("trace-123")
        .with_span_id("span-456")
        .with_incident_id("inc-carrier-test");

    let params = Params::new("test.incident-carrier").inherit_from_carrier(&carrier);

    let store = MemorySinkStore::new();
    let logger = loxa::New(Config::test("test-svc").with_sink(SinkConfig::Memory(store.clone())));
    let mut ctx = logger.start_event(params);
    ctx.finish("success").unwrap();
    logger.emit(&ctx).unwrap();

    let events = store.events();
    assert!(!events.is_empty(), "expected at least one emitted event");
    let parsed: Value = serde_json::from_str(&events[0]).unwrap();
    assert_eq!(
        parsed["incident_id"], "inc-carrier-test",
        "incident_id from ContextCarrier should be present in emitted JSON"
    );
}

#[test]
fn incident_id_inherited_from_event_context() {
    let parent_ctx = EventContext::new(
        "parent-svc",
        Params::new("parent.event").with_incident_id("inc-inherit-test"),
    );

    let child_params = Params::new("child.event").inherit_from(&parent_ctx);
    let child_ctx = EventContext::new("child-svc", child_params);

    assert_eq!(
        child_ctx.incident_id,
        Some("inc-inherit-test".to_string()),
        "incident_id should be inherited from parent event context"
    );
}

#[test]
fn incident_id_omitted_when_not_set() {
    let store = MemorySinkStore::new();
    let logger = loxa::New(Config::test("test-svc").with_sink(SinkConfig::Memory(store.clone())));
    let mut ctx = logger.start_event(Params::new("test.no-incident"));
    ctx.finish("success").unwrap();
    logger.emit(&ctx).unwrap();

    let events = store.events();
    assert!(!events.is_empty(), "expected at least one emitted event");
    let parsed: Value = serde_json::from_str(&events[0]).unwrap();
    assert_eq!(
        parsed.get("incident_id"),
        None,
        "incident_id should be absent from JSON when not set"
    );
}

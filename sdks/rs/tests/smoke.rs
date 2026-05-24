use loxa::{Config, New, Params, LOXA_EVENT_VERSION, LOXA_SPEC_VERSION};
use serde_json::Value;

#[test]
fn emit_smoke() {
    let logger = New(Config::test("test"));
    let mut ctx = logger.start_event(
        Params::new("test.run")
            .with_kind("cli")
            .with_message("hello"),
    );
    logger.enrich(&mut ctx, "answer", 42);
    logger.merge(&mut ctx, "user", "id", "user_001");
    logger.merge(&mut ctx, "tenant", "id", "tenant_001");
    let _ = logger.finish(&mut ctx, "success");

    let encoded = logger.emit(&ctx).unwrap();
    let payload: Value = serde_json::from_str(&encoded).unwrap();

    assert_eq!(payload["schema_version"], LOXA_SPEC_VERSION);
    assert_eq!(payload["event_version"], LOXA_EVENT_VERSION);
    assert_eq!(payload["service"], "test");
    assert_eq!(payload["event"], "test.run");
    assert_eq!(payload["kind"], "cli");
    assert_eq!(payload["user"]["id"], "user_001");
    assert_eq!(payload["tenant"]["id"], "tenant_001");
}

#[test]
fn finish_error_sets_error_payload() {
    let logger = New(Config::test("test"));
    let mut ctx = logger.start_event(Params::new("test.error"));
    let _ = logger.finish_error(&mut ctx, "boom");
    let encoded = logger.emit(&ctx).unwrap();
    let payload: Value = serde_json::from_str(&encoded).unwrap();
    assert_eq!(payload["outcome"], "error");
    assert_eq!(payload["error"]["message"], "boom");
}

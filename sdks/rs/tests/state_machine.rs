use loza::{Config, EventContext, LozaError, New, Params, SinkConfig};

#[test]
fn duplicate_emit_is_idempotent_and_finish_after_emit_are_typed() {
    let logger = New(Config::test("checkout").with_sink(SinkConfig::Noop));
    let mut ctx = logger.start_event(Params::new("state.machine"));
    logger.finish(&mut ctx, "success").unwrap();
    let first = logger.emit(&ctx).unwrap();
    let second = logger.emit(&ctx).unwrap();
    assert_eq!(first, second);
    assert!(matches!(
        logger.finish(&mut ctx, "success"),
        Err(LozaError::EventClosed { .. })
    ));
}

#[test]
fn validation_failure_does_not_mark_emitted() {
    let logger = New(Config::test("checkout"));
    let ctx = EventContext::new("", Params::new("bad"));
    assert!(matches!(logger.emit(&ctx), Err(LozaError::Validation(_))));
    assert!(!ctx.is_emitted());
    assert_eq!(ctx.lifecycle_state(), "failed_validation");
}

#[test]
fn created_transitions_to_active_on_enrich() {
    let logger = New(Config::test("checkout").with_sink(SinkConfig::Noop));
    let mut ctx = logger.start_event(Params::new("state.transition"));
    assert_eq!(ctx.lifecycle_state(), "created");
    logger.enrich(&mut ctx, "user.id", "u1");
    assert_eq!(ctx.lifecycle_state(), "active");
}

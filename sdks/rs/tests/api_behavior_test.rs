use loxa::{
    AnySampler, CollectorSink, Config, ContextCarrier, EventContext, HashString, Logger,
    MarkSensitive, NotSampler, Params, SampleErrors, SampleRandom, SensitiveString, SinkConfig,
    StartEvent, TryNew,
};

#[test]
fn collector_sink_matches_local_collector_default() {
    match CollectorSink() {
        SinkConfig::HttpBatch { endpoint, .. } => {
            assert_eq!(endpoint, "http://127.0.0.1:9090/v1/events");
        }
        other => panic!("expected HTTP batch sink, got {other:?}"),
    }
}

#[test]
fn start_event_inherits_parent_ids() {
    let mut parent = EventContext::new("checkout", Params::new("parent"));
    parent.trace_id = Some("trace_123".to_string());
    parent.span_id = Some("span_123".to_string());

    let child = StartEvent(Some(&parent), Params::new("child"));
    assert_eq!(child.request_id, parent.request_id);
    assert_eq!(child.trace_id.as_deref(), Some("trace_123"));
    assert_eq!(child.span_id.as_deref(), Some("span_123"));
    assert_eq!(child.service, "checkout");
}

#[test]
fn start_event_inherits_context_carrier_ids() {
    let carrier = ContextCarrier::new()
        .with_request_id("req_123")
        .with_trace_id("0af7651916cd43dd8448eb211c80319c")
        .with_span_id("b7ad6b7169203331");

    let ctx = StartEvent(Some(&carrier), Params::new("child"));
    assert_eq!(ctx.request_id, "req_123");
    assert_eq!(
        ctx.trace_id.as_deref(),
        Some("0af7651916cd43dd8448eb211c80319c")
    );
    assert_eq!(ctx.span_id.as_deref(), Some("b7ad6b7169203331"));
}

#[test]
fn context_carrier_parses_traceparent() {
    let carrier =
        ContextCarrier::from_traceparent("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
            .expect("traceparent should parse");
    assert_eq!(
        carrier.trace_id.as_deref(),
        Some("0af7651916cd43dd8448eb211c80319c")
    );
    assert_eq!(carrier.span_id.as_deref(), Some("b7ad6b7169203331"));
}

#[test]
fn sensitive_helpers_mark_attributes() {
    assert!(SensitiveString("user.email", "a@example.com").sensitive);
    assert!(HashString("user.email", "a@example.com").hash_value);
    assert!(MarkSensitive(loxa::String("token", "secret")).sensitive);
}

#[test]
fn sampler_combinators_are_real() {
    let allow_logger = Logger::new(
        Config::test("checkout").with_sampler(AnySampler(&[loxa::SampleNone(), SampleErrors()])),
    );
    let deny_logger =
        Logger::new(Config::test("checkout").with_sampler(NotSampler(SampleErrors())));
    let random_deny = Logger::new(
        Config::test("checkout").with_sampler(AnySampler(&[loxa::SampleNone(), SampleRandom(0.0)])),
    );

    let mut allow_ctx = EventContext::new("checkout", Params::new("request"));
    let _ = allow_ctx.finish_error("boom");
    assert!(!allow_logger.emit(&allow_ctx).unwrap().is_empty());

    let mut deny_ctx = EventContext::new("checkout", Params::new("request"));
    let _ = deny_ctx.finish_error("boom");
    assert!(deny_logger.emit(&deny_ctx).unwrap().is_empty());

    let mut random_ctx = EventContext::new("checkout", Params::new("request"));
    let _ = random_ctx.finish_error("boom");
    assert!(random_deny.emit(&random_ctx).unwrap().is_empty());
}

#[test]
fn try_new_rejects_invalid_config() {
    let cfg = Config::test("").with_environment("test");
    let err = TryNew(cfg).expect_err("strict config without service should fail");
    assert!(matches!(err, loxa::LoxaError::Validation(_)));
}

#[test]
fn oversized_events_return_validation_error() {
    let logger = Logger::try_new(Config::test("checkout")).expect("valid config");
    let mut ctx = logger.start_event(Params::new("oversized"));
    logger.set(&mut ctx, "payload", "x".repeat(300_000));
    logger
        .finish(&mut ctx, "success")
        .expect("finish should work");
    let err = logger.emit(&ctx).expect_err("oversized event should fail");
    assert!(matches!(err, loxa::LoxaError::Validation(_)));
}

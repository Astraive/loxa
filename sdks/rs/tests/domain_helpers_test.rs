use ::loza::*;

#[test]
fn test_domain_constructors() {
    let _ = PaymentID("pay_123");
    let _ = SubscriptionID("sub_456");
    let _ = InvoiceID("inv_789");
    let _ = JobID("job_001");
    let _ = MessageID("msg_002");
    let _ = CorrelationID("corr_003");
    let _ = CommitSHA("abc123def456");
    let _ = Release("v1.2.3");
    let _ = Money(19.99);
    let _ = Percent(50.0);
    let _ = Bytes(4096);
    let _ = HTTPStatus(200);
    let _ = Bucket("my-bucket");
    let _ = Tags(vec!["tag1", "tag2"]);
    let _ = Masked("secret");
    let _ = URL("https://example.com");
    let _ = EmailHash("user@example.com");
    let _ = IPHash("192.168.1.1");
}

#[test]
fn test_domain_packs() {
    let _ = CheckoutCartItemCount(3);
    let _ = CheckoutCartTotal(59.99);
    let _ = CheckoutPaymentMethod("credit_card");
    let _ = CheckoutStatus("completed");
    let _ = PaymentProvider("stripe");
    let _ = PaymentMethod("card");
    let _ = PaymentIntentID("pi_xyz");
    let _ = PaymentFailureCode("card_declined");
    let _ = PaymentRetryAttempt(2);
    let _ = BillingPlan("premium");
    let _ = BillingSubscriptionID("bs_001");
    let _ = BillingInvoiceID("bi_002");
    let _ = BillingAmount(29.99);
    let _ = BillingInterval("monthly");
    let _ = AgentName("assistant");
    let _ = AgentProvider("openai");
    let _ = AgentModel("gpt-4");
    let _ = AgentRunType("chat");
    let _ = AgentToolName("search");
    let _ = AgentToolOutcome("success");
    let _ = AgentInputTokens(150);
    let _ = AgentOutputTokens(300);
    let _ = AgentCost(0.015);
    let _ = RAGIndex("docs");
    let _ = RAGEmbeddingModel("text-embedding-3");
    let _ = RAGChunksRetrieved(5);
    let _ = RAGTopScore(0.95);
    let _ = RAGQueryHash("a1b2c3");
    let _ = RAGCitationCount(3);
    let _ = RAGRetrievalLatency(42);
}

#[test]
fn test_lifecycle_extras() {
    let mut ctx = start_event(None, Params::new("test"));
    Drop(&mut ctx, "timeout");
    assert_eq!(ctx.outcome, Some("dropped".to_string()));

    let mut ctx2 = start_event(None, Params::new("test"));
    Cancel(&mut ctx2);
    assert_eq!(ctx2.outcome, Some("cancelled".to_string()));

    let mut ctx3 = start_event(None, Params::new("test"));
    Abandon(&mut ctx3);
    assert_eq!(ctx3.outcome, Some("abandoned".to_string()));

    let mut ctx4 = start_event(None, Params::new("test"));
    Retry(&mut ctx4);
    assert_eq!(ctx4.outcome, Some("retried".to_string()));

    let mut ctx5 = start_event(None, Params::new("test"));
    Partial(&mut ctx5, "not_finished");
    assert_eq!(ctx5.outcome, Some("partial".to_string()));
    assert!(ctx5.partial);

    let cloned = CloneEvent(&ctx5);
    assert_eq!(cloned.event_id, ctx5.event_id);
    assert_eq!(cloned.outcome, ctx5.outcome);
}

#[test]
fn test_lifecycle_snake_case() {
    let mut ctx = start_event(None, Params::new("test"));
    drop(&mut ctx, "timeout");
    assert_eq!(ctx.outcome, Some("dropped".to_string()));

    let mut ctx2 = start_event(None, Params::new("test"));
    cancel(&mut ctx2);
    assert_eq!(ctx2.outcome, Some("cancelled".to_string()));

    let mut ctx3 = start_event(None, Params::new("test"));
    abandon(&mut ctx3);
    assert_eq!(ctx3.outcome, Some("abandoned".to_string()));

    let mut ctx4 = start_event(None, Params::new("test"));
    retry(&mut ctx4);
    assert_eq!(ctx4.outcome, Some("retried".to_string()));
}

#[test]
fn test_process_group_timer_extras() {
    let mut ctx = start_event(None, Params::new("test"));
    Measure(&mut ctx, "phase1", |_e| {});
    Step(&mut ctx, "step1", |_e| {});
    Phase(&mut ctx, "group1", |_e| {});
    Span(&mut ctx, "span1", |_e| {});

    WithProcess(&mut ctx, "custom", |handle, e| {
        handle.finish(e, &[]);
    });
    WithGroup(&mut ctx, "custom_group", |handle, e| {
        handle.finish(e, &[]);
    });
    WithTimer(&mut ctx, "custom_timer", |handle, e| {
        handle.stop(e, &[]);
    });
}

#[test]
fn test_process_group_timer_snake_case() {
    let mut ctx = start_event(None, Params::new("test"));
    measure(&mut ctx, "phase1", |_e| {});
    step(&mut ctx, "step1", |_e| {});
    phase(&mut ctx, "group1", |_e| {});
    span(&mut ctx, "span1", |_e| {});
}

#[test]
fn test_logging_helpers() {
    let _ = Event("my.event");
    let _ = Audit("security.audit");
    let _ = Security("access.control");
    let _ = Metric("requests.count");
    Count("test.count", 42);
    Gauge("test.gauge", std::f64::consts::PI);
    Histogram("test.hist", 0.5);
    Breadcrumb("navigated to page");
}

#[test]
fn test_config_extras() {
    let _cfg = DisabledConfig();
    let _cfg2 = FromEnv();
}

#[test]
fn test_sink_extras() {
    let _ = MultiSink(&[]);
    let _ = OtlpSink("http://localhost:4317");
    let sink = MemorySink();
    Drain(&sink);
    Pause(&sink);
    Resume(&sink);
    let _ = QueueSize(&sink);
    assert!(Health(&sink));
}

#[test]
fn test_sampling_policy_extras() {
    let mut ctx = start_event(None, Params::new("test"));
    Finish(&mut ctx);
    assert!(ShouldSample(&ctx, &SampleAll()));
    assert!(!ShouldSample(&ctx, &SampleNone()));
    let _ = SampleByEvent(|_| true);
    let _ = SampleByOutcome(&["error"]);
    let _ = AllowFields(&["safe.key"]);
    let _ = BlockFields(&["secret.key"]);
}

#[test]
fn test_sampling_snake_case() {
    let mut ctx = start_event(None, Params::new("test"));
    finish(&mut ctx);
    assert!(should_sample(&ctx, &sample_all()));
    assert!(!should_sample(&ctx, &sample_none()));
}

#[test]
fn test_sink_snake_case() {
    let _ = multi_sink(&[]);
    let _ = otlp_sink("http://localhost:4317");
    let _ = mock_sink();
    let mem = mock_sink();
    let _ = queue_size(&mem);
    assert!(health(&mem));
}

#[test]
fn test_testing_extras() {
    let config = Config::test("test-svc");
    let _logger = New(config);
    let _ = MockSink();
    FakeClock(0);
    SetIDGenerator(|| "test-id".to_string());
}

#[test]
fn test_notice_level() {
    assert_eq!(LevelNotice, "notice");
}

#[test]
fn test_has_event_snake_case() {
    let ctx = start_event(None, Params::new("test"));
    let _ = has_event(&ctx);
}

#[test]
fn test_domain_helpers_snake_case() {
    let _ = payment_id("pay_123");
    let _ = subscription_id("sub_456");
    let _ = invoice_id("inv_789");
    let _ = job_id("job_001");
    let _ = message_id("msg_002");
    let _ = correlation_id("corr_003");
    let _ = commit_sha("abc123");
    let _ = release("v1.0");
    let _ = money(9.99);
    let _ = percent(75.0);
    let _ = bytes(2048);
    let _ = http_status(404);
    let _ = bucket("assets");
    let _ = tags(vec!["a", "b"]);
    let _ = masked("pii");
    let _ = url("https://example.com");
    let _ = email_hash("test@test.com");
    let _ = ip_hash("10.0.0.1");
}

#[test]
fn test_domain_packs_snake_case() {
    let _ = checkout_cart_item_count(1);
    let _ = checkout_cart_total(10.99);
    let _ = checkout_payment_method("paypal");
    let _ = checkout_status("pending");
    let _ = payment_provider("square");
    let _ = payment_method("wallet");
    let _ = payment_intent_id("pi_abc");
    let _ = payment_failure_code("insufficient_funds");
    let _ = payment_retry_attempt(1);
    let _ = billing_plan("basic");
    let _ = billing_subscription_id("bs_xyz");
    let _ = billing_invoice_id("bi_abc");
    let _ = billing_amount(5.99);
    let _ = billing_interval("yearly");
    let _ = agent_name("bot");
    let _ = agent_provider("anthropic");
    let _ = agent_model("claude-3");
    let _ = agent_run_type("completion");
    let _ = agent_tool_name("calculator");
    let _ = agent_tool_outcome("error");
    let _ = agent_input_tokens(100);
    let _ = agent_output_tokens(200);
    let _ = agent_cost(0.01);
    let _ = rag_index("knowledge");
    let _ = rag_embedding_model("ada-002");
    let _ = rag_chunks_retrieved(10);
    let _ = rag_top_score(0.88);
    let _ = rag_query_hash("deadbeef");
    let _ = rag_citation_count(7);
    let _ = rag_retrieval_latency(100);
}

#[test]
fn test_config_extras_snake_case() {
    let _ = disabled_config();
    let _ = from_env();
}

#[test]
fn test_sampling_policy_snake_case() {
    let _ = allow_fields(&["safe"]);
    let _ = block_fields(&["danger"]);
}

#[test]
fn test_snapshot_event() {
    let ctx = start_event(None, Params::new("snap"));
    let json = SnapshotEvent(&ctx);
    assert!(json.contains("\"event\":\"snap\""));
}

#[test]
fn test_snapshot_snake_case() {
    let ctx = start_event(None, Params::new("snap"));
    let json = snapshot_event(&ctx);
    assert!(json.contains("\"event\":\"snap\""));
}

#[test]
fn test_with_process_integration() {
    let mut ctx = start_event(None, Params::new("test"));
    WithProcess(&mut ctx, "extraction", |handle, e| {
        handle.finish(e, &[String("source", "db")]);
    });
    assert!(!ctx.processes.is_empty());
    let first = &ctx.processes[0];
    assert_eq!(
        first.get("name").and_then(|v| v.as_str()),
        Some("extraction")
    );
    assert_eq!(first.get("source").and_then(|v| v.as_str()), Some("db"));
}

#[test]
fn test_with_timer_integration() {
    let mut ctx = start_event(None, Params::new("test"));
    WithTimer(&mut ctx, "api_call", |handle, e| {
        std::thread::sleep(std::time::Duration::from_millis(1));
        handle.stop(e, &[String("endpoint", "/users")]);
    });
    assert!(!ctx.timers.is_empty());
    let first = &ctx.timers[0];
    assert_eq!(first.get("name").and_then(|v| v.as_str()), Some("api_call"));
    assert!(
        first
            .get("duration_ms")
            .and_then(|v| v.as_u64())
            .unwrap_or(0)
            > 0
    );
}

#[test]
fn test_finish_group_error() {
    let mut ctx = start_event(None, Params::new("test"));
    let handle = ctx.start_group("db_query");
    FinishGroupError(handle, &mut ctx, "connection timeout");
    assert!(!ctx.groups.is_empty());
    let first = &ctx.groups[0];
    assert_eq!(first.get("name").and_then(|v| v.as_str()), Some("db_query"));
}

#[test]
fn test_collector_api_stubs() {
    let client = CollectorHttpClient::new("http://localhost:9308");
    // Test client construction and URL formatting without making HTTP calls
    assert_eq!(client.tail_endpoint(), "http://localhost:9308/tail");
    assert_eq!(client.sdk_name, "loza-rs");
    assert_eq!(client.sdk_version, "0.2.6");
    // Envelope building test
    let envelope = client.envelope(&["{\"event\":\"test\"}".to_string()]);
    assert_eq!(
        envelope.get("api_version").and_then(|v| v.as_str()),
        Some("v1")
    );
    // Validate (local, no HTTP call) should work
    let result = client.validate(&["{\"event\":\"test\"}".to_string()]);
    assert!(result.is_ok());
    assert_eq!(result.unwrap().status_code, 200);
}

#[test]
fn test_drain_sink() {
    let sink = MemorySink();
    let cfg = Config::test("drain-test").with_sink(sink.clone());
    let logger = New(cfg);
    let ctx = logger.start_event(Params::new("drain.event"));
    let _ = logger.emit(&ctx);
    Drain(&sink);
}

#[test]
fn test_wrap_event() {
    let mut ctx = start_event(None, Params::new("wrap-test"));
    Wrap(&mut ctx, |e| {
        e.append_attr(String("wrapped", "true"));
    });
    assert_eq!(
        ctx.attrs.get("wrapped").and_then(|v| v.as_str()),
        Some("true")
    );
}

#[test]
fn test_current_event_returns_none() {
    assert!(CurrentEvent().is_none());
}

#[test]
fn test_bind_event_clones() {
    let ctx = start_event(None, Params::new("bind-test"));
    let config = Config::test("bind-svc");
    let logger = New(config);
    let bound = BindEvent(&logger, &ctx);
    assert_eq!(bound.event_id, ctx.event_id);
}

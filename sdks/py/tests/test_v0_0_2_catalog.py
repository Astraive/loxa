from __future__ import annotations

import json
import hashlib
from datetime import datetime, timedelta, timezone

import loxa


def test_level_notice():
    assert loxa.LevelNotice == "notice"
    assert loxa.ParseLevel("notice") == "notice"


def test_notice_facade():
    logger, sink = loxa.TestLogger()
    logger.notice("test notice", foo="bar")
    payload = json.loads(sink.events[0])
    assert payload["level"] == "notice"
    assert payload["message"] == "test notice"


def test_event_facade():
    logger, sink = loxa.TestLogger()
    logger.event("my.event", foo="bar")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "my.event"


def test_track_facade():
    logger, sink = loxa.TestLogger()
    logger.track("page_view", page="/home")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "page_view"


def test_audit_facade():
    logger, sink = loxa.TestLogger()
    logger.audit("user.login", user_id="abc")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "user.login"


def test_security_facade():
    logger, sink = loxa.TestLogger()
    logger.security("auth.fail", reason="bad_password")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "auth.fail"
    assert payload["level"] == "warn"


def test_metric_facade():
    logger, sink = loxa.TestLogger()
    logger.metric("api.latency", 42, unit="ms")
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["metric_value"] == 42
    assert payload["attrs"]["unit"] == "ms"


def test_count_facade():
    logger, sink = loxa.TestLogger()
    logger.count("api.requests", 5)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["count"] == 5
    assert payload["attrs"]["metric_kind"] == "count"


def test_gauge_facade():
    logger, sink = loxa.TestLogger()
    logger.gauge("cpu.usage", 78.5)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["gauge"] == 78.5
    assert payload["attrs"]["metric_kind"] == "gauge"


def test_histogram_facade():
    logger, sink = loxa.TestLogger()
    logger.histogram("response.size", 2048.0)
    payload = json.loads(sink.events[0])
    assert payload["attrs"]["histogram_value"] == 2048.0
    assert payload["attrs"]["metric_kind"] == "histogram"


def test_breadcrumb_facade():
    logger, sink = loxa.TestLogger()
    logger.breadcrumb("nav.click", button="submit")
    payload = json.loads(sink.events[0])
    assert payload["event"] == "nav.click"
    assert payload["kind"] == "checkpoint"
    assert payload["level"] == "debug"


def test_drop_cancel_abandon_retry_partial():
    logger, sink = loxa.TestLogger()

    ctx = logger.start_event(loxa.Params(event="test.drop"))
    logger.drop(ctx)
    assert ctx.outcome == "abandoned"
    assert ctx.emitted

    ctx = logger.start_event(loxa.Params(event="test.cancel"))
    logger.cancel(ctx)
    assert ctx.outcome == "cancelled"

    ctx = logger.start_event(loxa.Params(event="test.abandon"))
    logger.abandon(ctx)
    assert ctx.outcome == "abandoned"

    ctx = logger.start_event(loxa.Params(event="test.retry"))
    logger.retry(ctx)
    assert ctx.outcome == "retried"

    ctx = logger.start_event(loxa.Params(event="test.partial"))
    logger.partial(ctx, reason="timeout")
    assert ctx.partial
    assert ctx.partial_reason == "timeout"


def test_clone_event():
    logger, _ = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.original"))
    cloned = logger.clone_event(ctx)
    assert cloned.event_id == ctx.event_id
    assert cloned.params.event == "test.original"
    assert cloned is not ctx


def test_link_event():
    logger, _ = loxa.TestLogger()
    parent = logger.start_event(loxa.Params(event="parent", trace_id="trace1"))
    child = logger.start_event(loxa.Params(event="child", trace_id="trace2"))
    logger.link_event(parent, child)
    assert parent.params.links == [child.event_id]


def test_wrap_success():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.wrap"))
    result = logger.wrap(ctx, lambda x: x + 1, 41)
    assert result == 42
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert payload["outcome"] == "success"


def test_wrap_error():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.wrap.fail"))
    try:
        logger.wrap(ctx, lambda: 1 / 0)
    except ZeroDivisionError:
        pass
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert payload["outcome"] == "error"


def test_domain_helpers_snake():
    assert loxa.payment_id("pay_123").key == "payment.id"
    assert loxa.subscription_id("sub_123").key == "payment.subscription_id"
    assert loxa.invoice_id("inv_123").key == "payment.invoice_id"
    assert loxa.job_id("job_123").key == "job.id"
    assert loxa.message_id("msg_123").key == "message.id"
    assert loxa.correlation_id("cor_123").key == "correlation_id"
    assert loxa.commit_sha("abc123").key == "commit.sha"
    assert loxa.release("v1.0").key == "release"
    assert loxa.region("us-east-1").key == "region"


def test_domain_helpers_pascal():
    assert loxa.PaymentID("pay_123").key == "payment.id"
    assert loxa.SubscriptionID("sub_123").key == "payment.subscription_id"
    assert loxa.InvoiceID("inv_123").key == "payment.invoice_id"
    assert loxa.JobID("job_123").key == "job.id"
    assert loxa.MessageID("msg_123").key == "message.id"
    assert loxa.CorrelationID("cor_123").key == "correlation_id"
    assert loxa.CommitSHA("abc123").key == "commit.sha"
    assert loxa.Release("v1.0").key == "release"
    assert loxa.Region("us-east-1").key == "region"


def test_money():
    attr = loxa.money("price", 1000, "USD")
    assert attr.key == "price"
    assert attr.value == {"amount_cents": 1000, "currency": "USD"}


def test_percent():
    attr = loxa.percent("cpu", 85.5)
    assert attr.key == "cpu"
    assert attr.value == 85.5


def test_bytes_attr():
    attr = loxa.bytes_attr("file.size", 1024)
    assert attr.key == "file.size"
    assert attr.value == 1024


def test_http_status():
    attr = loxa.http_status("response.status", 200)
    assert attr.key == "response.status"
    assert attr.value == 200


def test_status_code():
    attr = loxa.StatusCode("code", 404)
    assert attr.value == 404


def test_error_code():
    attr = loxa.error_code("err.code", "E123")
    assert attr.value == "E123"


def test_bucket():
    attr = loxa.bucket("user.tier", "premium")
    assert attr.key == "user.tier"
    assert attr.value == "premium"


def test_tags():
    attr = loxa.tags("env", "prod", "staging")
    assert attr.key == "env"


def test_masked():
    attr = loxa.masked("card", "4111111111111111")
    assert attr.value == "41************11"
    attr2 = loxa.masked("card", "ab")
    assert "****" in attr2.value


def test_url():
    attr = loxa.url("https://example.com")
    assert attr.key == "url"
    assert attr.value == "https://example.com"


def test_email_hash():
    attr = loxa.email_hash("User@Example.COM")
    assert attr.key == "email.hash"
    expected = hashlib.sha256(b"user@example.com").hexdigest()
    assert attr.value == expected


def test_ip_hash():
    attr = loxa.ip_hash("192.168.1.1")
    assert attr.key == "ip.hash"
    expected = hashlib.sha256(b"192.168.1.1").hexdigest()
    assert attr.value == expected


def test_money_pascal():
    attr = loxa.Money("price", 500, "EUR")
    assert attr.key == "price"
    assert attr.value == {"amount_cents": 500, "currency": "EUR"}


def test_percent_pascal():
    attr = loxa.Percent("mem", 72.3)
    assert attr.key == "mem"


def test_bytes_pascal():
    attr = loxa.Bytes("disk", 2048)
    assert attr.key == "disk"


def test_http_status_pascal():
    attr = loxa.HTTPStatus("status", 503)
    assert attr.key == "status"


def test_error_code_pascal():
    attr = loxa.error_code("code", "ERR_BAD")
    assert attr.key == "code"


def test_bucket_pascal():
    attr = loxa.Bucket("env", "prod")
    assert attr.key == "env"


def test_tags_pascal():
    attr = loxa.Tags("region", "us", "eu")
    assert attr.key == "region"


def test_masked_pascal():
    attr = loxa.Masked("ssn", "123456789")
    assert attr.value == "12*****89"


def test_url_pascal():
    attr = loxa.URL("https://loxa.dev")
    assert attr.key == "url"


def test_email_hash_pascal():
    attr = loxa.EmailHash("test@test.com")
    assert attr.key == "email.hash"


def test_ip_hash_pascal():
    attr = loxa.IPHash("10.0.0.1")
    assert attr.key == "ip.hash"


def test_domain_packs():
    assert loxa.checkout_cart_item_count(3).key == "checkout.cart_item_count"
    assert loxa.checkout_cart_total(2999).key == "checkout.cart_total"
    assert loxa.checkout_payment_method("card").key == "checkout.payment_method"
    assert loxa.checkout_status("completed").key == "checkout.status"
    assert loxa.payment_provider("stripe").key == "payment.provider"
    assert loxa.payment_method("credit_card").key == "payment.method"
    assert loxa.payment_intent_id("pi_123").key == "payment.intent_id"
    assert loxa.payment_failure_code("card_declined").key == "payment.failure_code"
    assert loxa.payment_retry_attempt(2).key == "payment.retry_attempt"
    assert loxa.billing_plan("pro").key == "billing.plan"
    assert loxa.billing_subscription_id("bs_123").key == "billing.subscription_id"
    assert loxa.billing_invoice_id("bi_123").key == "billing.invoice_id"
    assert loxa.billing_amount(999).key == "billing.amount"
    assert loxa.billing_interval("monthly").key == "billing.interval"
    assert loxa.agent_name("assistant").key == "agent.name"
    assert loxa.agent_provider("openai").key == "agent.provider"
    assert loxa.agent_model("gpt-4").key == "agent.model"
    assert loxa.agent_run_type("chat").key == "agent.run_type"
    assert loxa.agent_tool_name("search").key == "agent.tool_name"
    assert loxa.agent_tool_outcome("success").key == "agent.tool_outcome"
    assert loxa.agent_input_tokens(100).key == "agent.input_tokens"
    assert loxa.agent_output_tokens(50).key == "agent.output_tokens"
    assert loxa.agent_cost(0.002).key == "agent.cost"
    assert loxa.rag_index("docs").key == "rag.index"
    assert loxa.rag_embedding_model("text-embedding-3").key == "rag.embedding_model"
    assert loxa.rag_chunks_retrieved(5).key == "rag.chunks_retrieved"
    assert loxa.rag_top_score(0.95).key == "rag.top_score"
    assert loxa.rag_query_hash("abc123").key == "rag.query_hash"
    assert loxa.rag_citation_count(3).key == "rag.citation_count"
    assert loxa.rag_retrieval_latency(150).key == "rag.retrieval_latency"


def test_domain_packs_pascal():
    assert loxa.CheckoutCartItemCount(3).key == "checkout.cart_item_count"
    assert loxa.PaymentProvider("stripe").key == "payment.provider"
    assert loxa.PaymentIntentID("pi_123").key == "payment.intent_id"
    assert loxa.BillingPlan("pro").key == "billing.plan"
    assert loxa.BillingSubscriptionID("bs_123").key == "billing.subscription_id"
    assert loxa.AgentName("assistant").key == "agent.name"
    assert loxa.AgentModel("gpt-4").key == "agent.model"
    assert loxa.AgentCost(0.002).key == "agent.cost"
    assert loxa.RAGIndex("docs").key == "rag.index"
    assert loxa.RAGEmbeddingModel("text-embedding-3").key == "rag.embedding_model"
    assert loxa.RAGChunksRetrieved(5).key == "rag.chunks_retrieved"
    assert loxa.RAGTopScore(0.95).key == "rag.top_score"
    assert loxa.RAGQueryHash("abc123").key == "rag.query_hash"
    assert loxa.RAGCitationCount(3).key == "rag.citation_count"
    assert loxa.RAGRetrievalLatency(150).key == "rag.retrieval_latency"


def test_timing_helpers():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.timing"))
    p = loxa.with_process(ctx, "step1")
    p.finish()
    g = loxa.with_group(ctx, "phase1")
    g.finish()
    t = loxa.with_timer(ctx, "timer1")
    t.stop()
    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert len(payload.get("processes", [])) == 1
    assert len(payload.get("groups", [])) == 1
    assert len(payload.get("timers", [])) == 1


def test_finish_group_error():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.group_err"))
    g = loxa.with_group(ctx, "phase1")
    loxa.finish_group_error(g, ValueError("bad"))
    logger.finish(ctx, "success")
    logger.emit(ctx)


def test_measure():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.measure"))
    result = loxa.measure(ctx, "op1", lambda x: x * 2, 21)
    assert result == 42
    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert len(payload.get("timers", [])) == 1


def test_step_phase_span():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.spq"))
    p = loxa.step(ctx, "step1")
    p.finish()
    g = loxa.phase(ctx, "phase1")
    g.finish()
    t = loxa.span(ctx, "span1")
    t.stop()
    logger.finish(ctx, "success")
    logger.emit(ctx)
    payload = json.loads(sink.events[0])
    assert len(payload.get("processes", [])) == 1
    assert len(payload.get("groups", [])) == 1
    assert len(payload.get("timers", [])) == 1


def test_config_disabled():
    cfg = loxa.Config.disabled()
    assert cfg.environment == "test"
    assert cfg.level == "fatal"


def test_config_from_env(monkeypatch):
    monkeypatch.setenv("LOXA_SERVICE_NAME", "mysvc")
    cfg = loxa.Config.from_env()
    assert cfg.service == "mysvc"


def test_from_env_function():
    cfg = loxa.from_env()
    assert isinstance(cfg, loxa.Config)


def test_config_options_new():
    loxa.WithRelease("v2.0")
    loxa.WithNamespace("prod")
    loxa.WithApiKey("key123")
    loxa.WithOtelBridge(True)
    loxa.WithRetry(True)
    loxa.WithTimeout(5.0)
    loxa.WithQueueSize(4096)
    loxa.WithLogger(None)


def test_snake_config_options():
    loxa.with_release("v2.0")
    loxa.with_namespace("prod")
    loxa.with_api_key("key123")
    loxa.with_otel_bridge(True)
    loxa.with_retry(True)
    loxa.with_timeout(5.0)
    loxa.with_queue_size(4096)
    loxa.with_logger(None)
    loxa.disabled()
    loxa.from_env()


def test_sampler_new():
    assert loxa.SampleByEvent(lambda e: True) is not None
    assert loxa.SampleByOutcome("error") is not None
    assert loxa.ShouldSample(lambda e: True, None) is True
    assert loxa.AllowFields("key1") is not None
    assert loxa.BlockFields("secret") is not None

    assert loxa.sample_by_event(lambda e: True) is not None
    assert loxa.sample_by_outcome("error") is not None
    assert loxa.should_sample(lambda e: True, None) is True
    assert loxa.allow_fields("key1") is not None
    assert loxa.block_fields("secret") is not None


def test_testkit_new():
    logger, sink = loxa.TestLogger()
    logger.info("hello")
    events = loxa.DecodeEvents(sink)

    found = loxa.expect_event(events, message="hello")
    assert found is not None

    loxa.expect_attr(found, "message", "hello")

    snap = loxa.snapshot_event(found)
    assert "event_id" not in snap
    assert "timestamp" not in snap


def test_mock_sink():
    s = loxa.mock_sink()
    assert isinstance(s, loxa.MemorySink)


def test_sink_helpers():
    ms = loxa.multi_sink(loxa.MemorySink(), loxa.MemorySink())
    ms.write('{"test": true}')
    assert len(ms._sinks) == 2

    s = loxa.MemorySink()
    loxa.drain(s)
    loxa.pause(s)
    loxa.resume(s)
    assert loxa.queue_size(s) == 0
    assert loxa.health(s) is True

    ns = loxa.otlp_sink()
    ns.write("test")


def test_sink_helpers_pascal():
    assert loxa.Drain is not None
    assert loxa.Pause is not None
    assert loxa.Resume is not None
    assert loxa.QueueSize is not None
    assert loxa.Health is not None
    assert loxa.OTLPSink is not None
    assert loxa.MultiSinkFactory is not None


def test_collector_client_api():
    cc = loxa.CollectorClient("http://localhost:9090/v1/events")
    assert hasattr(cc, "validate")
    assert hasattr(cc, "ingest")
    assert hasattr(cc, "query")
    assert hasattr(cc, "tail")
    assert hasattr(cc, "delete")
    assert hasattr(cc, "replay")
    assert hasattr(cc, "dlq_list")
    assert hasattr(cc, "dlq_read")
    assert hasattr(cc, "dlq_replay")
    assert hasattr(cc, "keys_create")
    assert hasattr(cc, "keys_revoke")
    assert hasattr(cc, "sinks_list")


def test_uppercase_facade_methods():
    logger, sink = loxa.TestLogger()
    logger.notice("hello notice", foo="bar")
    payload = json.loads(sink.events[-1])
    assert payload["level"] == "notice"

    logger.event("my.event")
    payload = json.loads(sink.events[-1])
    assert payload["event"] == "my.event"

    logger.track("page_view")
    logger.audit("user.login")
    logger.security("auth.fail")
    logger.metric("latency", 42)
    logger.count("requests")
    logger.gauge("cpu", 50.0)
    logger.histogram("size", 100.0)
    logger.breadcrumb("nav.click")


def test_uppercase_lifecycle_extras():
    logger, sink = loxa.TestLogger()
    ctx = logger.start_event(loxa.Params(event="test.upper"))
    logger.drop(ctx)
    assert ctx.outcome == "abandoned"

    ctx = logger.start_event(loxa.Params(event="test.cancel"))
    logger.cancel(ctx)
    assert ctx.outcome == "cancelled"

    ctx = logger.start_event(loxa.Params(event="test.abandon"))
    logger.abandon(ctx, reason="gone")

    ctx = logger.start_event(loxa.Params(event="test.retry"))
    logger.retry(ctx)

    ctx = logger.start_event(loxa.Params(event="test.partial"))
    logger.partial(ctx, reason="broken")


def test_all_pascal_aliases_exist():
    for name in [
        "PaymentID", "SubscriptionID", "InvoiceID", "JobID", "MessageID",
        "CorrelationID", "CommitSHA", "Release",
        "Money", "Percent", "Bytes", "HTTPStatus", "StatusCode",
        "Bucket", "Tags", "Masked", "URL", "EmailHash", "IPHash", "Region",
        "CheckoutCartItemCount", "CheckoutCartTotal", "CheckoutPaymentMethod", "CheckoutStatus",
        "PaymentProvider", "PaymentMethod", "PaymentIntentID", "PaymentFailureCode", "PaymentRetryAttempt",
        "BillingPlan", "BillingSubscriptionID", "BillingInvoiceID", "BillingAmount", "BillingInterval",
        "AgentName", "AgentProvider", "AgentModel", "AgentRunType", "AgentToolName", "AgentToolOutcome",
        "AgentInputTokens", "AgentOutputTokens", "AgentCost",
        "RAGIndex", "RAGEmbeddingModel", "RAGChunksRetrieved", "RAGTopScore", "RAGQueryHash",
        "RAGCitationCount", "RAGRetrievalLatency",
        "Notice", "Event", "Track", "Audit", "Security", "Metric", "Count", "Gauge", "Histogram", "Breadcrumb",
        "SampleByEvent", "SampleByOutcome", "ShouldSample", "AllowFields", "BlockFields",
        "WithRelease", "WithNamespace", "WithApiKey", "WithOtelBridge", "WithRetry", "WithTimeout", "WithQueueSize", "WithLogger",
    ]:
        assert hasattr(loxa, name), f"Missing {name}"

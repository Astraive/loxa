use loxa::{
    ComposeRedactors, Config, ContextCarrier, ContextSource, HTTPClient, HTTPRequest,
    InjectHTTPHeadersFromCarrier, New, Params, RedactKeys, SampleErrors, SampleNone, SchemaConfig,
    String as LoxaString,
};
use serde_json::Value;
use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::mpsc;
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

#[test]
fn flat_schema_and_redaction_are_real() {
    let logger = New(Config::test("checkout")
        .with_schema(SchemaConfig::Flat)
        .with_redactor(RedactKeys(&["password"])));
    let mut ctx = logger.start_event(Params::new("checkout.run"));
    logger.append(&mut ctx, loxa::UserID("u1"));
    logger.append(&mut ctx, LoxaString("password", "secret"));
    let _ = logger.finish(&mut ctx, "success");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["user_id"], "u1");
    assert_eq!(payload["attrs_password"], "[REDACTED]");
}

#[test]
fn sampler_can_drop_event() {
    let logger = New(Config::test("checkout").with_sampler(SampleNone()));
    let mut ctx = logger.start_event(Params::new("dropped"));
    let _ = logger.finish(&mut ctx, "success");

    assert_eq!(logger.emit(&ctx).unwrap(), "");
}

#[test]
fn error_sampler_keeps_failed_events() {
    let logger = New(Config::test("checkout").with_sampler(SampleErrors()));
    let mut ctx = logger.start_event(Params::new("failed"));
    let _ = logger.finish_error(&mut ctx, "boom");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["outcome"], "error");
    assert_eq!(payload["error"]["message"], "boom");
}

#[test]
fn checkpoints_are_encoded() {
    let logger = New(Config::test("checkout"));
    let mut ctx = logger.start_event(Params::new("checkpointed"));
    loxa::CheckpointWithAttrs(&mut ctx, "db_started", &[loxa::String("phase", "db")]);
    let _ = logger.finish(&mut ctx, "success");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["checkpoints"][0]["name"], "db_started");
    assert_eq!(payload["checkpoints"][0]["phase"], "db");
}

#[test]
fn composite_redactors_are_real() {
    let redactor = ComposeRedactors(&[RedactKeys(&["token"]), RedactKeys(&["password"])]);
    let logger = New(Config::test("checkout").with_redactor(redactor));
    let mut ctx = logger.start_event(Params::new("redaction.compose"));
    logger.append(&mut ctx, LoxaString("message", "hello world"));
    logger.append(&mut ctx, LoxaString("token", "abc123"));
    logger.append(&mut ctx, LoxaString("password", "secret456"));
    let _ = logger.finish(&mut ctx, "success");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["attrs"]["message"], "hello world");
    assert_eq!(payload["attrs"]["token"], "[REDACTED]");
    assert_eq!(payload["attrs"]["password"], "[REDACTED]");
}

#[test]
fn duplicate_policy_keep_both_preserves_values() {
    let logger = New(Config::test("checkout").with_duplicate_policy(loxa::KeepBoth));
    let mut ctx = logger.start_event(Params::new("duplicate.keep_both"));
    logger.append(&mut ctx, LoxaString("tag", "first"));
    logger.append(&mut ctx, LoxaString("tag", "second"));
    let _ = logger.finish(&mut ctx, "success");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["attrs"]["tag"][0], "first");
    assert_eq!(payload["attrs"]["tag"][1], "second");
}

#[test]
fn duplicate_policy_error_on_duplicate_fails_emit() {
    let logger = New(Config::test("checkout").with_duplicate_policy(loxa::ErrorOnDuplicate));
    let mut ctx = logger.start_event(Params::new("duplicate.error"));
    logger.append(&mut ctx, LoxaString("tag", "first"));
    logger.append(&mut ctx, LoxaString("tag", "second"));
    let _ = logger.finish(&mut ctx, "success");

    let err = logger.emit(&ctx).expect_err("duplicate emit should fail");
    assert!(matches!(err, loxa::LoxaError::Validation(v) if v.message.contains("duplicate field")));
}

#[test]
fn collector_endpoint_installs_default_http_sink() {
    let logger =
        New(Config::dev("checkout").with_collector_endpoint("http://127.0.0.1:9308/events"));
    let debug = format!("{logger:?}");

    assert!(debug.contains("HttpBatch"));
    assert!(debug.contains("http://127.0.0.1:9308/events"));
}

#[test]
fn config_defaults_flow_into_payload() {
    let logger = New(Config::test("checkout")
        .with_version("1.2.3")
        .with_environment("staging")
        .with_region("ap-south-1"));
    let mut ctx = logger.start_event(Params::new("defaults.flow"));
    let _ = logger.finish(&mut ctx, "success");

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();

    assert_eq!(payload["service"], "checkout");
    assert_eq!(payload["version"], "1.2.3");
    assert_eq!(payload["environment"], "staging");
    assert_eq!(payload["region"], "ap-south-1");
}

#[test]
fn async_logger_flushes_buffered_events() {
    let path = temp_file("loxa-rs-async.ndjson");
    let logger = New(Config::test("checkout")
        .with_async(true)
        .with_sink(loxa::FileSink(path.to_str().expect("temp path"))));
    let mut ctx = logger.start_event(Params::new("async.flush"));
    let _ = logger.finish(&mut ctx, "success");

    logger.emit(&ctx).expect("async emit");
    logger.flush().expect("flush async queue");

    let content = fs::read_to_string(&path).expect("read async file");
    assert!(content.contains("\"event\":\"async.flush\""));

    let _ = fs::remove_file(path);
}

#[test]
fn inject_http_headers_preserves_trace_request_and_baggage() {
    let mut headers = BTreeMap::new();
    let carrier = ContextCarrier::new()
        .with_trace_id("0af7651916cd43dd8448eb211c80319c")
        .with_span_id("b7ad6b7169203331")
        .with_tracestate("vendor=value")
        .with_request_id("req_123")
        .with_baggage("tenant", "acme");

    InjectHTTPHeadersFromCarrier(&carrier, &mut headers);

    assert_eq!(
        headers.get("traceparent").map(String::as_str),
        Some("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
    );
    assert_eq!(
        headers.get("tracestate").map(String::as_str),
        Some("vendor=value")
    );
    assert_eq!(
        headers.get("x-request-id").map(String::as_str),
        Some("req_123")
    );
    assert_eq!(
        headers.get("baggage").map(String::as_str),
        Some("tenant=acme")
    );
}

#[test]
fn http_client_injects_headers_and_records_checkpoints() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
    let addr = listener.local_addr().expect("local addr");
    let (tx, rx) = mpsc::sync_channel(1);
    let server = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept request");
        let mut buf = [0_u8; 4096];
        let bytes = stream.read(&mut buf).expect("read request");
        tx.send(String::from_utf8_lossy(&buf[..bytes]).to_string())
            .expect("capture request");
        stream
            .write_all(b"HTTP/1.1 201 Created\r\nContent-Length: 2\r\nx-test: ok\r\n\r\nok")
            .expect("write response");
    });

    let logger = New(Config::test("checkout"));
    let carrier = ContextCarrier::new()
        .with_request_id("req_123")
        .with_trace_id("0af7651916cd43dd8448eb211c80319c")
        .with_span_id("b7ad6b7169203331");
    let mut ctx = logger.start_event(carrier.inherit_params(Params::new("http.client.test")));

    let response = HTTPClient::with_timeout_ms(2_000)
        .send_with_context(
            &mut ctx,
            &HTTPRequest::new("GET", format!("http://{addr}/hello"))
                .with_header("accept", "text/plain"),
        )
        .expect("send request");
    logger.finish(&mut ctx, "success").expect("finish event");

    let raw_request = rx.recv().expect("captured request");
    server.join().expect("server join");

    let normalized_request = raw_request.to_ascii_lowercase();
    assert!(normalized_request
        .contains("traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"));
    assert!(normalized_request.contains("x-request-id: req_123"));
    assert_eq!(response.status_code, 201);
    assert_eq!(response.body, "ok");
    assert_eq!(
        response.headers.get("x-test").map(String::as_str),
        Some("ok")
    );

    let payload: Value = serde_json::from_str(&logger.emit(&ctx).unwrap()).unwrap();
    assert_eq!(payload["checkpoints"][0]["name"], "http.client.started");
    assert_eq!(payload["checkpoints"][0]["http.client.method"], "GET");
    assert_eq!(payload["checkpoints"][1]["name"], "http.client.finished");
    assert_eq!(payload["checkpoints"][1]["http.client.status_code"], 201);
}

fn temp_file(name: &str) -> PathBuf {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time")
        .as_nanos();
    std::env::temp_dir().join(format!("{nanos}-{name}"))
}

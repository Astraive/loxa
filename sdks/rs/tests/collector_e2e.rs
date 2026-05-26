// End-to-end test: loxa-rs SDK -> HTTPBatchSink -> loxa-collector
//
// Requires a live loxa-collector on http://127.0.0.1:9090
// Run: cargo test --test collector_e2e -- --test-threads=1

use loxa::{
    CollectorSinkWithEndpoint, Config, HTTPClient, HTTPRequest, New, Params, WithCollectorEndpoint,
};
use serde_json::Value;

const COLLECTOR_URL: &str = "http://127.0.0.1:9090";

fn collector_health() -> bool {
    let client = HTTPClient::with_timeout_ms(2_000);
    let req = HTTPRequest::new("GET", format!("{COLLECTOR_URL}/healthz"));
    client
        .send(&req)
        .map(|r| r.status_code == 200)
        .unwrap_or(false)
}

fn send_raw_envelope(envelope: &Value) -> (u16, String) {
    let client = HTTPClient::with_timeout_ms(5_000);
    let body = serde_json::to_vec(envelope).unwrap();
    let req = HTTPRequest::new("POST", format!("{COLLECTOR_URL}/events"))
        .with_header("Content-Type", "application/json")
        .with_body(body);
    let resp = client.send(&req).unwrap();
    (resp.status_code, resp.body)
}

// ---------------------------------------------------------------------------
// Config wiring tests (no collector needed)
// ---------------------------------------------------------------------------

#[test]
fn production_config_with_collector_endpoint_auto_wires_httpbatch() {
    let cfg = Config::production("wired_svc").with_collector_endpoint(COLLECTOR_URL);
    let logger = New(cfg);

    // Logger should have HttpBatch as the only sink
    assert!(
        logger.sink_names().contains(&"HttpBatch".to_string()),
        "expected HttpBatch sink, got: {:?}",
        logger.sink_names()
    );
    assert_eq!(logger.collector_endpoint(), COLLECTOR_URL);
}

#[test]
fn dev_config_with_collector_endpoint_auto_wires_httpbatch() {
    let cfg = Config::dev("wired_dev").with_collector_endpoint(COLLECTOR_URL);
    let logger = New(cfg);

    assert!(
        logger.sink_names().contains(&"HttpBatch".to_string()),
        "expected HttpBatch sink, got: {:?}",
        logger.sink_names()
    );
}

#[test]
fn config_option_with_collector_endpoint_auto_wires_httpbatch() {
    let cfg = loxa::NewWith(vec![WithCollectorEndpoint(COLLECTOR_URL)]);

    assert!(
        cfg.sink_names().contains(&"HttpBatch".to_string()),
        "expected HttpBatch sink, got: {:?}",
        cfg.sink_names()
    );
}

#[test]
fn explicit_file_sink_preserved_alongside_collector_endpoint() {
    let cfg = Config::test("explicit")
        .with_sink(loxa::FileSink("/tmp/loxa-e2e.log"))
        .with_collector_endpoint(COLLECTOR_URL);
    let logger = New(cfg);

    // File sink is non-terminal — it should be preserved alongside HttpBatch
    let names = logger.sink_names();
    assert!(
        names.contains(&"File".to_string()),
        "explicit File must be preserved, got: {:?}",
        names
    );
    assert!(
        names.contains(&"HttpBatch".to_string()),
        "HttpBatch must be added, got: {:?}",
        names
    );
}

#[test]
fn httpbatch_sink_not_duplicated_when_already_configured() {
    let cfg = Config::test("no_dup")
        .with_sink(CollectorSinkWithEndpoint(COLLECTOR_URL))
        .with_collector_endpoint(COLLECTOR_URL);
    let logger = New(cfg);

    let http_count = logger
        .sink_names()
        .iter()
        .filter(|n| n.as_str() == "HttpBatch")
        .count();
    assert_eq!(http_count, 1, "HttpBatch must not be duplicated");
}

#[test]
fn no_collector_endpoint_keeps_default_sink() {
    let cfg = Config::test("no_endpoint");
    let logger = New(cfg);

    // No collector endpoint = no HttpBatch auto-wire
    assert!(!logger.sink_names().contains(&"HttpBatch".to_string()));
}

// ---------------------------------------------------------------------------
// Live E2E tests (requires collector on :9090)
// ---------------------------------------------------------------------------

#[test]
fn collector_is_healthy() {
    assert!(
        collector_health(),
        "collector must be running on {COLLECTOR_URL}"
    );
}

#[test]
fn raw_envelope_accepted_by_collector() {
    assert!(collector_health());

    let envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "e2e_raw"},
        "events": [{
            "event_id": "evt_e2e_raw_001",
            "event": "e2e.raw_envelope",
            "kind": "event",
            "level": "info",
            "timestamp": "2026-05-20T12:00:00Z",
            "service": "e2e_raw",
            "schema_version": "v1",
            "event_version": "v1",
            "outcome": "success"
        }]
    });

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");

    let resp: Value = serde_json::from_str(&body).unwrap();
    assert_eq!(resp["status"], "accepted");
    assert_eq!(resp["accepted"], 1);
}

#[test]
fn batch_envelope_accepted_by_collector() {
    assert!(collector_health());

    let events: Vec<Value> = (0..5)
        .map(|i| {
            serde_json::json!({
                "event_id": format!("evt_e2e_batch_{i:03}"),
                "event": format!("e2e.batch_{i}"),
                "kind": "event",
                "level": "info",
                "timestamp": "2026-05-20T12:00:00Z",
                "service": "e2e_batch",
                "schema_version": "v1",
                "event_version": "v1",
                "outcome": "success"
            })
        })
        .collect();

    let envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "e2e_batch"},
        "events": events
    });

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");

    let resp: Value = serde_json::from_str(&body).unwrap();
    assert_eq!(resp["status"], "accepted");
    assert_eq!(resp["accepted"], 5);
}

#[test]
fn sdk_emit_produces_valid_envelope() {
    assert!(collector_health());

    let client = loxa::core::client::CollectorHttpClient::new(COLLECTOR_URL);

    let event_json = serde_json::json!({
        "event_id": "evt_e2e_sdk_001",
        "event": "e2e.sdk_emit",
        "kind": "event",
        "level": "info",
        "timestamp": "2026-05-20T12:00:00Z",
        "service": "e2e_sdk",
        "schema_version": "v1",
        "event_version": "v1",
        "outcome": "success"
    });

    let encoded = serde_json::to_string(&event_json).unwrap();
    let envelope = client.envelope(&[encoded]);

    assert_eq!(envelope["api_version"], "v1");
    assert_eq!(envelope["source"]["sdk"], "loxa-rs");
    client
        .validate_envelope(&envelope)
        .expect("envelope must pass validation");

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");
}

#[test]
fn http_event_with_trace_context_accepted() {
    assert!(collector_health());

    let envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "e2e_http"},
        "events": [{
            "event_id": "evt_e2e_http_001",
            "event": "GET /api/users",
            "kind": "http",
            "level": "info",
            "timestamp": "2026-05-20T12:00:00Z",
            "service": "e2e_http",
            "schema_version": "v1",
            "event_version": "v1",
            "method": "GET",
            "path": "/api/users",
            "status_code": 200,
            "trace_id": "0af7651916cd43dd8448eb211c80319c",
            "span_id": "b7ad6b7169203331",
            "outcome": "success"
        }]
    });

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");
}

#[test]
fn error_event_with_error_details_accepted() {
    assert!(collector_health());

    let envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "e2e_error"},
        "events": [{
            "event_id": "evt_e2e_error_001",
            "event": "e2e.error_event",
            "kind": "event",
            "level": "error",
            "timestamp": "2026-05-20T12:00:00Z",
            "service": "e2e_error",
            "schema_version": "v1",
            "event_version": "v1",
            "outcome": "error",
            "message": "Database connection failed",
            "error": {"type": "ConnectionError", "code": "DB_TIMEOUT"}
        }]
    });

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");
}

#[test]
fn enriched_event_with_attrs_accepted() {
    assert!(collector_health());

    let envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "e2e_attrs"},
        "events": [{
            "event_id": "evt_e2e_attrs_001",
            "event": "e2e.enriched",
            "kind": "event",
            "level": "info",
            "timestamp": "2026-05-20T12:00:00Z",
            "service": "e2e_attrs",
            "schema_version": "v1",
            "event_version": "v1",
            "outcome": "success",
            "user": {"id": "usr_12345"},
            "tenant": {"id": "ten_abcdef"},
            "attrs": {"region": "us-west-2", "custom_metric": 42}
        }]
    });

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");
}

#[test]
fn collector_rejects_invalid_envelope() {
    assert!(collector_health());

    let bad_envelope = serde_json::json!({
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "test"},
        "events": [{"event_id": "x", "event": "test", "kind": "event", "timestamp": "2026-05-20T12:00:00Z", "service": "test"}]
    });

    let client = loxa::core::client::CollectorHttpClient::new(COLLECTOR_URL);
    let err = client.validate_envelope(&bad_envelope);
    assert!(err.is_err(), "should reject envelope without api_version");
}

#[test]
fn collector_rejects_empty_events() {
    assert!(collector_health());

    let bad_envelope = serde_json::json!({
        "api_version": "v1",
        "source": {"sdk": "loxa-rs", "version": "0.2.0", "service": "test"},
        "events": []
    });

    let client = loxa::core::client::CollectorHttpClient::new(COLLECTOR_URL);
    let err = client.validate_envelope(&bad_envelope);
    assert!(err.is_err(), "should reject envelope with empty events");
}

// ---------------------------------------------------------------------------
// Full SDK flow: init -> emit -> collector receives
// ---------------------------------------------------------------------------

#[test]
fn full_sdk_flow_emit_to_collector() {
    assert!(collector_health());

    // 1. Init logger with collector endpoint (auto-wires HttpBatchSink)
    let logger = New(Config::production("e2e_full_flow").with_collector_endpoint(COLLECTOR_URL));

    // 2. Start an event
    let mut ctx = logger.start_event(
        Params::new("e2e.full_flow")
            .with_message("Full SDK flow test")
            .with_kind("event"),
    );

    // 3. Enrich with attributes
    logger.enrich(&mut ctx, "test_type", "full_flow");
    logger.enrich(&mut ctx, "iteration", 1_i64);

    // 4. Finish and emit
    let _ = logger.finish(&mut ctx, "success");
    let payload = logger.emit(&ctx).expect("emit must succeed");

    // 5. Verify payload is valid envelope-ready JSON
    let event: Value = serde_json::from_str(&payload).unwrap();
    assert_eq!(event["event"], "e2e.full_flow");
    assert_eq!(event["service"], "e2e_full_flow");
    assert_eq!(event["outcome"], "success");

    // 6. Build envelope and send to collector
    let client = loxa::core::client::CollectorHttpClient::new(COLLECTOR_URL);
    let envelope = client.envelope(&[payload]);
    client.validate_envelope(&envelope).expect("envelope valid");

    let (status, body) = send_raw_envelope(&envelope);
    assert_eq!(status, 202, "collector returned {status}: {body}");

    let resp: Value = serde_json::from_str(&body).unwrap();
    assert_eq!(resp["status"], "accepted");
    assert_eq!(resp["accepted"], 1);
}

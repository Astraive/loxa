use loza::{Config, New, Params, SinkConfig};
use serde_json::Value;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::thread;
use std::time::Duration;

#[test]
fn collector_sink_rejects_partial_invalid_batch() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
    let addr = listener.local_addr().expect("listener addr");

    let handle = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept request");
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("set timeout");
        let _ = read_http_request(&mut stream);
        write_json_response(
            &mut stream,
            207,
            r#"{"status":"partial","accepted":1,"invalid":1,"acks":[{"event_id":"evt_1","status":"accepted","reason":"accepted"},{"event_id":"evt_2","status":"invalid","reason":"schema_invalid","message":"schema validation failed"}]}"#,
        );
    });

    let endpoint = format!("http://{addr}/events");
    let logger = New(Config::test("checkout").with_sink(loza::HttpBatchSink(&endpoint)));
    let mut ctx = logger.start_event(Params::new("checkout.collector").with_kind("cli"));
    logger.finish(&mut ctx, "success").expect("finish event");
    let err = logger
        .emit(&ctx)
        .expect_err("partial invalid batch should fail");
    handle.join().expect("server thread");

    let message = err.to_string();
    assert!(message.contains("collector rejected batch"));
    assert!(message.contains("schema_invalid"));
}

#[test]
fn emitted_event_shape_matches_cortex_contract() {
    let logger = New(Config::test("checkout").with_sink(SinkConfig::Noop));
    let mut ctx = logger.start_event(Params::new("payment.completed").with_kind("http"));
    logger.set(&mut ctx, "tenant.id", "tenant-1");
    logger.set(&mut ctx, "request.method", "POST");
    logger.finish(&mut ctx, "success").expect("finish event");

    let encoded = logger.emit(&ctx).expect("emit event");
    let payload: Value = serde_json::from_str(&encoded).expect("parse event payload");

    for field in [
        "event_id",
        "timestamp",
        "service",
        "event",
        "kind",
        "level",
        "event_state",
        "schema_version",
        "event_version",
    ] {
        assert!(
            payload.get(field).is_some(),
            "expected cortex-consumable field {field}"
        );
    }

    assert_eq!(
        payload.get("service").and_then(Value::as_str),
        Some("checkout")
    );
    assert_eq!(
        payload.get("event").and_then(Value::as_str),
        Some("payment.completed")
    );
    assert_eq!(payload.get("kind").and_then(Value::as_str), Some("http"));
    assert_eq!(
        payload.get("event_state").and_then(Value::as_str),
        Some("finished")
    );
    assert_eq!(
        payload.pointer("/tenant/id").and_then(Value::as_str),
        Some("tenant-1")
    );
    assert_eq!(
        payload
            .pointer("/attrs/request/method")
            .and_then(Value::as_str),
        Some("POST")
    );
}

fn read_http_request(stream: &mut std::net::TcpStream) -> Vec<u8> {
    let mut raw = Vec::new();
    let mut headers_done = false;
    let mut content_length = 0usize;
    loop {
        let mut chunk = [0u8; 1024];
        let n = stream.read(&mut chunk).expect("read request");
        if n == 0 {
            break;
        }
        raw.extend_from_slice(&chunk[..n]);
        if let Some(pos) = find_headers_end(&raw) {
            if !headers_done {
                headers_done = true;
                let head = String::from_utf8_lossy(&raw[..pos]);
                content_length = parse_content_length(&head);
            }
            let body_len = raw.len() - (pos + 4);
            if body_len >= content_length {
                break;
            }
        }
    }
    raw
}

fn find_headers_end(raw: &[u8]) -> Option<usize> {
    raw.windows(4).position(|window| window == b"\r\n\r\n")
}

fn parse_content_length(headers: &str) -> usize {
    headers
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            if name.eq_ignore_ascii_case("content-length") {
                return value.trim().parse::<usize>().ok();
            }
            None
        })
        .unwrap_or(0)
}

fn write_json_response(stream: &mut std::net::TcpStream, status: u16, body: &str) {
    let status_text = match status {
        207 => "207 Multi-Status",
        202 => "202 Accepted",
        _ => "200 OK",
    };
    let response = format!(
        "HTTP/1.1 {status_text}\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
        body.len(),
        body
    );
    stream
        .write_all(response.as_bytes())
        .expect("write response");
}

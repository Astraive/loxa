use std::io::{Read, Write};
use std::net::TcpListener;
use std::thread;
use std::time::Duration;

use loxa::core::client::CollectorHttpClient;

fn start_server() -> (String, std::sync::mpsc::Sender<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
    let addr = listener.local_addr().expect("local addr");
    listener
        .set_nonblocking(true)
        .expect("set nonblocking listener");
    let (tx, rx) = std::sync::mpsc::channel::<()>();
    thread::spawn(move || loop {
        if rx.try_recv().is_ok() {
            break;
        }
        match listener.accept() {
            Ok((mut stream, _peer)) => {
                let mut buf = [0_u8; 8192];
                let _ = stream.read(&mut buf);
                let req = String::from_utf8_lossy(&buf);
                let first_line = req.lines().next().unwrap_or_default();
                let parts: Vec<&str> = first_line.split_whitespace().collect();
                let method = parts.first().copied().unwrap_or("GET");
                let path = parts.get(1).copied().unwrap_or("/");
                let (status, body) = match (method, path) {
                    ("GET", "/health") => ("200 OK", r#"{"status":"ok"}"#),
                    ("POST", "/validate") => ("200 OK", r#"{"valid":true}"#),
                    ("POST", "/events") => ("200 OK", r#"{"accepted":1,"rejected":0,"invalid":0}"#),
                    ("POST", "/query") => ("200 OK", r#"{"rows":[]}"#),
                    ("GET", p) if p.starts_with("/tail") => ("200 OK", r#"{"events":[]}"#),
                    ("DELETE", p) if p.starts_with("/events/") => ("200 OK", r#"{"deleted":1}"#),
                    ("POST", "/replay") => ("202 Accepted", r#"{"replayed":1}"#),
                    ("GET", p) if p.starts_with("/dlq?") => ("200 OK", r#"{"events":[]}"#),
                    ("GET", p) if p.starts_with("/dlq/") => ("200 OK", r#"{"entry":{}}"#),
                    ("POST", "/dlq/replay") => ("200 OK", r#"{"replayed":1}"#),
                    ("POST", "/keys") => ("201 Created", r#"{"id":"k_1"}"#),
                    ("DELETE", p) if p.starts_with("/keys/") => ("200 OK", r#"{"revoked":true}"#),
                    ("POST", p) if p.starts_with("/keys/") && p.ends_with("/rotate") => {
                        ("200 OK", r#"{"rotated":true}"#)
                    }
                    ("GET", "/sinks") => ("200 OK", r#"{"sinks":[]}"#),
                    ("POST", p) if p.starts_with("/sinks/") && p.ends_with("/test") => {
                        ("200 OK", r#"{"status":"healthy"}"#)
                    }
                    ("POST", "/policy/validate") => ("200 OK", r#"{"valid":true,"errors":[]}"#),
                    ("POST", "/schema/check") => ("200 OK", r#"{"valid":true}"#),
                    ("POST", "/schema/publish") => ("200 OK", r#"{"published":true}"#),
                    ("POST", "/retention/apply") => ("200 OK", r#"{"applied":true}"#),
                    _ => ("404 Not Found", r#"{"error":"not_found"}"#),
                };
                let response = format!(
                    "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                    body.len(),
                    body
                );
                let _ = stream.write_all(response.as_bytes());
                let _ = stream.flush();
            }
            Err(err) if err.kind() == std::io::ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(10));
            }
            Err(_) => break,
        }
    });
    (format!("http://{}", addr), tx)
}

#[test]
fn collector_and_cortex_client_families_work() {
    let (base, stop) = start_server();
    let collector = CollectorHttpClient::new(format!("{}/events", base))
        .with_api_key("secret")
        .with_timeout_ms(500)
        .with_service("catalog");

    let encoded = vec![
        serde_json::to_string(&serde_json::json!({"service":"catalog","event":"checkout.request"}))
            .unwrap(),
    ];
    let envelope = collector.envelope(&encoded);
    collector.validate_envelope(&envelope).expect("envelope");
    assert!(collector.tail_endpoint().ends_with("/tail"));
    assert_eq!(collector.validate(&encoded).unwrap().status_code, 200);
    assert_eq!(collector.ingest(&encoded).unwrap().status_code, 200);
    assert_eq!(collector.query("select 1").unwrap().status_code, 200);
    assert_eq!(collector.tail(10).unwrap().status_code, 200);
    assert_eq!(collector.delete("evt_1").unwrap().status_code, 200);
    assert_eq!(
        collector.replay(&["evt_1".to_string()]).unwrap().status_code,
        202
    );
    assert_eq!(collector.dlq_list(10).unwrap().status_code, 200);
    assert_eq!(collector.dlq_read("dlq_1").unwrap().status_code, 200);
    assert_eq!(
        collector
            .dlq_replay(&["dlq_1".to_string()])
            .unwrap()
            .status_code,
        200
    );
    assert_eq!(collector.keys_create("catalog").unwrap().status_code, 201);
    assert_eq!(collector.keys_revoke("key_1").unwrap().status_code, 200);
    assert_eq!(collector.keys_rotate("key_1").unwrap().status_code, 200);
    assert_eq!(collector.sinks_list().unwrap().status_code, 200);
    assert_eq!(collector.sinks_test("stdout").unwrap().status_code, 200);
    assert_eq!(
        collector
            .policy_validate(&serde_json::json!({"sample_rate": 1.0}))
            .unwrap()
            .status_code,
        200
    );
    assert_eq!(
        collector
            .schema_check(&serde_json::json!({"event":"checkout.request"}))
            .unwrap()
            .status_code,
        200
    );
    assert_eq!(
        collector
            .schema_publish(&serde_json::json!({"schema":"v1"}))
            .unwrap()
            .status_code,
        200
    );
    assert_eq!(
        collector
            .retention_apply(&serde_json::json!({"days": 7}))
            .unwrap()
            .status_code,
        200
    );
    assert_eq!(collector.health().unwrap().status_code, 200);
    let _ = stop.send(());

    let cortex = loxa::CortexClient::new("http://127.0.0.1:9")
        .with_timeout(Duration::from_millis(10));
    assert!(!cortex.health());
    assert!(!cortex.ready());
}

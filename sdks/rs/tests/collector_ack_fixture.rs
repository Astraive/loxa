use loxa::{Config, Logger, Params};
use serde::Deserialize;
use serde_json::Value;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant};

#[derive(Debug, Deserialize)]
struct AckFixture {
    http_status: u16,
    response: Value,
    expected: AckExpected,
}

#[derive(Debug, Deserialize)]
struct AckExpected {
    outcome: String,
    message_contains: Option<String>,
}

#[test]
fn collector_ack_behavior_fixtures() {
    for path in fixture_paths() {
        let fixture: AckFixture =
            serde_json::from_str(&fs::read_to_string(&path).expect("fixture")).expect("json");
        println!(
            "running fixture: {} => expected: {}",
            path.display(),
            fixture.expected.outcome
        );
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
        listener
            .set_nonblocking(true)
            .expect("configure nonblocking listener");
        let addr = listener.local_addr().expect("listener addr");
        let body = serde_json::to_string(&fixture.response).expect("response json");

        let handle = thread::spawn(move || {
            let started = Instant::now();
            let mut accepted = 0usize;
            while accepted < 3 && started.elapsed() < Duration::from_secs(5) {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        accepted += 1;
                        stream
                            .set_nonblocking(false)
                            .expect("configure blocking stream");
                        stream
                            .set_read_timeout(Some(Duration::from_secs(5)))
                            .expect("set timeout");
                        let _ = read_http_request(&mut stream);
                        write_json_response(&mut stream, fixture.http_status, &body);
                    }
                    Err(err) if err.kind() == std::io::ErrorKind::WouldBlock => {
                        if fixture.http_status != 429
                            && accepted > 0
                            && started.elapsed() > Duration::from_millis(200)
                        {
                            break;
                        }
                        thread::sleep(Duration::from_millis(10));
                    }
                    Err(err) => panic!("accept request: {err}"),
                }
            }
        });

        let endpoint = format!("http://{addr}/v1/events");
        println!("test endpoint: {endpoint}");
        let logger =
            Logger::new(Config::test("checkout").with_sink(loxa::HttpBatchSink(&endpoint)));
        let mut ctx = logger.start_event(Params::new("payment.completed").with_kind("cli"));
        logger.finish(&mut ctx, "success").expect("finish event");
        let result = logger.emit(&ctx);
        handle.join().expect("server thread");

        match fixture.expected.outcome.as_str() {
            "success" => {
                assert!(result.is_ok(), "expected success for {}", path.display());
            }
            "failure" => {
                let err = result.expect_err("expected failure");
                if let Some(snippet) = fixture.expected.message_contains.as_deref() {
                    assert!(
                        err.to_string()
                            .to_lowercase()
                            .contains(&snippet.to_lowercase()),
                        "expected error containing {snippet}, got {err}"
                    );
                }
            }
            other => panic!("unknown expected outcome {other}"),
        }
    }
}

fn fixture_paths() -> Vec<PathBuf> {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("loxa-spec")
        .join("examples")
        .join("golden")
        .join("collector-acks");
    let mut files: Vec<_> = fs::read_dir(root)
        .expect("fixture dir")
        .filter_map(|entry| entry.ok().map(|item| item.path()))
        .filter(|path| path.extension().is_some_and(|ext| ext == "json"))
        .collect();
    files.sort();
    files
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
        202 => "202 Accepted",
        207 => "207 Multi-Status",
        429 => "429 Too Many Requests",
        _ => "400 Bad Request",
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

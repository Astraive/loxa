use flate2::read::GzDecoder;
use loxa::{Config, Logger, Params, SinkConfig};
use std::io::{Read, Write};
use std::net::TcpListener;
use std::sync::mpsc;
use std::sync::{Mutex, OnceLock};
use std::thread;
use std::time::Duration;

#[test]
fn http_batch_sink_posts_gzipped_events_to_collector() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
    listener
        .set_nonblocking(false)
        .expect("configure blocking listener");
    let addr = listener.local_addr().expect("listener addr");
    let (tx, rx) = mpsc::channel();

    let handle = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept request");
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("set timeout");

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
            if !headers_done {
                if let Some(pos) = find_headers_end(&raw) {
                    headers_done = true;
                    let head = String::from_utf8_lossy(&raw[..pos]);
                    content_length = parse_content_length(&head);
                    let body_len = raw.len() - (pos + 4);
                    if body_len >= content_length {
                        break;
                    }
                }
            } else if let Some(pos) = find_headers_end(&raw) {
                let body_len = raw.len() - (pos + 4);
                if body_len >= content_length {
                    break;
                }
            }
        }

        let headers_end = find_headers_end(&raw).expect("headers end");
        let head = String::from_utf8_lossy(&raw[..headers_end]).to_string();
        let body = &raw[headers_end + 4..headers_end + 4 + content_length];

        let mut decoder = GzDecoder::new(body);
        let mut decoded = String::new();
        decoder
            .read_to_string(&mut decoded)
            .expect("decode gzip body");
        tx.send((head, decoded)).expect("send request capture");

        write_json_response(
            &mut stream,
            r#"{"accepted":1,"rejected":0,"invalid":0,"acks":[]}"#,
        );
        stream.flush().expect("flush response");
    });

    let endpoint = format!("http://{addr}/v1/events");
    let logger = Logger::new(Config::test("checkout").with_sink(loxa::HttpBatchSink(&endpoint)));
    let mut ctx = logger.start_event(Params::new("checkout.collector").with_kind("cli"));
    logger.enrich(&mut ctx, "tenant.id", "tenant-1");
    logger.finish(&mut ctx, "success").expect("finish event");
    logger.emit(&ctx).expect("emit via collector sink");

    let (head, decoded) = rx
        .recv_timeout(Duration::from_secs(5))
        .expect("captured request");
    handle.join().expect("server thread");

    assert!(head.starts_with("POST /v1/events HTTP/1.1"));
    assert!(head.contains("Content-Type: application/json"));
    assert!(head.contains("Content-Encoding: gzip"));
    assert!(decoded.contains("\"events\":["));
    assert!(decoded.contains("\"event\":\"checkout.collector\""));
    assert!(decoded.contains("\"service\":\"checkout\""));
}

#[test]
fn http_batch_sink_uses_api_key_from_environment() {
    let _guard = env_lock().lock().expect("env lock");
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
    let addr = listener.local_addr().expect("listener addr");
    let (tx, rx) = mpsc::channel();

    let handle = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept request");
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("set timeout");
        let raw = read_http_request(&mut stream);
        let headers_end = find_headers_end(&raw).expect("headers end");
        let head = String::from_utf8_lossy(&raw[..headers_end]).to_string();
        tx.send(head).expect("send head");
        write_json_response(
            &mut stream,
            r#"{"accepted":1,"rejected":0,"invalid":0,"acks":[]}"#,
        );
    });

    std::env::set_var("LOXA_COLLECTOR_API_KEY", "secret-key");
    std::env::set_var("LOXA_COLLECTOR_API_KEY_HEADER", "X-API-Key");

    let endpoint = format!("http://{addr}/v1/events");
    let logger = Logger::new(Config::test("checkout").with_sink(loxa::HttpBatchSink(&endpoint)));
    let mut ctx = logger.start_event(Params::new("checkout.collector").with_kind("cli"));
    logger.finish(&mut ctx, "success").expect("finish event");
    logger.emit(&ctx).expect("emit via collector sink");

    let head = rx
        .recv_timeout(Duration::from_secs(5))
        .expect("captured request");
    handle.join().expect("server thread");

    std::env::remove_var("LOXA_COLLECTOR_API_KEY");
    std::env::remove_var("LOXA_COLLECTOR_API_KEY_HEADER");

    assert!(head.contains("X-API-Key: secret-key"));
}

fn find_headers_end(raw: &[u8]) -> Option<usize> {
    raw.windows(4).position(|window| window == b"\r\n\r\n")
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

fn write_json_response(stream: &mut std::net::TcpStream, body: &str) {
    let response = format!(
        "HTTP/1.1 202 Accepted\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
        body.len(),
        body
    );
    stream
        .write_all(response.as_bytes())
        .expect("write collector response");
}

fn env_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}

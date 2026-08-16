use loza::{CollectorHttpClient, QueryValue};
use std::io::{Read, Write};
use std::net::TcpListener;
use std::thread;

#[test]
fn query_lql_uses_scoped_route_and_bearer() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let server = thread::spawn(move || {
        let (mut stream, _) = listener.accept().unwrap();
        let mut request_bytes = Vec::new();
        let mut byte = [0_u8; 1];
        while !request_bytes.ends_with(b"\r\n\r\n") {
            stream.read_exact(&mut byte).unwrap();
            request_bytes.push(byte[0]);
        }
        let request = String::from_utf8_lossy(&request_bytes).to_ascii_lowercase();
        assert!(request.contains("post /collectors/demo/lql/query"));
        let mut body_buffer = [0_u8; 2048];
        let _ = stream.read(&mut body_buffer).unwrap();
        assert!(request.contains("authorization: bearer api"));
        let body = r#"{"columns":["event_id"],"rows":[{"event_id":"evt-1"}],"row_count":1}"#;
        write!(stream, "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
    });
    let client = CollectorHttpClient::new(format!("http://{address}"))
        .with_collector("demo")
        .with_api_key("api")
        .with_insecure(true);
    let result = client
        .query_lql(
            "from events",
            [("id".into(), QueryValue::new("string", "evt-1"))]
                .into_iter()
                .collect(),
            10,
        )
        .unwrap();
    assert_eq!(result.row_count, 1);
    server.join().unwrap();
}

use loxa::{middleware::tower::capture_request, Config, New};

pub fn capture_http_once() -> String {
    let logger = New(Config::test("bench"));
    capture_request(&logger, "GET", "/health", 200)
        .map(|result| result.encoded)
        .unwrap_or_default()
}


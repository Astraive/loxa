use loxa::{middleware::tower::capture_request, Config, Logger};

pub fn capture_http_once() -> String {
    let logger = Logger::new(Config::test("bench"));
    capture_request(&logger, "GET", "/health", 200)
        .map(|result| result.encoded)
        .unwrap_or_default()
}


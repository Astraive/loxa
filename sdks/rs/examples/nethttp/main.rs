use loxa::{middleware::tower::capture_request, Config, Logger};

fn main() {
    let logger = Logger::new(Config::test("web"));
    let encoded = capture_request(&logger, "GET", "/ready", 200)
        .map(|result| result.encoded)
        .unwrap_or_default();
    println!("{encoded}");
}

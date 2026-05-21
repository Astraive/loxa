#[test]
fn middleware_module_captures_http_request() {
    let logger = loxa::Logger::new(loxa::Config::test("web"));
    let result = loxa::middleware::tower::capture_request(&logger, "GET", "/health", 200).unwrap();
    assert!(result.encoded.contains("\"kind\":\"http\""));
}

#[test]
fn middleware_modules_are_wired_for_supported_frameworks() {
    assert_eq!(loxa::middleware::actix::middleware_name(), "loxa-actix");
    assert_eq!(loxa::middleware::axum::middleware_name(), "loxa-axum");
    assert_eq!(loxa::middleware::hyper::middleware_name(), "loxa-hyper");
}

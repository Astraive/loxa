#[test]
fn middleware_module_captures_http_request() {
    let logger = loza::Logger::new(loza::Config::test("web"));
    let result = loza::middleware::tower::capture_request(&logger, "GET", "/health", 200).unwrap();
    assert!(result.encoded.contains("\"kind\":\"http\""));
}

#[test]
fn middleware_modules_are_wired_for_supported_frameworks() {
    assert_eq!(loza::middleware::actix::middleware_name(), "loza-actix");
    assert_eq!(loza::middleware::axum::middleware_name(), "loza-axum");
    assert_eq!(loza::middleware::hyper::middleware_name(), "loza-hyper");
}

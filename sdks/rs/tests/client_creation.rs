#[test]
fn client_creation_and_alias_helpers_work() {
    let logger = loza::create_loza(loza::Config::test("catalog"));
    assert_eq!(logger.config().service, "catalog");

    let _ = loza::configure(loza::Config::test("default"));
    let aliased = loza::alias("audit");
    assert_eq!(aliased.config().service, "default");
    assert_eq!(aliased.config().alias, "audit");
}

#[test]
fn direct_basic_auth_rejects_remote_plaintext_http() {
    let client = loza::core::CollectorHttpClient::new("http://collector.example.com")
        .with_basic_auth("key-id", "secret");
    let error = client
        .ingest(&[])
        .expect_err("remote Basic auth must not use plaintext HTTP");
    assert!(
        error.contains("credentialed HTTP requires TLS"),
        "unexpected error: {error}"
    );
}

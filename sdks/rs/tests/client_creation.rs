#[test]
fn client_creation_and_alias_helpers_work() {
    let logger = loza::create_loza(loza::Config::test("catalog"));
    assert_eq!(logger.config().service, "catalog");

    let _ = loza::configure(loza::Config::test("default"));
    let aliased = loza::alias("audit");
    assert_eq!(aliased.config().service, "default");
    assert_eq!(aliased.config().alias, "audit");
}

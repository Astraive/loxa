#[test]
fn client_creation_and_alias_helpers_work() {
    let logger = loxa::create_loxa(loxa::Config::test("catalog"));
    assert_eq!(logger.config().service, "catalog");

    let _ = loxa::configure(loxa::Config::test("default"));
    let aliased = loxa::alias("audit");
    assert_eq!(aliased.config().service, "default");
    assert_eq!(aliased.config().alias, "audit");
}

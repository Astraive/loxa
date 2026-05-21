#[test]
fn root_lifecycle_exports_emit() {
    let logger = loxa::Logger::new(loxa::Config::test("root"));
    let mut ctx = logger.start_event(loxa::Params::new("root.run"));
    logger.finish(&mut ctx, "success");
    assert!(logger.emit(&ctx).unwrap().contains("\"schema_version\":\"v1\""));
}


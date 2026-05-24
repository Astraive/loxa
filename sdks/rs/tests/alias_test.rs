#[test]
fn alias_creates_logger_with_alias_metadata() {
    let logger = loxa::New(loxa::Config::dev("api"));
    let aliased = logger.alias("audit");
    assert_eq!(aliased.config().service, "api");
    assert_eq!(aliased.config().alias, "audit");
    assert_eq!(logger.config().service, "api");
    assert_eq!(logger.config().alias, "");
}

#[test]
fn create_loxa_returns_logger() {
    let logger = loxa::create_loxa(loxa::Config::dev("test"));
    assert_eq!(logger.config().service, "test");
}

#[test]
fn module_level_alias_works() {
    let _ = loxa::configure(loxa::Config::dev("default"));
    let aliased = loxa::alias("other");
    assert_eq!(aliased.config().service, "default");
    assert_eq!(aliased.config().alias, "other");
}

#[test]
fn uppercase_aliases_work() {
    let logger = loxa::CreateLoxa(loxa::Config::dev("test"));
    assert_eq!(logger.config().service, "test");
}

#[test]
fn alias_emits_metadata_without_changing_service() {
    let events = loxa::testkit::capture(|logger| {
        let aliased = logger.alias("audit");
        let _ = aliased.info("permission changed");
    });
    let payload: serde_json::Value = serde_json::from_str(&events[0]).expect("event json");
    assert_eq!(payload["service"], "capture");
    assert_eq!(payload["attrs"]["loxa"]["alias"], "audit");
}

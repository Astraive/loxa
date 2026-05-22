use loxa;

#[test]
fn alias_creates_logger_with_different_service() {
    let logger = loxa::Logger::new(loxa::Config::dev("api"));
    let aliased = logger.alias("audit");
    assert_eq!(aliased.config().service, "audit");
    assert_eq!(logger.config().service, "api");
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
    assert_eq!(aliased.config().service, "other");
}

#[test]
fn uppercase_aliases_work() {
    let logger = loxa::CreateLoxa(loxa::Config::dev("test"));
    assert_eq!(logger.config().service, "test");
}

#[test]
fn basic_logging_and_event_facades_work() {
    let store = loza::MemorySinkStore::new();
    let logger = loza::configure(
        loza::Config::dev("capture").with_sink(loza::SinkConfig::Memory(store.clone())),
    )
    .expect("configure global logger");

    loza::notice("notice event");
    loza::track("checkout.page_view", &[loza::string("page", "/checkout")]);
    let mut audit = loza::audit("user.login");
    let _ = logger.finish(&mut audit, "success");
    let _ = logger.emit(&audit);
    let mut security = loza::security("auth.failure");
    let _ = logger.finish(&mut security, "success");
    let _ = logger.emit(&security);
    let mut metric = loza::metric("latency");
    logger.append(&mut metric, loza::string("unit", "ms"));
    let _ = logger.finish(&mut metric, "success");
    let _ = logger.emit(&metric);
    loza::count("requests", 4);
    loza::gauge("cpu", 0.72);
    loza::histogram("payload.bytes", 512.0);
    loza::breadcrumb("nav.click");
    loza::flush();

    let events = store.events();
    assert_eq!(events.len(), 9);
    assert!(events
        .iter()
        .any(|event| event.contains("\"message\":\"notice event\"")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"event\":\"checkout.page_view\"")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"event\":\"user.login\"")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"event\":\"auth.failure\"")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"event\":\"latency\"")));
    assert!(events.iter().any(|event| event.contains("\"count\":4")));
    assert!(events.iter().any(|event| event.contains("\"gauge\":0.72")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"histogram\":512.0")));
    assert!(events
        .iter()
        .any(|event| event.contains("\"message\":\"nav.click\"")));
}

#[test]
fn basic_logging_and_event_facades_work() {
    let store = loxa::MemorySinkStore::new();
    let logger = loxa::configure(
        loxa::Config::dev("capture").with_sink(loxa::SinkConfig::Memory(store.clone())),
    )
    .expect("configure global logger");

    loxa::notice("notice event");
    loxa::track("checkout.page_view", &[loxa::string("page", "/checkout")]);
    let mut audit = loxa::audit("user.login");
    let _ = logger.finish(&mut audit, "success");
    let _ = logger.emit(&audit);
    let mut security = loxa::security("auth.failure");
    let _ = logger.finish(&mut security, "success");
    let _ = logger.emit(&security);
    let mut metric = loxa::metric("latency");
    logger.append(&mut metric, loxa::string("unit", "ms"));
    let _ = logger.finish(&mut metric, "success");
    let _ = logger.emit(&metric);
    loxa::count("requests", 4);
    loxa::gauge("cpu", 0.72);
    loxa::histogram("payload.bytes", 512.0);
    loxa::breadcrumb("nav.click");
    loxa::flush();

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

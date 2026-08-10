#[test]
fn testing_and_conformance_helpers_work() {
    let events = loza::testkit::capture(|logger| {
        let mut event = logger.start_event(loza::Params::new("capture.event"));
        logger.append(&mut event, loza::string("family", "testkit"));
        logger.checkpoint(&mut event, "captured");
        let _ = logger.finish(&mut event, "success");
        let _ = logger.emit(&event);
    });
    assert_eq!(events.len(), 1);
    loza::testkit::assert_event(&events[0], "attrs.family", "testkit");
    loza::testkit::assert_has_checkpoint(&events[0], "captured");
    let _ = loza::testkit::mock_sink();
    loza::testkit::fake_clock(0);
    loza::testkit::set_id_generator(|| "evt_fixed".to_string());
}

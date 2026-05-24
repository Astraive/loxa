#[test]
fn testing_and_conformance_helpers_work() {
    let events = loxa::testkit::capture(|logger| {
        let mut event = logger.start_event(loxa::Params::new("capture.event"));
        logger.append(&mut event, loxa::string("family", "testkit"));
        logger.checkpoint(&mut event, "captured");
        let _ = logger.finish(&mut event, "success");
        let _ = logger.emit(&event);
    });
    assert_eq!(events.len(), 1);
    loxa::testkit::assert_event(&events[0], "attrs.family", "testkit");
    loxa::testkit::assert_has_checkpoint(&events[0], "captured");
    let _ = loxa::testkit::mock_sink();
    loxa::testkit::fake_clock(0);
    loxa::testkit::set_id_generator(|| "evt_fixed".to_string());
}

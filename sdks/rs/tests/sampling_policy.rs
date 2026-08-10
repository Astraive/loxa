#[test]
fn sampling_and_policy_helpers_work() {
    let mut ctx = loza::start_event(None, loza::Params::new("test"));
    loza::finish(&mut ctx);
    assert!(loza::should_sample(&ctx, &loza::sample_all()));
    assert!(!loza::should_sample(&ctx, &loza::sample_none()));
    let _ = loza::sample_by_event(|event| event.event == "test");
    let _ = loza::sample_by_outcome(&["success"]);
    let _ = loza::allow_fields(&["safe"]);
    let _ = loza::block_fields(&["danger"]);
}

#[test]
fn sampling_and_policy_helpers_work() {
    let mut ctx = loxa::start_event(None, loxa::Params::new("test"));
    loxa::finish(&mut ctx);
    assert!(loxa::should_sample(&ctx, &loxa::sample_all()));
    assert!(!loxa::should_sample(&ctx, &loxa::sample_none()));
    let _ = loxa::sample_by_event(|event| event.event == "test");
    let _ = loxa::sample_by_outcome(&["success"]);
    let _ = loxa::allow_fields(&["safe"]);
    let _ = loxa::block_fields(&["danger"]);
}

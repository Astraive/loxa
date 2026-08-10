use serde_json::Map;

#[test]
fn lifecycle_event_helpers_work() {
    let mut ctx = loza::start_event(
        None,
        loza::Params::new("checkout.request").with_kind("http"),
    );
    loza::append(&mut ctx, loza::user_id("u_123"));
    loza::enrich(&mut ctx, vec![loza::tenant_id("t_123")]);
    loza::set(&mut ctx, "payment.provider", "stripe");

    let mut merged = Map::new();
    merged.insert("cart.items".to_string(), serde_json::json!(3));
    loza::merge(&mut ctx, merged);

    assert_eq!(
        loza::get_group(&ctx, "payment")
            .and_then(|v| v.get("provider"))
            .and_then(|v| v.as_str()),
        Some("stripe")
    );
    assert_eq!(
        loza::get_group(&ctx, "cart")
            .and_then(|v| v.get("items"))
            .and_then(|v| v.as_i64()),
        Some(3)
    );

    loza::delete(&mut ctx, "payment.provider");
    loza::checkpoint(&mut ctx, "validated");
    loza::checkpoint_with_attrs(
        &mut ctx,
        "payment_started",
        &[loza::string("stage", "payment")],
    );

    let cloned = loza::clone_event(&ctx);
    assert_eq!(cloned.event_id, ctx.event_id);
    loza::link_event(&mut ctx, "evt_parent");

    loza::finish(&mut ctx);
    loza::emit(&mut ctx).expect("emit");
    assert_eq!(ctx.outcome, Some("success".to_string()));

    let mut dropped = loza::start_event(None, loza::Params::new("drop.event"));
    loza::drop(&mut dropped, "capacity");
    let mut cancelled = loza::start_event(None, loza::Params::new("cancel.event"));
    loza::cancel(&mut cancelled);
    let mut abandoned = loza::start_event(None, loza::Params::new("abandon.event"));
    loza::abandon(&mut abandoned);
    let mut retried = loza::start_event(None, loza::Params::new("retry.event"));
    loza::retry(&mut retried);
    let mut partial = loza::start_event(None, loza::Params::new("partial.event"));
    loza::partial(&mut partial, "timeout");

    assert!(loza::current_event().is_none());
}

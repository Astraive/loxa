use serde_json::Map;

#[test]
fn lifecycle_event_helpers_work() {
    let mut ctx = loxa::start_event(None, loxa::Params::new("checkout.request").with_kind("http"));
    loxa::append(&mut ctx, loxa::user_id("u_123"));
    loxa::enrich(&mut ctx, vec![loxa::tenant_id("t_123")]);
    loxa::set(&mut ctx, "payment.provider", "stripe");

    let mut merged = Map::new();
    merged.insert("cart.items".to_string(), serde_json::json!(3));
    loxa::merge(&mut ctx, merged);

    assert_eq!(
        loxa::get_group(&ctx, "payment")
            .and_then(|v| v.get("provider"))
            .and_then(|v| v.as_str()),
        Some("stripe")
    );
    assert_eq!(
        loxa::get_group(&ctx, "cart")
            .and_then(|v| v.get("items"))
            .and_then(|v| v.as_i64()),
        Some(3)
    );

    loxa::delete(&mut ctx, "payment.provider");
    loxa::checkpoint(&mut ctx, "validated");
    loxa::checkpoint_with_attrs(&mut ctx, "payment_started", &[loxa::string("stage", "payment")]);

    let cloned = loxa::clone_event(&ctx);
    assert_eq!(cloned.event_id, ctx.event_id);
    loxa::link_event(&mut ctx, "evt_parent");

    loxa::finish(&mut ctx);
    loxa::emit(&mut ctx).expect("emit");
    assert_eq!(ctx.outcome, Some("success".to_string()));

    let mut dropped = loxa::start_event(None, loxa::Params::new("drop.event"));
    loxa::drop(&mut dropped, "capacity");
    let mut cancelled = loxa::start_event(None, loxa::Params::new("cancel.event"));
    loxa::cancel(&mut cancelled);
    let mut abandoned = loxa::start_event(None, loxa::Params::new("abandon.event"));
    loxa::abandon(&mut abandoned);
    let mut retried = loxa::start_event(None, loxa::Params::new("retry.event"));
    loxa::retry(&mut retried);
    let mut partial = loxa::start_event(None, loxa::Params::new("partial.event"));
    loxa::partial(&mut partial, "timeout");

    assert!(loxa::current_event().is_none());
}

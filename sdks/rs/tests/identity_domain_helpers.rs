#[test]
fn identity_domain_helpers_work() {
    assert_eq!(loxa::user_id("u_123").key, "user.id");
    assert_eq!(loxa::tenant_id("t_123").key, "tenant.id");
    assert_eq!(loxa::session_id("s_123").key, "session.id");
    assert_eq!(loxa::request_id("req_123").key, "request_id");
    assert_eq!(loxa::trace_id("trace_123").key, "trace_id");
    assert_eq!(loxa::span_id("span_123").key, "span_id");
    assert_eq!(loxa::order_id("ord_123").key, "order.id");
    assert_eq!(loxa::cart_id("cart_123").key, "cart.id");
    assert_eq!(loxa::payment_id("pay_123").key, "payment.id");
    assert_eq!(loxa::subscription_id("sub_123").key, "subscription.id");
    assert_eq!(loxa::invoice_id("inv_123").key, "invoice.id");
    assert_eq!(loxa::job_id("job_123").key, "job.id");
    assert_eq!(loxa::message_id("msg_123").key, "message.id");
    assert_eq!(loxa::correlation_id("corr_123").key, "correlation.id");
    assert_eq!(loxa::commit_sha("abc123").key, "commit.sha");
    assert_eq!(loxa::release("1.2.3").key, "release");
    assert_eq!(loxa::region("us-east-1").key, "region");
    assert_eq!(
        loxa::checkout_cart_item_count(3).key,
        "checkout.cart_item_count"
    );
    assert_eq!(loxa::payment_provider("stripe").key, "payment.provider");
    assert_eq!(loxa::billing_plan("pro").key, "billing.plan");
    assert_eq!(loxa::agent_model("gpt-5.5").key, "agent.model");
    assert_eq!(loxa::rag_index("docs").key, "rag.index");
}

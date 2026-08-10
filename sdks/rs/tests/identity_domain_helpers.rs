#[test]
fn identity_domain_helpers_work() {
    assert_eq!(loza::user_id("u_123").key, "user.id");
    assert_eq!(loza::tenant_id("t_123").key, "tenant.id");
    assert_eq!(loza::session_id("s_123").key, "session.id");
    assert_eq!(loza::request_id("req_123").key, "request_id");
    assert_eq!(loza::trace_id("trace_123").key, "trace_id");
    assert_eq!(loza::span_id("span_123").key, "span_id");
    assert_eq!(loza::order_id("ord_123").key, "order.id");
    assert_eq!(loza::cart_id("cart_123").key, "cart.id");
    assert_eq!(loza::payment_id("pay_123").key, "payment.id");
    assert_eq!(loza::subscription_id("sub_123").key, "subscription.id");
    assert_eq!(loza::invoice_id("inv_123").key, "invoice.id");
    assert_eq!(loza::job_id("job_123").key, "job.id");
    assert_eq!(loza::message_id("msg_123").key, "message.id");
    assert_eq!(loza::correlation_id("corr_123").key, "correlation.id");
    assert_eq!(loza::commit_sha("abc123").key, "commit.sha");
    assert_eq!(loza::release("1.2.3").key, "release");
    assert_eq!(loza::region("us-east-1").key, "region");
    assert_eq!(
        loza::checkout_cart_item_count(3).key,
        "checkout.cart_item_count"
    );
    assert_eq!(loza::payment_provider("stripe").key, "payment.provider");
    assert_eq!(loza::billing_plan("pro").key, "billing.plan");
    assert_eq!(loza::agent_model("gpt-5.5").key, "agent.model");
    assert_eq!(loza::rag_index("docs").key, "rag.index");
}

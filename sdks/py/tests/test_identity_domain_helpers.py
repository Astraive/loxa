from __future__ import annotations

import loza


def test_identity_domain_helpers_snake_case():
    assert loza.UserID("u_123").key == "user.id"
    assert loza.TenantID("t_123").key == "tenant.id"
    assert loza.SessionID("s_123").key == "session.id"
    assert loza.RequestID("req_123").key == "request_id"
    assert loza.TraceID("trace_123").key == "trace_id"
    assert loza.SpanID("span_123").key == "span_id"
    assert loza.OrderID("ord_123").key == "order.id"
    assert loza.CartID("cart_123").key == "cart.id"
    assert loza.payment_id("pay_123").key == "payment.id"
    assert loza.subscription_id("sub_123").key == "payment.subscription_id"
    assert loza.invoice_id("inv_123").key == "payment.invoice_id"
    assert loza.job_id("job_123").key == "job.id"
    assert loza.message_id("msg_123").key == "message.id"
    assert loza.correlation_id("cor_123").key == "correlation_id"
    assert loza.commit_sha("abc123").key == "commit.sha"
    assert loza.release("v1.0").key == "release"
    assert loza.region("us-east-1").key == "region"


def test_identity_domain_helpers_domain_packs():
    assert loza.checkout_cart_item_count(3).key == "checkout.cart_item_count"
    assert loza.checkout_cart_total(2999).key == "checkout.cart_total"
    assert loza.payment_provider("stripe").key == "payment.provider"
    assert loza.payment_method("credit_card").key == "payment.method"
    assert loza.billing_plan("pro").key == "billing.plan"
    assert loza.agent_model("gpt-4").key == "agent.model"
    assert loza.rag_index("docs").key == "rag.index"


def test_identity_domain_helpers_pascal_case():
    assert loza.PaymentID("pay_123").key == "payment.id"
    assert loza.SubscriptionID("sub_123").key == "payment.subscription_id"
    assert loza.InvoiceID("inv_123").key == "payment.invoice_id"
    assert loza.JobID("job_123").key == "job.id"
    assert loza.MessageID("msg_123").key == "message.id"
    assert loza.CorrelationID("cor_123").key == "correlation_id"
    assert loza.CommitSHA("abc123").key == "commit.sha"
    assert loza.Release("v1.0").key == "release"
    assert loza.Region("us-east-1").key == "region"

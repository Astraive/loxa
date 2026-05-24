from __future__ import annotations

import loxa


def test_identity_domain_helpers_snake_case():
    assert loxa.UserID("u_123").key == "user.id"
    assert loxa.TenantID("t_123").key == "tenant.id"
    assert loxa.SessionID("s_123").key == "session.id"
    assert loxa.RequestID("req_123").key == "request_id"
    assert loxa.TraceID("trace_123").key == "trace_id"
    assert loxa.SpanID("span_123").key == "span_id"
    assert loxa.OrderID("ord_123").key == "order.id"
    assert loxa.CartID("cart_123").key == "cart.id"
    assert loxa.payment_id("pay_123").key == "payment.id"
    assert loxa.subscription_id("sub_123").key == "payment.subscription_id"
    assert loxa.invoice_id("inv_123").key == "payment.invoice_id"
    assert loxa.job_id("job_123").key == "job.id"
    assert loxa.message_id("msg_123").key == "message.id"
    assert loxa.correlation_id("cor_123").key == "correlation_id"
    assert loxa.commit_sha("abc123").key == "commit.sha"
    assert loxa.release("v1.0").key == "release"
    assert loxa.region("us-east-1").key == "region"


def test_identity_domain_helpers_domain_packs():
    assert loxa.checkout_cart_item_count(3).key == "checkout.cart_item_count"
    assert loxa.checkout_cart_total(2999).key == "checkout.cart_total"
    assert loxa.payment_provider("stripe").key == "payment.provider"
    assert loxa.payment_method("credit_card").key == "payment.method"
    assert loxa.billing_plan("pro").key == "billing.plan"
    assert loxa.agent_model("gpt-4").key == "agent.model"
    assert loxa.rag_index("docs").key == "rag.index"


def test_identity_domain_helpers_pascal_case():
    assert loxa.PaymentID("pay_123").key == "payment.id"
    assert loxa.SubscriptionID("sub_123").key == "payment.subscription_id"
    assert loxa.InvoiceID("inv_123").key == "payment.invoice_id"
    assert loxa.JobID("job_123").key == "job.id"
    assert loxa.MessageID("msg_123").key == "message.id"
    assert loxa.CorrelationID("cor_123").key == "correlation_id"
    assert loxa.CommitSHA("abc123").key == "commit.sha"
    assert loxa.Release("v1.0").key == "release"
    assert loxa.Region("us-east-1").key == "region"

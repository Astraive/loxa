package conformance

import (
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestIdentityDomainHelpers(t *testing.T) {
	attrCases := []struct {
		name string
		attr loxa.Attr
		key  string
	}{
		{"user id", loxa.UserID("u_123"), "user.id"},
		{"tenant id", loxa.TenantID("t_123"), "tenant.id"},
		{"session id", loxa.SessionID("s_123"), "session.id"},
		{"request id", loxa.RequestID("req_123"), "request_id"},
		{"trace id", loxa.TraceID("trace_123"), "trace_id"},
		{"span id", loxa.SpanID("span_123"), "span_id"},
		{"order id", loxa.OrderID("ord_123"), "order.id"},
		{"cart id", loxa.CartID("cart_123"), "cart.id"},
		{"payment id", loxa.PaymentID("pay_123"), "payment.id"},
		{"subscription id", loxa.SubscriptionID("sub_123"), "subscription.id"},
		{"invoice id", loxa.InvoiceID("inv_123"), "invoice.id"},
		{"job id", loxa.JobID("job_123"), "job.id"},
		{"message id", loxa.MessageID("msg_123"), "message.id"},
		{"correlation id", loxa.CorrelationID("corr_123"), "correlation.id"},
		{"deployment id", loxa.DeploymentID("dep_123"), "deployment_id"},
		{"release", loxa.Release("1.2.3"), "release"},
		{"region", loxa.Region("us-east-1"), "region"},
		{"checkout cart item count", loxa.CheckoutCartItemCount(3), "checkout.cart_item_count"},
		{"payment method", loxa.PaymentMethod("card"), "payment.method"},
		{"billing plan", loxa.BillingPlan("pro"), "billing.plan"},
		{"agent model", loxa.AgentModel("gpt-5.5"), "agent.model"},
		{"rag index", loxa.RAGIndex("docs"), "rag.index"},
	}

	for _, tc := range attrCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.attr.Key != tc.key {
				t.Fatalf("expected key %q, got %q", tc.key, tc.attr.Key)
			}
		})
	}
}

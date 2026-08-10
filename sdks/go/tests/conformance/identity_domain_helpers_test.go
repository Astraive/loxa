package conformance

import (
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestIdentityDomainHelpers(t *testing.T) {
	attrCases := []struct {
		name string
		attr loza.Attr
		key  string
	}{
		{"user id", loza.UserID("u_123"), "user.id"},
		{"tenant id", loza.TenantID("t_123"), "tenant.id"},
		{"session id", loza.SessionID("s_123"), "session.id"},
		{"request id", loza.RequestID("req_123"), "request_id"},
		{"trace id", loza.TraceID("trace_123"), "trace_id"},
		{"span id", loza.SpanID("span_123"), "span_id"},
		{"order id", loza.OrderID("ord_123"), "order.id"},
		{"cart id", loza.CartID("cart_123"), "cart.id"},
		{"payment id", loza.PaymentID("pay_123"), "payment.id"},
		{"subscription id", loza.SubscriptionID("sub_123"), "subscription.id"},
		{"invoice id", loza.InvoiceID("inv_123"), "invoice.id"},
		{"job id", loza.JobID("job_123"), "job.id"},
		{"message id", loza.MessageID("msg_123"), "message.id"},
		{"correlation id", loza.CorrelationID("corr_123"), "correlation.id"},
		{"deployment id", loza.DeploymentID("dep_123"), "deployment_id"},
		{"release", loza.Release("1.2.3"), "release"},
		{"region", loza.Region("us-east-1"), "region"},
		{"checkout cart item count", loza.CheckoutCartItemCount(3), "checkout.cart_item_count"},
		{"payment method", loza.PaymentMethod("card"), "payment.method"},
		{"billing plan", loza.BillingPlan("pro"), "billing.plan"},
		{"agent model", loza.AgentModel("gpt-5.5"), "agent.model"},
		{"rag index", loza.RAGIndex("docs"), "rag.index"},
	}

	for _, tc := range attrCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.attr.Key != tc.key {
				t.Fatalf("expected key %q, got %q", tc.key, tc.attr.Key)
			}
		})
	}
}

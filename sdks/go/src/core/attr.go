package core

import (
	"fmt"
	"time"
)

// Kind identifies the type stored in an Attr's Value field.
// Using Kind avoids reflection on the hot encoding path.
type Kind uint8

const (
	KindString   Kind = iota
	KindInt           // stored as int
	KindInt64         // stored as int64
	KindUint64        // stored as uint64
	KindFloat64       // stored as float64
	KindBool          // stored as bool
	KindTime          // stored as time.Time
	KindDuration      // stored as time.Duration
	KindGroup         // stored as []Attr
	KindAny           // stored as any — slow path via encoding/json
	KindStringer      // stored as fmt.Stringer
	KindError         // stored as error
	KindNull          // stored as nil
)

// Attr is a typed key-value pair used to enrich canonical events.
// Canonical fields use Kind to skip reflection on the fast path.
type Attr struct {
	Key   string
	Kind  Kind
	Value any
}

// ── Primitive constructors ────────────────────────────────────────────────────

// String creates a string Attr.
func String(key, val string) Attr { return Attr{Key: key, Kind: KindString, Value: val} }

// Int creates an int Attr.
func Int(key string, val int) Attr { return Attr{Key: key, Kind: KindInt, Value: val} }

// Int64 creates an int64 Attr.
func Int64(key string, val int64) Attr { return Attr{Key: key, Kind: KindInt64, Value: val} }

// Uint64 creates a uint64 Attr.
func Uint64(key string, val uint64) Attr { return Attr{Key: key, Kind: KindUint64, Value: val} }

// Float64 creates a float64 Attr.
func Float64(key string, val float64) Attr { return Attr{Key: key, Kind: KindFloat64, Value: val} }

// Bool creates a bool Attr.
func Bool(key string, val bool) Attr { return Attr{Key: key, Kind: KindBool, Value: val} }

// Time creates a time.Time Attr (encoded as RFC3339Nano).
func Time(key string, val time.Time) Attr { return Attr{Key: key, Kind: KindTime, Value: val} }

// Duration creates a time.Duration Attr (encoded as float64 milliseconds).
func Duration(key string, val time.Duration) Attr {
	return Attr{Key: key, Kind: KindDuration, Value: val}
}

// Any creates an Attr whose value is encoded via encoding/json (slow path).
func Any(key string, val any) Attr { return Attr{Key: key, Kind: KindAny, Value: val} }

// Null creates an Attr with a null JSON value.
func Null(key string) Attr { return Attr{Key: key, Kind: KindNull, Value: nil} }

// Err creates an error Attr under the key "error". If err is nil the Attr is zero-valued.
func Err(err error) Attr {
	if err == nil {
		return Attr{}
	}
	return Attr{Key: "error", Kind: KindError, Value: err}
}

// Stringer creates an Attr from any value implementing fmt.Stringer.
func Stringer(key string, val fmt.Stringer) Attr {
	return Attr{Key: key, Kind: KindStringer, Value: val}
}

// Group creates a nested object Attr.
//
//	loza.Group("user", loza.String("id", uid), loza.String("plan", "pro"))
//	→ {"user":{"id":"...","plan":"pro"}}
func Group(key string, attrs ...Attr) Attr {
	return Attr{Key: key, Kind: KindGroup, Value: attrs}
}

// ── Canonical helper constructors ─────────────────────────────────────────────

// RequestID sets the canonical request_id field.
func RequestID(id string) Attr { return String("request_id", id) }

// TraceID sets the canonical trace_id field.
func TraceID(id string) Attr { return String("trace_id", id) }

// SpanID sets the canonical span_id field.
func SpanID(id string) Attr { return String("span_id", id) }

// Service sets the canonical service field.
func Service(service string) Attr { return String("service", service) }

// Version sets the canonical version field.
func Version(version string) Attr { return String("version", version) }

// DeploymentID sets the canonical deployment_id field.
func DeploymentID(id string) Attr { return String("deployment_id", id) }

// Region sets the canonical region field.
func Region(region string) Attr { return String("region", region) }

// Method sets the canonical method field.
func Method(method string) Attr { return String("method", method) }

// Path sets the canonical path field.
func Path(path string) Attr { return String("path", path) }

// Route sets the canonical route field.
func Route(route string) Attr { return String("route", route) }

// IncidentID sets the canonical incident_id field.
func IncidentID(id string) Attr { return String("incident_id", id) }

// StatusCode sets the canonical status_code field.
func StatusCode(code int) Attr { return Int("status_code", code) }

// DurationMS sets the canonical duration_ms field.
func DurationMS(ms int64) Attr { return Int64("duration_ms", ms) }

// Outcome sets the canonical outcome field.
func Outcome(outcome string) Attr { return String("outcome", outcome) }

// ── Domain helper constructors ────────────────────────────────────────────────

// UserID sets user.id as a dot-key attr (expanded to {"user":{"id":...}}).
func UserID(id string) Attr { return String("user.id", id) }

// TenantID sets tenant.id.
func TenantID(id string) Attr { return String("tenant.id", id) }

// WorkspaceID sets workspace.id.
func WorkspaceID(id string) Attr { return String("workspace.id", id) }

// OrganizationID sets organization.id.
func OrganizationID(id string) Attr { return String("organization.id", id) }

// SessionID sets session.id.
func SessionID(id string) Attr { return String("session.id", id) }

// UserSubscription sets user.subscription.
func UserSubscription(sub string) Attr { return String("user.subscription", sub) }

// CartID sets cart.id.
func CartID(id string) Attr { return String("cart.id", id) }

// CartTotalCents sets cart.total_cents.
func CartTotalCents(total int64) Attr { return Int64("cart.total_cents", total) }

// PaymentProvider sets payment.provider.
func PaymentProvider(provider string) Attr { return String("payment.provider", provider) }

// PaymentLatencyMS sets payment.latency_ms.
func PaymentLatencyMS(ms int64) Attr { return Int64("payment.latency_ms", ms) }

// FeatureFlag adds a feature flag entry under the feature group.
func FeatureFlag(name string, value any) Attr {
	switch v := value.(type) {
	case bool:
		return Group("feature", Bool(name, v))
	case string:
		return Group("feature", String(name, v))
	default:
		return Group("feature", Any(name, v))
	}
}

// FeatureFlagBool adds a boolean feature flag.
func FeatureFlagBool(name string, enabled bool) Attr {
	return FeatureFlag(name, enabled)
}

// Experiment adds an experiment variant field.
func Experiment(name, variant string) Attr {
	return Group("experiment", String(name, variant))
}

// OrderID sets order.id.
func OrderID(id string) Attr { return String("order.id", id) }

// ProductID sets product.id.
func ProductID(id string) Attr { return String("product.id", id) }

// CustomerID sets customer.id.
func CustomerID(id string) Attr { return String("customer.id", id) }

// Plan sets customer.plan.
func Plan(name string) Attr { return String("customer.plan", name) }

// Currency sets payment.currency.
func Currency(code string) Attr { return String("payment.currency", code) }

// Amount sets payment.amount.
func Amount(value int64) Attr { return Int64("payment.amount", value) }

// Country sets geo.country.
func Country(code string) Attr { return String("geo.country", code) }

// Device sets device.name.
func Device(value string) Attr { return String("device.name", value) }

// Platform sets device.platform.
func Platform(value string) Attr { return String("device.platform", value) }

// AppVersion sets app.version.
func AppVersion(value string) Attr { return String("app.version", value) }

// JobName sets job.name.
func JobName(name string) Attr { return String("job.name", name) }

// QueueName sets queue.name.
func QueueName(name string) Attr { return String("queue.name", name) }

// MessageID sets message.id.
func MessageID(id string) Attr { return String("message.id", id) }

// Attempt sets retry.attempt.
func Attempt(n int) Attr { return Int("retry.attempt", n) }

// ErrorType sets error.type.
func ErrorType(name string) Attr { return String("error.type", name) }

// ErrorCode sets error_code.
func ErrorCode(code string) Attr { return String("error_code", code) }

// ErrorMessage sets error.message.
func ErrorMessage(msg string) Attr { return String("error.message", msg) }

// ErrorStack sets error.stack.
func ErrorStack(stack string) Attr { return String("error.stack", stack) }

// Retryable sets error.retryable.
func Retryable(v bool) Attr { return Bool("error.retryable", v) }

// MarkSensitive marks attr key as sensitive metadata.
func MarkSensitive(attr Attr) Attr {
	if attr.Key == "" {
		return attr
	}
	attr.Key = "sensitive." + attr.Key
	return attr
}

// SensitiveString marks a string field as sensitive.
func SensitiveString(key, value string) Attr {
	return MarkSensitive(String(key, value))
}

// HashString stores a hash-ready marker field for sensitive values.
func HashString(key, value string) Attr {
	return String("hash."+key, value)
}

// ── Additional domain helpers ────────────────────────────────────────────────

// PaymentID sets payment.id.
func PaymentID(id string) Attr { return String("payment.id", id) }

// SubscriptionID sets subscription.id.
func SubscriptionID(id string) Attr { return String("subscription.id", id) }

// InvoiceID sets invoice.id.
func InvoiceID(id string) Attr { return String("invoice.id", id) }

// JobID sets job.id.
func JobID(id string) Attr { return String("job.id", id) }

// CorrelationID sets correlation.id.
func CorrelationID(id string) Attr { return String("correlation.id", id) }

// CommitSha sets commit.sha.
func CommitSha(sha string) Attr { return String("commit.sha", sha) }

// Release sets release.
func Release(version string) Attr { return String("release", version) }

// Money creates a money attribute with amount in cents and currency.
func Money(key string, amountCents int64, currency string) Attr {
	return Group(key, Int64("amount_cents", amountCents), String("currency", currency))
}

// Percent creates a percentage attribute.
func Percent(key string, val float64) Attr { return Float64(key, val) }

// Bytes creates a bytes attribute (int64 byte count).
func Bytes(key string, val int64) Attr { return Int64(key, val) }

// HTTPStatus sets http.status (alias for StatusCode).
func HTTPStatus(code int) Attr { return StatusCode(code) }

// Bucket creates a bucket/tag grouping attribute.
func Bucket(key string, vals ...string) Attr {
	items := make([]Attr, len(vals))
	for i, v := range vals {
		items[i] = String(key, v)
	}
	return Group("bucket", items...)
}

// Tags creates a comma-separated tags attribute.
func Tags(key string, vals ...string) Attr {
	if len(vals) == 0 {
		return String(key, "")
	}
	return String(key, joinStrings(vals, ","))
}

// Masked creates a masked value attribute.
func Masked(key, value string) Attr { return String(key, value) }

// URL creates a URL attribute.
func URL(key, value string) Attr { return String(key, value) }

// EmailHash creates a hashed email attribute.
func EmailHash(key, value string) Attr { return String(key, value) }

// IPHash creates a hashed IP attribute.
func IPHash(key, value string) Attr { return String(key, value) }

// List creates a list/array attribute.
func List(key string, values ...any) Attr { return Any(key, values) }

// Map creates a map/object attribute.
func Map(key string, value map[string]any) Attr { return Any(key, value) }

// Enum creates an enum attribute with optional allowed values.
func Enum(key, value string, allowed ...string) Attr { return String(key, value) }

// ID creates a high-cardinality ID attribute.
func ID(key, value string) Attr { return String(key, value) }

// Hash creates a hashed attribute.
func Hash(key, value string) Attr { return HashString(key, value) }

// Redacted creates an explicit redacted marker attribute.
func Redacted(key string) Attr { return String(key, "[REDACTED]") }

// AccountID creates a canonical account.id attribute.
func AccountID(id string) Attr { return String("account.id", id) }

// ── Checkout domain helpers ──────────────────────────────────────────────────

// CheckoutCartItemCount sets checkout.cart_item_count.
func CheckoutCartItemCount(count int) Attr { return Int("checkout.cart_item_count", count) }

// CheckoutCartTotal sets checkout.cart_total.
func CheckoutCartTotal(total int64) Attr { return Int64("checkout.cart_total", total) }

// CheckoutPaymentMethod sets checkout.payment_method.
func CheckoutPaymentMethod(method string) Attr { return String("checkout.payment_method", method) }

// CheckoutStatus sets checkout.status.
func CheckoutStatus(status string) Attr { return String("checkout.status", status) }

// ── Payment domain helpers ───────────────────────────────────────────────────

// PaymentMethod sets payment.method.
func PaymentMethod(method string) Attr { return String("payment.method", method) }

// PaymentIntentID sets payment.intent_id.
func PaymentIntentID(id string) Attr { return String("payment.intent_id", id) }

// PaymentFailureCode sets payment.failure_code.
func PaymentFailureCode(code string) Attr { return String("payment.failure_code", code) }

// PaymentRetryAttempt sets payment.retry_attempt.
func PaymentRetryAttempt(attempt int) Attr { return Int("payment.retry_attempt", attempt) }

// ── Billing domain helpers ───────────────────────────────────────────────────

// BillingPlan sets billing.plan.
func BillingPlan(plan string) Attr { return String("billing.plan", plan) }

// BillingSubscriptionID sets billing.subscription_id.
func BillingSubscriptionID(id string) Attr { return String("billing.subscription_id", id) }

// BillingInvoiceID sets billing.invoice_id.
func BillingInvoiceID(id string) Attr { return String("billing.invoice_id", id) }

// BillingAmount sets billing.amount.
func BillingAmount(amount int64) Attr { return Int64("billing.amount", amount) }

// BillingInterval sets billing.interval.
func BillingInterval(interval string) Attr { return String("billing.interval", interval) }

// ── Agent/AI domain helpers ──────────────────────────────────────────────────

// AgentName sets agent.name.
func AgentName(name string) Attr { return String("agent.name", name) }

// AgentProvider sets agent.provider.
func AgentProvider(provider string) Attr { return String("agent.provider", provider) }

// AgentModel sets agent.model.
func AgentModel(model string) Attr { return String("agent.model", model) }

// AgentRunType sets agent.run_type.
func AgentRunType(runType string) Attr { return String("agent.run_type", runType) }

// AgentToolName sets agent.tool_name.
func AgentToolName(name string) Attr { return String("agent.tool_name", name) }

// AgentToolOutcome sets agent.tool_outcome.
func AgentToolOutcome(outcome string) Attr { return String("agent.tool_outcome", outcome) }

// AgentInputTokens sets agent.input_tokens.
func AgentInputTokens(tokens int) Attr { return Int("agent.input_tokens", tokens) }

// AgentOutputTokens sets agent.output_tokens.
func AgentOutputTokens(tokens int) Attr { return Int("agent.output_tokens", tokens) }

// AgentCost sets agent.cost.
func AgentCost(cost float64) Attr { return Float64("agent.cost", cost) }

// ── RAG domain helpers ───────────────────────────────────────────────────────

// RAGIndex sets rag.index.
func RAGIndex(index string) Attr { return String("rag.index", index) }

// RAGEmbeddingModel sets rag.embedding_model.
func RAGEmbeddingModel(model string) Attr { return String("rag.embedding_model", model) }

// RAGChunksRetrieved sets rag.chunks_retrieved.
func RAGChunksRetrieved(count int) Attr { return Int("rag.chunks_retrieved", count) }

// RAGTopScore sets rag.top_score.
func RAGTopScore(score float64) Attr { return Float64("rag.top_score", score) }

// RAGQueryHash sets rag.query_hash.
func RAGQueryHash(hash string) Attr { return String("rag.query_hash", hash) }

// RAGCitationCount sets rag.citation_count.
func RAGCitationCount(count int) Attr { return Int("rag.citation_count", count) }

// RAGRetrievalLatency sets rag.retrieval_latency_ms.
func RAGRetrievalLatency(ms int64) Attr { return Int64("rag.retrieval_latency_ms", ms) }

func joinStrings(vals []string, sep string) string {
	if len(vals) == 0 {
		return ""
	}
	out := vals[0]
	for _, v := range vals[1:] {
		out += sep + v
	}
	return out
}

from __future__ import annotations

import hashlib
from datetime import datetime, timedelta
from typing import Any as _TypingAny

from .event import Attr

def String(key: str, value: str) -> Attr: return Attr(key, value)
def Int(key: str, value: int) -> Attr: return Attr(key, value)
def Int64(key: str, value: int) -> Attr: return Attr(key, value)
def Uint64(key: str, value: int) -> Attr: return Attr(key, value)
def Float64(key: str, value: float) -> Attr: return Attr(key, value)
def Bool(key: str, value: bool) -> Attr: return Attr(key, value)
def Time(key: str, value: datetime) -> Attr: return Attr(key, value.isoformat())
def Duration(key: str, value: timedelta) -> Attr: return Attr(key, int(value.total_seconds() * 1000))
def Any(key: str, value: _TypingAny) -> Attr: return Attr(key, value)
def Null(key: str) -> Attr: return Attr(key, None)
def Group(key: str, *attrs: Attr) -> Attr:
    result: dict[str, object] = {}
    for a in attrs:
        _set_nested(result, a.key, a.value)
    return Attr(key, result)

def _set_nested(target: dict[str, object], key: str, value: object) -> None:
    if "." not in key:
        target[key] = value
        return
    parts = key.split(".")
    current = target
    for part in parts[:-1]:
        current = current.setdefault(part, {})  # type: ignore[arg-type]
    current[parts[-1]] = value
def SensitiveString(key: str, value: str) -> Attr: return Attr(key, value, sensitive=True)
def HashString(key: str, value: str) -> Attr: return Attr(key, value, hash_value=True)
def MarkSensitive(attr: Attr) -> Attr: return Attr(attr.key, attr.value, sensitive=True, hash_value=attr.hash_value, drop=attr.drop)

# --- Generic typed attr constructors ---
def list_(key: str, *values: Any) -> Attr: return Any(key, list(values))
def map_(key: str, value: dict) -> Attr: return Any(key, value)
def enum_(key: str, value: str, *allowed: str) -> Attr: return String(key, value)
def id_(key: str, value: str) -> Attr: return String(key, value)
def hash_(key: str, value: str) -> Attr: return HashString(key, value)
def redacted(key: str) -> Attr: return String(key, "[REDACTED]")
def account_id(value: str) -> Attr: return String("account.id", value)
def deployment_id(value: str) -> Attr: return String("deployment.id", value)
def http_route(value: str) -> Attr: return String("http.route", value)
def http_method(value: str) -> Attr: return String("http.method", value.upper())
def http_path(value: str) -> Attr: return String("http.path", value)
def http_user_agent(value: str) -> Attr: return String("http.user_agent", value[:512])
def http_referer(value: str) -> Attr: return String("http.referer", value.split("?", 1)[0])

# --- Identity & domain helpers ---
def payment_id(value: str) -> Attr: return String("payment.id", value)
def subscription_id(value: str) -> Attr: return String("payment.subscription_id", value)
def invoice_id(value: str) -> Attr: return String("payment.invoice_id", value)
def job_id(value: str) -> Attr: return String("job.id", value)
def message_id(value: str) -> Attr: return String("message.id", value)
def correlation_id(value: str) -> Attr: return String("correlation_id", value)
def commit_sha(value: str) -> Attr: return String("commit.sha", value)
def release(value: str) -> Attr: return String("release", value)
def money(key: str, amount_cents: int, currency: str) -> Attr: return Group(key, Int("amount_cents", amount_cents), String("currency", currency))
def percent(key: str, value: float) -> Attr: return Float64(key, value)
def bytes_attr(key: str, value: int) -> Attr: return Int64(key, value)
def http_status(key: str, value: int) -> Attr: return Int(key, value)
def status_code(key: str, value: int) -> Attr: return Int(key, value)
def error_code(key: str, value: str) -> Attr: return String(key, value)
def bucket(key: str, value: str) -> Attr: return String(key, value)
def tags(key: str, *values: str) -> Attr: return Group(key, *[String(f"{key}.{v}", v) for v in values])
def masked(key: str, value: str, prefix: int = 2, suffix: int = 2) -> Attr:
    text = str(value)
    if len(text) <= prefix + suffix:
        return String(key, "*" * max(4, len(text)))
    return String(key, text[:prefix] + "*" * (len(text) - prefix - suffix) + text[-suffix:])
def url(value: str) -> Attr: return String("url", value)
def email_hash(value: str) -> Attr: return String("email.hash", hashlib.sha256(value.strip().lower().encode("utf-8")).hexdigest())
def ip_hash(value: str) -> Attr: return String("ip.hash", hashlib.sha256(value.strip().encode("utf-8")).hexdigest())
def region(value: str) -> Attr: return String("region", value)

# --- Domain helper packs ---
def checkout_cart_item_count(value: int) -> Attr: return Int("checkout.cart_item_count", value)
def checkout_cart_total(value: int) -> Attr: return Int("checkout.cart_total", value)
def checkout_payment_method(value: str) -> Attr: return String("checkout.payment_method", value)
def checkout_status(value: str) -> Attr: return String("checkout.status", value)
def payment_provider(value: str) -> Attr: return String("payment.provider", value)
def payment_method(value: str) -> Attr: return String("payment.method", value)
def payment_intent_id(value: str) -> Attr: return String("payment.intent_id", value)
def payment_failure_code(value: str) -> Attr: return String("payment.failure_code", value)
def payment_retry_attempt(value: int) -> Attr: return Int("payment.retry_attempt", value)
def billing_plan(value: str) -> Attr: return String("billing.plan", value)
def billing_subscription_id(value: str) -> Attr: return String("billing.subscription_id", value)
def billing_invoice_id(value: str) -> Attr: return String("billing.invoice_id", value)
def billing_amount(value: int) -> Attr: return Int("billing.amount", value)
def billing_interval(value: str) -> Attr: return String("billing.interval", value)
def agent_name(value: str) -> Attr: return String("agent.name", value)
def agent_provider(value: str) -> Attr: return String("agent.provider", value)
def agent_model(value: str) -> Attr: return String("agent.model", value)
def agent_run_type(value: str) -> Attr: return String("agent.run_type", value)
def agent_tool_name(value: str) -> Attr: return String("agent.tool_name", value)
def agent_tool_outcome(value: str) -> Attr: return String("agent.tool_outcome", value)
def agent_input_tokens(value: int) -> Attr: return Int("agent.input_tokens", value)
def agent_output_tokens(value: int) -> Attr: return Int("agent.output_tokens", value)
def agent_cost(value: float) -> Attr: return Float64("agent.cost", value)
def rag_index(value: str) -> Attr: return String("rag.index", value)
def rag_embedding_model(value: str) -> Attr: return String("rag.embedding_model", value)
def rag_chunks_retrieved(value: int) -> Attr: return Int("rag.chunks_retrieved", value)
def rag_top_score(value: float) -> Attr: return Float64("rag.top_score", value)
def rag_query_hash(value: str) -> Attr: return String("rag.query_hash", value)
def rag_citation_count(value: int) -> Attr: return Int("rag.citation_count", value)
def rag_retrieval_latency(value: int) -> Attr: return Int("rag.retrieval_latency", value)

# --- PascalCase aliases ---
List = list_
Map = map_
Enum = enum_
ID = id_
Hash = hash_
Redacted = redacted
AccountID = account_id
DeploymentID = deployment_id
HTTPRoute = http_route
HTTPMethod = http_method
HTTPPath = http_path
HTTPUserAgent = http_user_agent
HTTPReferer = http_referer
PaymentID = payment_id
SubscriptionID = subscription_id
InvoiceID = invoice_id
JobID = job_id
MessageID = message_id
CorrelationID = correlation_id
CommitSHA = commit_sha
Release = release
Money = money
Percent = percent
Bytes = bytes_attr
HTTPStatus = http_status
StatusCode = status_code
Bucket = bucket
Tags = tags
Masked = masked
URL = url
EmailHash = email_hash
IPHash = ip_hash
Region = region
CheckoutCartItemCount = checkout_cart_item_count
CheckoutCartTotal = checkout_cart_total
CheckoutPaymentMethod = checkout_payment_method
CheckoutStatus = checkout_status
PaymentProvider = payment_provider
PaymentMethod = payment_method
PaymentIntentID = payment_intent_id
PaymentFailureCode = payment_failure_code
PaymentRetryAttempt = payment_retry_attempt
BillingPlan = billing_plan
BillingSubscriptionID = billing_subscription_id
BillingInvoiceID = billing_invoice_id
BillingAmount = billing_amount
BillingInterval = billing_interval
AgentName = agent_name
AgentProvider = agent_provider
AgentModel = agent_model
AgentRunType = agent_run_type
AgentToolName = agent_tool_name
AgentToolOutcome = agent_tool_outcome
AgentInputTokens = agent_input_tokens
AgentOutputTokens = agent_output_tokens
AgentCost = agent_cost
RAGIndex = rag_index
RAGEmbeddingModel = rag_embedding_model
RAGChunksRetrieved = rag_chunks_retrieved
RAGTopScore = rag_top_score
RAGQueryHash = rag_query_hash
RAGCitationCount = rag_citation_count
RAGRetrievalLatency = rag_retrieval_latency

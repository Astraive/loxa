LOZA_SPEC_VERSION = "v1"
LOZA_INGEST_API_VERSION = "v1"
LOZA_EVENT_VERSION = "v1"
REQUIRED_EVENT_FIELDS = {"schema_version", "event_version", "event_id", "timestamp", "service", "event"}

# Canonical field name aliases — older/dashed forms map to camelCase canonical.
_FIELD_ALIASES: dict[str, str] = {
    "event_id": "eventId",
    "schema_version": "schemaVersion",
    "event_version": "eventVersion",
    "started_at": "startedAt",
    "started_at_ms": "startedAtMs",
    "finished_at": "finishedAt",
    "finished_at_ms": "finishedAtMs",
    "duration_ms": "durationMs",
    "status_code": "statusCode",
    "deployment_id": "deploymentId",
    "user_id": "userId",
    "tenant_id": "tenantId",
    "session_id": "sessionId",
    "request_id": "requestId",
    "correlation_id": "correlationId",
    "trace_id": "traceId",
    "span_id": "spanId",
    "incident_id": "incidentId",
    "error_message": "errorMessage",
    "error_stack": "errorStack",
    "error_type": "errorType",
    "error_code": "errorCode",
    "order_id": "orderId",
    "cart_id": "cartId",
    "payment_id": "paymentId",
    "subscription_id": "subscriptionId",
    "invoice_id": "invoiceId",
    "job_id": "jobId",
    "message_id": "messageId",
    "org_id": "orgId",
    "account_id": "accountId",
}


def validate_event_map(event: dict[str, object]) -> tuple[bool, list[str]]:
    """Validate an event map against required Loza spec fields.

    Returns (valid, errors) where errors is a list of human-readable strings.
    """
    errors: list[str] = []
    for field in ("event_id", "eventId", "timestamp", "service", "event"):
        if field not in event:
            errors.append(f"missing required field: {field}")
    if not errors:
        return True, []
    return False, errors


def normalize_event_aliases(event: dict[str, object]) -> dict[str, object]:
    """Normalize event field names from dashed/snake_case aliases to canonical camelCase."""
    out: dict[str, object] = {}
    for key, value in event.items():
        canonical = _FIELD_ALIASES.get(key, key)
        out[canonical] = value
    return out

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from typing import Any, Iterable

CONTRACT = {
  "product_version": "0.3.0",
  "spec_version": "v1",
  "api_version": "v1",
  "event_version": "v1",
  "schemas": {
  "event": "spec/schemas/json/event.schema.json",
  "event_strict": "spec/schemas/json/event.strict.schema.json",
  "event_loose": "spec/schemas/json/event.loose.schema.json",
  "ingest_envelope": "spec/schemas/json/ingest.schema.json",
  "collector_response": "spec/schemas/json/collector-response.schema.json"
},
  "required_fields": [
  "schema_version",
  "event_version",
  "event_id",
  "timestamp",
  "service",
  "event",
  "kind"
],
  "allowed_top_level_fields": [
  "attrs",
  "checkpoints",
  "collector",
  "delivery_attempts",
  "deployment",
  "duration_ms",
  "environment",
  "error",
  "errors",
  "event",
  "event_id",
  "event_state",
  "event_version",
  "finished_at",
  "groups",
  "http",
  "incident_id",
  "kind",
  "level",
  "links",
  "message",
  "method",
  "organization",
  "outcome",
  "partial",
  "partial_reason",
  "path",
  "pii",
  "processes",
  "redaction",
  "release",
  "request_id",
  "resource",
  "route",
  "sampling",
  "schema_version",
  "sdk",
  "service",
  "source",
  "span_id",
  "started_at",
  "status_code",
  "tenant",
  "timers",
  "timestamp",
  "trace_flags",
  "trace_id",
  "user",
  "version",
  "workspace"
],
  "canonical_fields": [
  "attrs",
  "checkpoints",
  "collector",
  "delivery_attempts",
  "deployment",
  "duration_ms",
  "environment",
  "error",
  "errors",
  "event",
  "event_id",
  "event_state",
  "event_version",
  "finished_at",
  "groups",
  "http",
  "incident_id",
  "kind",
  "level",
  "links",
  "message",
  "method",
  "organization",
  "outcome",
  "partial",
  "partial_reason",
  "path",
  "pii",
  "processes",
  "redaction",
  "release",
  "request_id",
  "resource",
  "route",
  "sampling",
  "schema_version",
  "sdk",
  "service",
  "source",
  "span_id",
  "started_at",
  "status_code",
  "tenant",
  "timers",
  "timestamp",
  "trace_flags",
  "trace_id",
  "user",
  "version",
  "workspace"
],
  "enums": {
  "schema_versions": [
  "v1"
],
  "event_versions": [
  "v1"
],
  "kinds": [
  "event",
  "http",
  "job",
  "queue",
  "cli",
  "cron",
  "log",
  "checkpoint",
  "agent",
  "ai"
],
  "levels": [
  "debug",
  "info",
  "notice",
  "warn",
  "error",
  "fatal"
],
  "outcomes": [
  "success",
  "error",
  "partial",
  "abandoned",
  "retried",
  "cancelled",
  "timeout",
  "skipped",
  "rejected",
  "quarantined",
  "unknown"
],
  "partial_reasons": [
  "not_finished",
  "process_exit",
  "timeout",
  "panic",
  "collector_unavailable"
],
  "event_states": [
  "created",
  "active",
  "finished",
  "emitting",
  "emitted",
  "invalid",
  "dropped",
  "emit_failed",
  "spooled",
  "dlq_written",
  "failed_validation",
  "delivery_failed"
],
  "source_sdks": [
  "loza-cli",
  "loza-go",
  "loza-py",
  "loza-rs"
]
},
  "collector_statuses": [
  "accepted",
  "partial",
  "rejected",
  "invalid",
  "quarantined"
],
  "collector_ack_statuses": [
  "accepted",
  "rejected",
  "invalid",
  "quarantined"
],
  "aliases": {
  "event_type": "event"
},
  "alias_policy": {
  "strict_mode": "reject_before_normalization",
  "loose_mode": "normalize_then_use"
},
  "strict_mode": {
  "allow_unknown_top_level_fields": False,
  "allow_aliases": False,
  "enforce_required_fields": True,
  "enforce_enums": True,
  "enforce_status_codes": True,
  "enforce_timestamps": True,
  "normalize_aliases": False
},
  "loose_mode": {
  "allow_unknown_top_level_fields": True,
  "allow_aliases": True,
  "enforce_required_fields": True,
  "enforce_enums": True,
  "normalize_aliases": True
},
  "validation_modes": {
  "off": {
  "name": "off",
  "description": "Accept everything, still normalize payload",
  "accept_all": True,
  "normalize": True,
  "validate": False,
  "reject_on_failure": False
},
  "warn": {
  "name": "warn",
  "description": "Accept and report schema issues as warnings",
  "accept_all": True,
  "normalize": True,
  "validate": True,
  "reject_on_failure": False
},
  "enforce": {
  "name": "enforce",
  "description": "Reject invalid events, accept valid ones only",
  "accept_all": False,
  "normalize": True,
  "validate": True,
  "reject_on_failure": True
},
  "quarantine": {
  "name": "quarantine",
  "description": "Store invalid events separately in quarantine, accept valid ones",
  "accept_all": False,
  "normalize": True,
  "validate": True,
  "reject_on_failure": False,
  "quarantine_on_failure": True
},
  "strict": {
  "name": "strict",
  "description": "Reject unknown fields, aliases, and enforce all constraints",
  "allow_aliases": False,
  "allow_unknown_top_level_fields": False,
  "reject_on_failure": True
},
  "loose": {
  "name": "loose",
  "description": "Accept aliases and some unknown fields, normalize before validation",
  "allow_aliases": True,
  "allow_unknown_top_level_fields": True
}
},
  "wire_formats": [
  "json",
  "jsonl",
  "protobuf"
],
  "limits": {
  "max_event_size_bytes": 65536,
  "max_batch_events": 1000,
  "max_batch_size_bytes": 1048576,
  "max_attrs_depth": 8,
  "max_attr_key_length": 128,
  "max_attr_value_length": 4096,
  "max_error_stack_length": 16384
},
  "fixtures": {
  "version": "v1",
  "product_version": "0.3.0",
  "strict_schema": "../schema/event.strict.schema.json",
  "loose_schema": "../schema/event.loose.schema.json",
  "valid": [
  "valid/http_success.json",
  "valid/http_error.json",
  "valid/job_success.json",
  "valid/queue_retry.json",
  "valid/cron_run.json",
  "valid/partial_abandoned.json",
  "valid/cli_run.json",
  "valid/duplicate_fields.json",
  "valid/minimal_event.json",
  "valid/error_event.json",
  "valid/trace_context_event.json",
  "valid/agent_run.json",
  "valid/rag_query.json",
  "valid/release_field.json",
  "valid/notice_level.json"
],
  "loose_only_valid": [
  "valid/loose_event_type_alias.json"
],
  "invalid": [
  "invalid/missing_event_id.json",
  "invalid/missing_versions.json",
  "invalid/bad_timestamp.json",
  "invalid/bad_duration.json",
  "invalid/invalid_enum_values.json"
],
  "strict_only_invalid": [
  "invalid/strict_event_type_alias.json"
],
  "invalid_ingest": [
  "invalid/bad_ingest_events_array.json"
],
  "invalid_collector_response": [
  "invalid/bad_collector_status.json"
],
  "invalid_limits": [
  "invalid/oversized.json"
],
  "emitted_shape": [
  "emitted-shape/structured_http_success.json"
],
  "collector_ack_behavior": [
  "valid/collector_ack.json",
  "collector-acks/accepted_clean.json",
  "collector-acks/accepted_duplicate.json",
  "collector-acks/partial_invalid.json",
  "collector-acks/retryable_rate_limited.json",
  "collector-acks/partial_quarantined.json"
],
  "ingest_envelopes": [
  "ingest-envelopes/single_event_json.json",
  "ingest-envelopes/wrapped_batch_json.json",
  "ingest-envelopes/ndjson_ingest.json"
],
  "fixtures_by_coverage": {
  "http_success.json": "Standard HTTP event with all canonical fields",
  "http_error.json": "HTTP error response event",
  "error_event.json": "Error/exception event with stack trace",
  "job_success.json": "Background job success event",
  "queue_retry.json": "Queue/retry event",
  "cron_run.json": "Cron/scheduled task event",
  "partial_abandoned.json": "Partial/abandoned outcome event",
  "cli_run.json": "CLI/process event",
  "collector_ack.json": "Collector acknowledgment response payload",
  "duplicate_fields.json": "Canonical duplicate field handling test",
  "minimal_event.json": "Minimal valid event (required fields only)",
  "loose_event_type_alias.json": "Legacy event_type alias accepted only in loose mode",
  "strict_event_type_alias.json": "Legacy event_type alias rejected in strict mode",
  "missing_versions.json": "Missing schema/event versions should fail event validation",
  "bad_ingest_events_array.json": "Ingest envelope with non-array events should fail ingest schema",
  "bad_collector_status.json": "Collector response with unsupported status should fail collector schema",
  "trace_context_event.json": "Distributed trace context propagation",
  "agent_run.json": "AI agent run event with agent kind and processes",
  "rag_query.json": "RAG pipeline query event with ai kind",
  "release_field.json": "Event with release and trace_flags fields",
  "notice_level.json": "Event with notice level",
  "structured_http_success.json": "Cross-SDK emitted payload layout with structured groups and finished event_state",
  "accepted_clean.json": "Collector sink should treat clean accepted acknowledgements as success",
  "accepted_duplicate.json": "Collector sink should treat duplicate accepted acknowledgements as success",
  "partial_invalid.json": "Collector sink should fail on partial invalid acknowledgements",
  "retryable_rate_limited.json": "Collector sink should fail on retryable/rate-limited responses",
  "single_event_json.json": "Collector accepts a single-event JSON ingest payload",
  "wrapped_batch_json.json": "Canonical wrapped batch envelope generated by SDK sinks",
  "ndjson_ingest.json": "Collector accepts NDJSON ingest payloads"
}
},
  "paths": {
  "event_schema": "schemas/json/event.schema.json",
  "strict_schema": "schemas/json/event.strict.schema.json",
  "loose_schema": "schemas/json/event.loose.schema.json",
  "ingest_schema": "schemas/json/ingest.schema.json",
  "collector_response_schema": "schemas/json/collector-response.schema.json",
  "manifest": "conformance/manifest.json"
}
}
LOZA_SPEC_VERSION = CONTRACT['spec_version']
LOZA_INGEST_API_VERSION = CONTRACT['api_version']
LOZA_EVENT_VERSION = CONTRACT['event_version']
MAX_EVENT_BYTES = CONTRACT['limits']['max_event_size_bytes']
ALLOWED_TOP_LEVEL_FIELDS = set(CONTRACT['allowed_top_level_fields'])
CANONICAL_FIELDS = set(CONTRACT['canonical_fields'])
ALLOWED_KINDS = set(CONTRACT['enums']['kinds'])
ALLOWED_LEVELS = set(CONTRACT['enums']['levels'])
ALLOWED_OUTCOMES = set(CONTRACT['enums']['outcomes'])
ALLOWED_PARTIAL_REASONS = set(CONTRACT['enums']['partial_reasons'])
ALLOWED_EVENT_STATES = set(CONTRACT['enums']['event_states'])
ALLOWED_COLLECTOR_STATUSES = set(CONTRACT['collector_statuses'])
NON_ACCEPTED_COLLECTOR_STATUSES = ALLOWED_COLLECTOR_STATUSES - {'accepted'}
RFC3339_RE = re.compile(r'^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2}))$')

@dataclass(slots=True)
class ValidationError:
    field: str
    code: str
    message: str
    event_id: str = ''
    retryable: bool = False

class ValidationErrors(ValueError):
    def __init__(self, errors: list[ValidationError]) -> None:
        self.errors = errors
        super().__init__(errors[0].message if errors else 'validation failed')

@dataclass(slots=True)
class CollectorAck:
    event_id: str = ''
    status: str = ''
    retryable: bool = False
    reason: str = ''
    message: str = ''

@dataclass(slots=True)
class CollectorError:
    code: str = ''
    message: str = ''
    retryable: bool = False
    field: str = ''
    event_id: str = ''

@dataclass(slots=True)
class CollectorResponse:
    request_id: str = ''
    status: str = ''
    accepted: int = 0
    rejected: int = 0
    invalid: int = 0
    deduped: int = 0
    reason: str = ''
    error: str = ''
    acks: list[CollectorAck] = field(default_factory=list)
    errors: list[CollectorError] = field(default_factory=list)

    def retryable_error(self) -> tuple[bool, str]:
        for item in self.errors:
            if item.retryable:
                return True, item.message or item.code
        for ack in self.acks:
            if ack.retryable:
                return True, ack.message or ack.reason or ack.status
        return False, ''

    def permanent_failure(self) -> tuple[bool, str]:
        if self.rejected <= 0 and self.invalid <= 0 and self.status not in NON_ACCEPTED_COLLECTOR_STATUSES:
            return False, ''
        for ack in self.acks:
            if not ack.retryable and ack.status in {'rejected', 'invalid'}:
                return True, ack.reason or ack.message or ack.status
        for item in self.errors:
            if not item.retryable and (item.message or item.code):
                return True, item.message or item.code
        return True, self.error or self.reason or f'accepted={self.accepted} rejected={self.rejected} invalid={self.invalid}'

def parse_collector_response(payload: dict[str, Any]) -> CollectorResponse:
    return CollectorResponse(
        request_id=str(payload.get('request_id', '')),
        status=str(payload.get('status', '')),
        accepted=int(payload.get('accepted', 0)),
        rejected=int(payload.get('rejected', 0)),
        invalid=int(payload.get('invalid', 0)),
        deduped=int(payload.get('deduped', 0)),
        reason=str(payload.get('reason', '')),
        error=str(payload.get('error', '')),
        acks=[CollectorAck(event_id=str(item.get('event_id', '')), status=str(item.get('status', '')), retryable=bool(item.get('retryable', False)), reason=str(item.get('reason', '')), message=str(item.get('message', ''))) for item in payload.get('acks', []) if isinstance(item, dict)],
        errors=[CollectorError(code=str(item.get('code', '')), message=str(item.get('message', '')), retryable=bool(item.get('retryable', False)), field=str(item.get('field', '')), event_id=str(item.get('event_id', ''))) for item in payload.get('errors', []) if isinstance(item, dict)],
    )

def looks_like_loza_event(payload: dict[str, Any]) -> bool:
    return any(key in payload for key in ('schema_version', 'event_version', 'event', 'event_type'))

def normalize_event_aliases(payload: dict[str, Any]) -> tuple[dict[str, Any], bool]:
    normalized = dict(payload)
    if isinstance(normalized.get('event'), str) and normalized['event'].strip():
        if 'event_type' in normalized:
            normalized.pop('event_type', None)
            return normalized, True
        return normalized, False
    alias = normalized.get('event_type')
    if isinstance(alias, str) and alias.strip():
        normalized['event'] = alias.strip()
        normalized.pop('event_type', None)
        return normalized, True
    return normalized, False

def normalize_event_aliases_in_place(payload: dict[str, Any]) -> bool:
    normalized, changed = normalize_event_aliases(payload)
    payload.clear()
    payload.update(normalized)
    return changed

def build_ingest_envelope(events: Iterable[dict[str, Any]], sdk_name: str, sdk_version: str, service: str) -> dict[str, Any]:
    events_list = [normalize_event_aliases(e)[0] for e in events]
    return {'api_version': LOZA_INGEST_API_VERSION, 'source': {'sdk': sdk_name, 'version': sdk_version, 'service': service.strip() or _infer_service(events_list)}, 'events': events_list}

def validate_event_payload(payload: dict[str, Any], strict: bool) -> None:
    errors = validate_event_payload_detailed(payload, strict)
    if errors:
        raise ValidationErrors(errors)

def validate_event_payload_detailed(payload: dict[str, Any], strict: bool) -> list[ValidationError]:
    normalized, _ = normalize_event_aliases(payload)
    errors: list[ValidationError] = []
    if strict:
        for key in normalized:
            if key not in ALLOWED_TOP_LEVEL_FIELDS:
                errors.append(ValidationError(field=key, code='unknown_strict_field', message=f'field "{key}" is not allowed by strict schema'))
    _req_str(normalized, 'schema_version', 'unsupported_schema_version', errors)
    _req_str(normalized, 'event_version', 'unsupported_event_version', errors)
    _req_str(normalized, 'event_id', 'missing_event_id', errors)
    _req_str(normalized, 'event', 'missing_event', errors)
    _req_ts(normalized, 'timestamp', errors)
    if normalized.get('schema_version') not in {LOZA_SPEC_VERSION}:
        errors.append(ValidationError(field='schema_version', code='unsupported_schema_version', message=f'field "schema_version" has unsupported value "{normalized.get("schema_version")}"'))
    if normalized.get('event_version') not in {LOZA_EVENT_VERSION}:
        errors.append(ValidationError(field='event_version', code='unsupported_event_version', message=f'field "event_version" has unsupported value "{normalized.get("event_version")}"'))
    _req_service(normalized, errors)
    _req_enum(normalized, 'kind', ALLOWED_KINDS, errors)
    _opt_enum(normalized, 'level', ALLOWED_LEVELS, errors)
    _opt_enum(normalized, 'outcome', ALLOWED_OUTCOMES, errors)
    _opt_enum(normalized, 'partial_reason', ALLOWED_PARTIAL_REASONS, errors)
    _opt_enum(normalized, 'event_state', ALLOWED_EVENT_STATES, errors)
    _opt_nonneg_int(normalized, 'duration_ms', errors)
    _opt_status_code(normalized, 'status_code', errors)
    return errors

def validate_flexible_json_bytes(raw: bytes, strict: bool) -> None:
    trimmed = raw.strip()
    if not trimmed:
        raise ValidationErrors([ValidationError(field='payload', code='empty_payload', message='payload is empty')])
    if len(trimmed) > MAX_EVENT_BYTES and b'"events"' not in trimmed and b'\n' not in trimmed:
        raise ValidationErrors([ValidationError(field='payload', code='too_large', message=f'payload exceeds max_event_size_bytes ({len(trimmed)} > {MAX_EVENT_BYTES})')])
    try:
        payload = json.loads(trimmed)
    except json.JSONDecodeError:
        for line in trimmed.splitlines():
            if line.strip():
                validate_event_payload(json.loads(line), strict)
        return
    if isinstance(payload, dict) and 'events' in payload and 'api_version' not in payload:
        for event in payload['events']:
            validate_event_payload(event, strict)
        return
    if isinstance(payload, list):
        for event in payload:
            validate_event_payload(event, strict)
        return
    if isinstance(payload, dict):
        validate_event_payload(payload, strict)
        return
    raise ValidationErrors([ValidationError(field='payload', code='invalid_payload', message='payload must be a JSON object, array, wrapper, or NDJSON')])

def _infer_service(events: list[dict[str, Any]]) -> str:
    for event in events:
        service = event.get('service')
        if isinstance(service, str) and service.strip():
            return service
    return 'unknown'

def _req_str(payload: dict[str, Any], key: str, code: str, errors: list[ValidationError]) -> None:
    value = payload.get(key)
    if not isinstance(value, str) or not value.strip():
        errors.append(ValidationError(field=key, code=code, message=f'field "{key}" must be a non-empty string'))

def _req_enum(payload: dict[str, Any], key: str, allowed: set[str], errors: list[ValidationError]) -> None:
    value = payload.get(key)
    if isinstance(value, str) and value.strip() and value.strip() not in allowed:
        errors.append(ValidationError(field=key, code='invalid_enum', message=f'field "{key}" has unsupported value "{value}"'))

def _opt_enum(payload: dict[str, Any], key: str, allowed: set[str], errors: list[ValidationError]) -> None:
    if key not in payload:
        return
    value = payload[key]
    if not isinstance(value, str) or not value.strip() or value.strip() not in allowed:
        errors.append(ValidationError(field=key, code='invalid_enum', message=f'field "{key}" has unsupported value "{value}"'))

def _opt_nonneg_int(payload: dict[str, Any], key: str, errors: list[ValidationError]) -> None:
    if key in payload and (not isinstance(payload[key], int) or payload[key] < 0):
        errors.append(ValidationError(field=key, code='invalid_integer', message=f'field "{key}" must be a non-negative integer'))

def _opt_status_code(payload: dict[str, Any], key: str, errors: list[ValidationError]) -> None:
    if key in payload and (not isinstance(payload[key], int) or payload[key] < 100 or payload[key] > 599):
        errors.append(ValidationError(field=key, code='invalid_status_code', message=f'field "{key}" must be between 100 and 599'))

def _req_service(payload: dict[str, Any], errors: list[ValidationError]) -> None:
    value = payload.get('service')
    if isinstance(value, str) and value.strip():
        return
    if isinstance(value, dict) and isinstance(value.get('name'), str) and value['name'].strip():
        return
    errors.append(ValidationError(field='service', code='invalid_service', message='field "service" must be a non-empty string or object with name'))

def _req_ts(payload: dict[str, Any], key: str, errors: list[ValidationError]) -> None:
    value = payload.get(key)
    if not isinstance(value, str) or not RFC3339_RE.match(value.strip()):
        errors.append(ValidationError(field=key, code='invalid_rfc3339', message=f'field "{key}" must be RFC3339'))

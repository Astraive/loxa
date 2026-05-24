/**
 * LOXA Spec Contract — constants and validation from loxa-spec.
 * Generated from loxa-spec/spec/schemas/json/event.schema.json
 */

export const LOXA_SPEC_VERSION = 'v1';
export const LOXA_EVENT_VERSION = 'v1';
export const LOXA_INGEST_API_VERSION = 'v1';
export const MAX_EVENT_BYTES = 65536;

export const ALLOWED_KINDS = new Set(['event', 'http', 'job', 'queue', 'cli', 'cron', 'log', 'checkpoint']);
export const ALLOWED_LEVELS = new Set(['debug', 'info', 'warn', 'error', 'fatal']);
export const ALLOWED_OUTCOMES = new Set(['success', 'error', 'partial', 'abandoned', 'retried', 'cancelled', 'timeout', 'skipped', 'rejected', 'quarantined', 'unknown']);
export const ALLOWED_EVENT_STATES = new Set(['created', 'active', 'finished', 'emitting', 'emitted', 'invalid', 'dropped', 'emit_failed', 'spooled', 'dlq_written', 'failed_validation', 'delivery_failed']);

export const CANONICAL_FIELDS = new Set([
  'attrs', 'checkpoints', 'collector', 'delivery_attempts', 'deployment', 'duration_ms',
  'environment', 'error', 'errors', 'event', 'event_id', 'event_state', 'event_version',
  'groups', 'http', 'kind', 'level', 'links', 'message', 'method', 'organization', 'outcome',
  'partial', 'partial_reason', 'path', 'pii', 'processes', 'redaction', 'request_id', 'resource',
  'route', 'sampling', 'schema_version', 'sdk', 'service', 'source', 'span_id', 'status_code',
  'tenant', 'timestamp', 'timers', 'trace_flags', 'trace_id', 'user', 'version', 'workspace',
]);

export const ALLOWED_TOP_LEVEL_FIELDS = new Set([
  ...CANONICAL_FIELDS,
  'host', 'runtime', 'region', 'deployment_id', 'parent_id',
  'finished_at', 'custom',
]);

export interface CollectorAck {
  event_id: string;
  status: string;
  retryable: boolean;
  reason?: string;
  code?: string;
  message?: string;
}

export interface CollectorError {
  index: number;
  event_id: string;
  code: string;
  message: string;
  retryable: boolean;
}

export interface CollectorResponse {
  request_id: string;
  status: string;
  accepted: number;
  rejected: number;
  invalid: number;
  deduped?: number;
  duplicates?: number;
  retry_after_ms?: number;
  error?: string;
  reason?: string;
  acks: CollectorAck[];
  errors?: CollectorError[];
}

/** Normalize event aliases: event_type → event. Returns a copy. */
export function normalizeEventAliases(payload: Record<string, any>): Record<string, any> {
  const normalized = { ...payload };
  if (typeof normalized.event === 'string' && normalized.event.trim()) {
    if ('event_type' in normalized) {
      delete normalized.event_type;
    }
    return normalized;
  }
  const alias = normalized.event_type;
  if (typeof alias === 'string' && alias.trim()) {
    normalized.event = alias.trim();
    delete normalized.event_type;
    return normalized;
  }
  return normalized;
}

/** Build an ingest envelope for the collector, normalizing event aliases. */
export function buildIngestEnvelope(
  sdkName: string,
  sdkVersion: string,
  service: string,
  events: Record<string, any>[],
): Record<string, any> {
  const normalizedEvents = events.map(normalizeEventAliases);
  return {
    api_version: LOXA_INGEST_API_VERSION,
    source: { sdk: sdkName, version: sdkVersion, service },
    events: normalizedEvents,
  };
}

/** Parse a collector response. */
export function parseCollectorResponse(raw: string): CollectorResponse {
  return JSON.parse(raw);
}

/** Check if a key is in the canonical field set. */
export function isCanonical(key: string): boolean {
  return CANONICAL_FIELDS.has(key);
}

/**
 * Validate an event payload against the spec contract.
 * Returns true if valid, throws Error if invalid.
 */
export function validateEvent(payload: Record<string, any>, strict = true): boolean {
  if (!payload || typeof payload !== 'object') {
    throw new Error('Event payload must be a non-null object');
  }
  if (strict) {
    if (!payload.event || typeof payload.event !== 'string' || !payload.event.trim()) {
      throw new Error('Event must have a non-empty "event" field');
    }
    if (!ALLOWED_KINDS.has(payload.kind ?? 'event')) {
      throw new Error(`Invalid kind: ${payload.kind}`);
    }
    if (payload.level && !ALLOWED_LEVELS.has(payload.level)) {
      throw new Error(`Invalid level: ${payload.level}`);
    }
  } else {
    if (payload.event_type && !payload.event) {
      payload.event = payload.event_type;
      delete payload.event_type;
    }
  }
  return true;
}

/** NormalizeEvent aliases normalizeEventAliases — cross-SDK naming parity. */
export const normalizeEvent = normalizeEventAliases;

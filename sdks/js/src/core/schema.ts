import type { EventView } from './event-view.ts';
import { levelName } from './level.ts';

/** Schema interface — controls how events are encoded. */
export interface Schema {
  encode(view: EventView): Record<string, any>;
}

export type SchemaFunc = (view: EventView) => Record<string, any>;

/** Set a value in a nested object using dot-separated key path. */
function setNestedValue(obj: Record<string, any>, key: string, value: any): void {
  const parts = key.split('.');
  let current = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const part = parts[i];
    if (!(part in current) || typeof current[part] !== 'object' || current[part] === null) {
      current[part] = {};
    }
    current = current[part];
  }
  current[parts[parts.length - 1]] = value;
}

/** Known group prefixes for routing attrs to top-level objects. */
const GROUP_PREFIXES = new Set([
  'user', 'tenant', 'http', 'resource',
]);

/** Default schema — emits all event fields with group routing. */
export class DefaultSchema implements Schema {
  encode(view: EventView): Record<string, any> {
    let state = view.state as string;
    if (state === 'emitting' && view.outcome) {
      state = 'finished';
    }

    const out: Record<string, any> = {
      schema_version: view.schemaVersion,
      event_version: view.eventVersion,
      event_id: view.eventId,
      request_id: view.requestId,
      timestamp: view.timestamp,
      service: view.service,
      event: view.event,
      kind: view.kind,
      level: levelName(view.level),
      event_state: state,
    };

    if (view.message) out.message = view.message;
    if (view.outcome) out.outcome = view.outcome;
    if (view.version) out.version = view.version;
    if (view.environment) out.environment = view.environment;
    if (view.deploymentId) out.deployment_id = view.deploymentId;
    if (view.region) out.region = view.region;
    if (view.host) out.host = view.host;
    if (view.runtime) out.runtime = view.runtime;
    if (view.durationMs) out.duration_ms = view.durationMs;
    if (view.traceId) out.trace_id = view.traceId;
    if (view.spanId) out.span_id = view.spanId;
    if (view.parentId) out.parent_id = view.parentId;
    if (view.finishedAt) out.finished_at = new Date(view.finishedAt).toISOString();

    // Route attrs by prefix into groups
    const groups: Record<string, Record<string, any>> = {};
    const leftover: Record<string, any> = {};

    for (const [k, v] of Object.entries(view.attrs)) {
      const dotIdx = k.indexOf('.');
      if (dotIdx > 0) {
        const prefix = k.substring(0, dotIdx);
        if (GROUP_PREFIXES.has(prefix)) {
          if (!groups[prefix]) groups[prefix] = {};
          groups[prefix][k.substring(dotIdx + 1)] = v;
          continue;
        }
      }
      // Expand dot-separated keys into nested objects
      setNestedValue(leftover, k, v);
    }

    // Merge HTTP fields from event params into http group (matching Go behavior)
    if (view.method || view.path || view.route || view.statusCode || groups.http) {
      const httpOut: Record<string, any> = { ...(groups.http || {}) };
      if (view.method) httpOut.method = view.method;
      if (view.path) httpOut.path = view.path;
      if (view.route) httpOut.route = view.route;
      if (view.statusCode) httpOut.status_code = view.statusCode;
      if (Object.keys(httpOut).length > 0) {
        out.http = httpOut;
      }
      delete groups.http;
    }

    // Emit remaining groups as top-level objects
    for (const [prefix, groupAttrs] of Object.entries(groups)) {
      if (Object.keys(groupAttrs).length > 0) {
        out[prefix] = groupAttrs;
      }
    }

    // Emit leftover attrs
    if (Object.keys(leftover).length > 0) {
      out.attrs = leftover;
    }

    if (view.checkpoints.length > 0) {
      out.checkpoints = [...view.checkpoints];
    }
    if (view.processes && view.processes.length > 0) {
      out.processes = view.processes.map(p => ({ ...p }));
    }
    if (view.groups && view.groups.length > 0) {
      out.groups = view.groups.map(g => ({ ...g }));
    }
    if (view.timers && view.timers.length > 0) {
      out.timers = view.timers.map(t => ({ ...t }));
    }
    if (view.error) {
      out.error = { ...view.error };
    }

    return out;
  }
}

/** Nested schema — alias for DefaultSchema (canonical fields with nested attrs/groups). */
export class NestedSchema extends DefaultSchema {}

/** OTel log schema — emits OpenTelemetry-flavored log shape. */
export class OTelLogSchema implements Schema {
  encode(view: EventView): Record<string, any> {
    const out: Record<string, any> = {
      timestamp: view.timestamp,
      severity: levelName(view.level),
      body: view.event,
      attributes: { ...view.attrs },
    };
    if (view.requestId) out.request_id = view.requestId;
    if (view.service) out['service.name'] = view.service;
    if (view.error) {
      out.error = {
        type: view.error.type,
        message: view.error.message,
        code: (view.error as any).code,
        retryable: view.error.retryable,
      };
    }
    return out;
  }
}

/** OTel schema — alias for OTelLogSchema. */
export class OTelSchema extends OTelLogSchema {}

/** ECSchema — emits Elastic Common Schema-inspired log shape. */
export class ECSchema implements Schema {
  encode(view: EventView): Record<string, any> {
    const out: Record<string, any> = {
      '@timestamp': view.timestamp,
      event: {
        id: view.eventId,
        action: view.event,
        outcome: view.outcome,
        duration: view.durationMs * 1_000_000, // ms to nanoseconds
      },
      log: {
        level: levelName(view.level),
      },
      labels: { ...view.attrs },
    };
    if (view.service) {
      out.service = { name: view.service };
    }
    if (view.requestId) {
      out.trace = { id: view.requestId };
    }
    if (view.error) {
      out.error = {
        type: view.error.type,
        message: view.error.message,
        code: (view.error as any).code,
        stack: view.error.stack,
      };
    }
    return out;
  }
}

/** Datadog schema — emits Datadog-like JSON shape. */
export class DatadogSchema implements Schema {
  encode(view: EventView): Record<string, any> {
    const out: Record<string, any> = {
      timestamp: new Date(view.timestamp).getTime(),
      status: levelName(view.level),
      message: view.event,
      service: view.service,
      ddtags: attrsToTagString(view.attrs),
      fields: { ...view.attrs },
    };
    if (view.requestId) out.request_id = view.requestId;
    if (view.error) {
      out.error = {
        type: view.error.type,
        message: view.error.message,
        code: (view.error as any).code,
      };
    }
    return out;
  }
}

function attrsToTagString(attrs: Record<string, any>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(attrs)) {
    if (typeof v !== 'object') {
      parts.push(`${k}:${v}`);
    }
  }
  parts.sort();
  return parts.join(',');
}

/** Custom schema — creates a schema from a projection function. */
export function CustomSchema(fn: (view: EventView) => Record<string, any>): Schema {
  return { encode: fn };
}

/** Flat schema — flattens nested attrs with dot keys. */
export class FlatSchema implements Schema {
  private inner = new DefaultSchema();

  encode(view: EventView): Record<string, any> {
    const base = this.inner.encode(view);
    const flat: Record<string, any> = {};

    function flatten(obj: any, prefix: string = ''): void {
      for (const [k, v] of Object.entries(obj)) {
        const key = prefix ? `${prefix}.${k}` : k;
        if (k === 'attrs' && typeof v === 'object' && v && !Array.isArray(v)) {
          flatten(v, '');
        } else if (typeof v === 'object' && v && !Array.isArray(v) && k !== 'error') {
          flatten(v, key);
        } else {
          flat[k === 'attrs' ? key : key] = v;
        }
      }
    }

    flatten(base);
    return flat;
  }
}

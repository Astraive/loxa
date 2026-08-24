import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Event } from '../src/core/event.ts';
import { EventView } from '../src/core/event-view.ts';
import {
  DefaultSchema, NestedSchema, OTelLogSchema, OTelSchema, ECSchema,
  DatadogSchema, CustomSchema, FlatSchema,
} from '../src/core/schema.ts';

describe('Schema boundaries', () => {
  it('routes default fields, groups, nested attrs, and collections', () => {
    const event = new Event({
      event: 'checkout.completed', service: 'checkout', kind: 'http', level: 'debug',
      message: 'ok', version: '1.2.3', environment: 'production', deploymentId: 'dep',
      region: 'us-east', host: 'host-1', runtime: 'node', method: 'POST', path: '/checkout',
      route: '/checkout', statusCode: 201, durationMs: 12,
      requestId: 'req', traceId: 'trace', spanId: 'span', incidentId: 'incident', parentId: 'parent',
    }, 'checkout', 'development');
    event.attrs = {
      'user.id': 'u1', 'http.user_agent': 'ua', 'resource.name': 'checkout',
      'custom.deep.value': 42, 'simple': true,
    };
    event.checkpoints = [{ name: 'db', at_ms: 2 }];
    event.processes = [{ step: 1, name: 'worker', started_at_ms: 1, ended_at_ms: 2, duration_ms: 1 }];
    event.groups = [{ name: 'group', started_at_ms: 1, ended_at_ms: 2, duration_ms: 1 }];
    event.timers = [{ name: 'timer', duration_ms: 1 }];
    event.error = { type: 'Error', message: 'failed', stack: 'stack', code: 'E_FAIL', retryable: true };
    event.finishedAt = Date.parse('2025-01-01T00:00:00.000Z');
    const view = new EventView(event);
    const encoded = new DefaultSchema().encode(view);
    assert.equal(encoded.event_state, 'created');
    assert.deepEqual(encoded.http, { user_agent: 'ua', method: 'POST', path: '/checkout', route: '/checkout', status_code: 201 });
    assert.deepEqual(encoded.user, { id: 'u1' });
    assert.deepEqual(encoded.resource, { name: 'checkout' });
    assert.deepEqual(encoded.attrs, { custom: { deep: { value: 42 } }, simple: true });
    assert.equal(encoded.finished_at, '2025-01-01T00:00:00.000Z');
    assert.equal(Array.isArray(encoded.checkpoints), true);
    assert.equal(Array.isArray(encoded.processes), true);
    assert.equal(Array.isArray(encoded.groups), true);
    assert.equal(Array.isArray(encoded.timers), true);
    assert.deepEqual(encoded.error, event.error);
  });

  it('normalizes emitting state with an outcome and omits empty optional fields', () => {
    const event = new Event({ event: 'done', service: 'svc', outcome: 'success' }, 'svc', 'test');
    const encoded = new DefaultSchema().encode(new EventView(event));
    assert.equal(encoded.event_state, 'created');
    assert.equal('message' in encoded, false);
    assert.equal('attrs' in encoded, false);
    assert.equal('error' in encoded, false);
  });

  it('encodes OTel, ECS, Datadog, flat, nested, and custom schemas', () => {
    const event = new Event({ event: 'query', service: 'svc', requestId: 'req', level: 'warn', durationMs: 3 }, 'svc', 'test');
    event.attrs = { plain: 'value', nested: { object: true } };
    event.error = { type: 'Error', message: 'bad', stack: 'stack', code: 'E_BAD', retryable: false };
    const view = new EventView(event);
    const otel = new OTelLogSchema().encode(view);
    assert.equal(otel.body, 'query');
    assert.equal(otel['service.name'], 'svc');
    assert.equal(otel.request_id, 'req');
    assert.deepEqual(otel.error, { type: 'Error', message: 'bad', code: 'E_BAD', retryable: false });
    assert.equal(new OTelSchema().encode(view).body, 'query');

    const ecs = new ECSchema().encode(view);
    assert.equal(ecs.event.duration, 3_000_000);
    assert.deepEqual(ecs.service, { name: 'svc' });
    assert.deepEqual(ecs.trace, { id: 'req' });
    assert.equal(ecs.error.message, 'bad');

    const datadog = new DatadogSchema().encode(view);
    assert.equal(datadog.message, 'query');
    assert.equal(datadog.ddtags, 'plain:value');
    assert.equal(datadog.request_id, 'req');
    assert.equal(datadog.error.code, 'E_BAD');

    const flat = new FlatSchema().encode(view);
    assert.equal(flat.plain, 'value');
    assert.deepEqual(flat['nested.object'], true);
    assert.deepEqual(flat.error, { type: 'Error', message: 'bad', code: 'E_BAD', stack: 'stack', retryable: false });

    const custom = CustomSchema(input => ({ name: input.event, attrs: input.attrs })).encode(view);
    assert.deepEqual(custom, { name: 'query', attrs: event.attrs });
    assert.equal(new NestedSchema().encode(view).event, 'query');
  });
  it('normalizes sparse event collection fields in EventView', () => {
    const event = new Event({ event: 'sparse', service: 'svc' }, 'svc', 'test');
    const sparse = { ...event, processes: undefined, groups: undefined, timers: undefined } as unknown as Event;
    const view = new EventView(sparse);
    assert.deepEqual(view.processes, []);
    assert.deepEqual(view.groups, []);
    assert.deepEqual(view.timers, []);
  });
});

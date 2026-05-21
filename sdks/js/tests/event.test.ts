import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  Event, String, Int, Bool, Group, Null, Any,
  SensitiveString, HashString, MarkSensitive,
  UserID, TenantID, FeatureFlag,
  RequestID, TraceID, SpanID, Plan, Currency, Amount, Country, Device, Platform, Retryable,
  userId, requestId, traceId,
  EventStateCreated, EventStateActive, EventStateFinished,
} from '../src/core/event.ts';

describe('Event', () => {
  it('creates event with default values', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    assert.equal(ev.event, 'test');
    assert.equal(ev.service, 'checkout');
    assert.equal(ev.kind, 'event');
    assert.equal(ev.state, EventStateCreated);
    assert.ok(ev.eventId);
    assert.ok(ev.timestamp);
  });

  it('transitions to active on enrich', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    assert.equal(ev.state, EventStateCreated);
    ev.enrich(String('key', 'value'));
    assert.equal(ev.state, EventStateActive);
    assert.equal(ev.attrs['key'], 'value');
  });

  it('transitions to finished on finish', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.finish('success');
    assert.equal(ev.state, EventStateFinished);
    assert.equal(ev.outcome, 'success');
    assert.ok(ev.durationMs >= 0);
  });

  it('records checkpoints', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.checkpoint('step1');
    ev.checkpoint('step2', { key: 'value' });
    assert.equal(ev.checkpoints.length, 2);
    assert.equal(ev.checkpoints[0].name, 'step1');
    assert.equal(ev.checkpoints[1].name, 'step2');
  });

  it('extracts error on finishError', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.finishError(new Error('boom'));
    assert.equal(ev.outcome, 'error');
    assert.ok(ev.error);
    assert.equal(ev.error!.type, 'Error');
    assert.equal(ev.error!.message, 'boom');
  });

  it('prevents mutation after emit', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.markEmitted();
    assert.throws(() => ev.enrich(String('key', 'value')), /event is closed/);
  });

  it('prevents double finish', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.finish('success');
    assert.throws(() => ev.finish('error'), /event already finished/);
  });

  it('sets params from constructor', () => {
    const ev = new Event({
      event: 'payment.completed',
      service: 'checkout',
      kind: 'http',
      method: 'POST',
      path: '/checkout',
      statusCode: 200,
      userId: 'u_123',
      tenantId: 't_456',
      custom: [String('custom.key', 'value')],
    }, 'checkout', 'development');

    assert.equal(ev.event, 'payment.completed');
    assert.equal(ev.kind, 'http');
    assert.equal(ev.method, 'POST');
    assert.equal(ev.path, '/checkout');
    assert.equal(ev.statusCode, 200);
    assert.equal(ev.attrs['user.id'], 'u_123');
    assert.equal(ev.attrs['tenant.id'], 't_456');
    assert.equal(ev.attrs['custom.key'], 'value');
  });

  it('append works like enrich', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.append(String('key', 'value'));
    assert.equal(ev.attrs['key'], 'value');
    assert.equal(ev.state, EventStateActive);
  });

  it('set/get/delete work', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.set('user.id', 'u123');
    assert.equal(ev.get('user.id'), 'u123');
    ev.delete('user.id');
    assert.equal(ev.get('user.id'), undefined);
  });

  it('merge works', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.merge({ 'user.id': 'u123', 'user.name': 'Alice' });
    assert.equal(ev.get('user.id'), 'u123');
    assert.equal(ev.get('user.name'), 'Alice');
  });

  it('getGroup returns matching attrs', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.set('user.id', 'u123');
    ev.set('user.name', 'Alice');
    ev.set('other', 'value');
    const group = ev.getGroup('user');
    assert.equal(group.id, 'u123');
    assert.equal(group.name, 'Alice');
    assert.equal(group.other, undefined);
  });

  it('tracks sensitive keys', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.enrich(SensitiveString('secret', 'value'));
    assert.ok(ev.sensitiveKeys.has('secret'));
    assert.equal(ev.attrs.secret, 'value'); // still stored, sanitized on emit
  });

  it('tracks hash keys', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.enrich(HashString('email', 'user@example.com'));
    assert.ok(ev.hashKeys.has('email'));
  });

  it('drop attrs are not stored', () => {
    const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
    ev.enrich({ key: 'temp', kind: 'string', value: 'val', drop: true });
    assert.equal(ev.attrs.temp, undefined);
    assert.ok(ev.droppedKeys.has('temp'));
  });
});

describe('Attr constructors', () => {
  it('creates string attr', () => {
    const a = String('key', 'value');
    assert.equal(a.key, 'key');
    assert.equal(a.kind, 'string');
    assert.equal(a.value, 'value');
  });

  it('creates int attr', () => {
    const a = Int('count', 42);
    assert.equal(a.key, 'count');
    assert.equal(a.kind, 'number');
    assert.equal(a.value, 42);
  });

  it('creates bool attr', () => {
    const a = Bool('active', true);
    assert.equal(a.kind, 'boolean');
    assert.equal(a.value, true);
  });

  it('creates group attr', () => {
    const g = Group('user', [String('name', 'alice')]);
    assert.equal(g.kind, 'group');
    assert.ok(Array.isArray(g.value));
  });

  it('creates semantic shortcuts', () => {
    assert.equal(UserID('u1').key, 'user.id');
    assert.equal(TenantID('t1').key, 'tenant.id');
    assert.equal(FeatureFlag('dark_mode', true).key, 'feature.dark_mode');
  });
});

describe('Canonical key parity', () => {
  it('RequestID uses request_id', () => {
    assert.equal(RequestID('r1').key, 'request_id');
    assert.equal(requestId('r1').key, 'request_id');
  });

  it('TraceID uses trace_id', () => {
    assert.equal(TraceID('t1').key, 'trace_id');
    assert.equal(traceId('t1').key, 'trace_id');
  });

  it('SpanID uses span_id', () => {
    assert.equal(SpanID('s1').key, 'span_id');
  });

  it('Plan uses customer.plan', () => {
    assert.equal(Plan('pro').key, 'customer.plan');
  });

  it('Currency uses payment.currency', () => {
    assert.equal(Currency('USD').key, 'payment.currency');
  });

  it('Amount uses payment.amount', () => {
    assert.equal(Amount(99.99).key, 'payment.amount');
  });

  it('Country uses geo.country', () => {
    assert.equal(Country('US').key, 'geo.country');
  });

  it('Device uses device.name', () => {
    assert.equal(Device('iPhone').key, 'device.name');
  });

  it('Platform uses device.platform', () => {
    assert.equal(Platform('ios').key, 'device.platform');
  });

  it('Retryable uses error.retryable', () => {
    assert.equal(Retryable(true).key, 'error.retryable');
  });
});

describe('camelCase aliases', () => {
  it('userId is alias for UserID', () => {
    assert.equal(userId, UserID);
    assert.equal(userId('u1').key, 'user.id');
  });

  it('requestId is alias for RequestID', () => {
    assert.equal(requestId, RequestID);
  });
});

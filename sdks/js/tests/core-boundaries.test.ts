import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  LevelDebug, LevelInfo, LevelNotice, LevelWarn, LevelError, LevelFatal,
  parseLevel, levelName,
} from '../src/core/level.ts';
import { LozaError, DuplicateEmitError, EventClosedError, EventAlreadyFinishedError, EventValidationError, extractError } from '../src/core/errors.ts';
import { Event, String, SensitiveString, HashString, resetClock, setClock } from '../src/core/event.ts';
import { sanitizeEvent } from '../src/core/sanitize.ts';
import { setPath, getPath, deletePath } from '../src/core/paths.ts';
import { storeEvent, getEvent, hasEvent, eventId, runWithEvent, requestIdFromContext, traceIdFromContext } from '../src/core/context.ts';
import { encodeJSON, encodePrettyJSON } from '../src/jsonenc/encoder.ts';
import { MetricsCollector, RenderPrometheus } from '../src/metrics.ts';
import { SecurityLimiter } from '../src/config/security.ts';

function makeEvent(): Event {
  return new Event({ event: 'core.test', service: 'svc', requestId: 'req', traceId: 'trace' }, 'svc', 'test');
}

describe('Core boundaries', () => {
  it('parses known and unknown levels and names invalid values safely', () => {
    assert.equal(parseLevel('DEBUG'), LevelDebug);
    assert.equal(parseLevel('info'), LevelInfo);
    assert.equal(parseLevel('notice'), LevelNotice);
    assert.equal(parseLevel('warn'), LevelWarn);
    assert.equal(parseLevel('error'), LevelError);
    assert.equal(parseLevel('fatal'), LevelFatal);
    assert.equal(parseLevel('unknown'), LevelInfo);
    assert.equal(levelName(LevelDebug), 'debug');
    assert.equal(levelName(99 as never), 'info');
  });

  it('constructs SDK errors and extracts Error and primitive values', () => {
    assert.equal(new LozaError('message').name, 'LozaError');
    assert.equal(new DuplicateEmitError().message, 'event already emitted');
    assert.equal(new EventClosedError().message, 'event is closed');
    assert.equal(new EventAlreadyFinishedError().message, 'event already finished');
    assert.equal(new EventValidationError('invalid').message, 'invalid');
    const error = extractError(new TypeError('bad'));
    assert.equal(error.type, 'TypeError');
    assert.equal(error.message, 'bad');
    assert.deepEqual(extractError(42), { type: 'Error', message: '42' });
    const unnamed = new Error('unnamed');
    unnamed.name = '';
    assert.equal(extractError(unnamed).type, 'Error');
  });
    runWithEvent(undefined as never, () => {
      assert.equal(getEvent(), undefined);
      assert.equal(hasEvent(), false);
      assert.equal(eventId(), '');
      assert.equal(requestIdFromContext(), '');
      assert.equal(traceIdFromContext(), '');
    });

  it('stores and restores async event context', () => {
    const event = makeEvent();
    assert.equal(hasEvent(), false);
    storeEvent(event);
    assert.equal(getEvent(), event);
    assert.equal(hasEvent(), true);
    assert.equal(eventId(), event.eventId);
    assert.equal(requestIdFromContext(), 'req');
    assert.equal(traceIdFromContext(), 'trace');
    assert.equal(runWithEvent(event, () => getEvent()), event);
  });

  it('sets, gets, and deletes nested paths across object and array boundaries', () => {
    const root: Record<string, unknown> = { existing: { value: 1 }, array: [1] };
    setPath(root, 'existing.next.value', 2);
    setPath(root, 'array.item.value', 3);
    setPath(root, 'new.value', 4);
    assert.equal(getPath(root, 'existing.next.value'), 2);
    assert.equal(getPath(root, 'array.item.value'), 3);
    assert.equal(getPath(root, 'missing.value'), undefined);
    assert.equal(getPath(root, 'existing'), root.existing);
    deletePath(root, 'existing.next.value');
    deletePath(root, 'missing.value');
    deletePath(root, 'existing');
    assert.equal(getPath(root, 'existing'), undefined);
    assert.equal(getPath({ value: 1 }, 'value.child'), undefined);
    deletePath({ value: 1 }, 'value.child');
  });
  it('sanitizes sensitive, hashed, and dropped attrs without mutating the event', () => {
    const event = makeEvent();
    event.enrich(
      SensitiveString('secret', 'value'),
      HashString('token', 'value'),
      String('drop', 'value'),
      SensitiveString('missing', 'value'),
      { key: 'number', kind: 'number', value: 123, hashValue: true },
    );
    event.enrich({ key: 'drop', kind: 'string', value: 'value', drop: true });
    const sanitized = sanitizeEvent(event);
    assert.equal(sanitized.attrs.secret, '[REDACTED]');
    assert.match(sanitized.attrs.token, /^[a-f0-9]{64}$/);
    assert.equal('drop' in sanitized.attrs, false);
    assert.equal(sanitized.attrs.missing, '[REDACTED]');
    assert.equal(sanitized.attrs.number, 123);
    assert.equal(event.attrs.secret, 'value');
    assert.equal(event.attrs.token, 'value');
  });

  it('encodes compact and pretty JSON and collects Prometheus metrics', () => {
    const payload = { key: 'value', count: 2 };
    assert.equal(encodeJSON(payload), '{"key":"value","count":2}');
    assert.match(encodePrettyJSON(payload), /\n  "key": "value"/);
    const metrics = new MetricsCollector();
    metrics.inc('events');
    metrics.inc('events', 2);
    metrics.setGauge('queue', 4);
    assert.deepEqual(metrics.snapshot(), { counters: { events: 3 }, gauges: { queue: 4 } });
    assert.equal(RenderPrometheus(metrics), '# TYPE loza_sdk_events counter\nloza_sdk_events 3\n# TYPE loza_sdk_queue gauge\nloza_sdk_queue 4\n');
    assert.equal(metrics.renderPrometheus('custom'), '# TYPE custom_events counter\ncustom_events 3\n# TYPE custom_queue gauge\ncustom_queue 4\n');
  });

  it('enforces security limits and returns defensive config snapshots', () => {
    const limiter = new SecurityLimiter({ maxEventBytes: 3, maxAttrCount: 2, maxFieldBytes: 3 });
    assert.equal(limiter.shouldDrop('1234', 0), true);
    assert.equal(limiter.shouldDrop('1', 3), true);
    assert.equal(limiter.shouldDrop('1', 1), false);
    assert.equal(limiter.isFieldOversized('1234'), true);
    assert.equal(limiter.isFieldOversized('123'), false);
    const config = limiter.getConfig();
    config.maxEventBytes = 100;
    assert.equal(limiter.getConfig().maxEventBytes, 3);
    assert.equal(new SecurityLimiter({ dropOversizedEvents: false }).shouldDrop('long', 1000), false);
  });

  it('supports deterministic event clocks and mutable state transitions', () => {
    const previous = setClock(() => 1000);
    try {
      const event = makeEvent();
      assert.equal(event.startedAt, 1000);
      event.checkpoint('start');
      event.finish('success');
      assert.equal(event.durationMs, 0);
      assert.equal(event.markEmitted(), true);
      assert.equal(event.markEmitted(), false);
      event.markEmittedDone();
      assert.equal(event.getEventState(), 'emitted');
      event.markDeliveryFailed();
      assert.equal(event.getEventState(), 'delivery_failed');
      event.markFailedValidation();
      assert.equal(event.getEventState(), 'failed_validation');
    } finally {
      resetClock();
      previous();
    }
  });
});

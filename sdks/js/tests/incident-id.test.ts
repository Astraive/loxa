import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Event, MemorySink, createLoxa, IncidentID } from '../src/index.ts';

describe('IncidentID', () => {
  it('sets incidentId from params', () => {
    const ev = new Event({
      event: 'test.incident',
      service: 'test-svc',
      incidentId: 'inc-test-001',
    }, 'test-svc', 'development');
    assert.equal(ev.incidentId, 'inc-test-001');
  });

  it('emits incident_id in JSON output', async () => {
    const sink = new MemorySink();
    const logger = createLoxa({
      service: 'test-svc',
      environment: 'test',
      sink,
    });

    const ctx = logger.startEvent({
      event: 'test.incident',
      kind: 'log',
      incidentId: 'inc-emit-test',
    });

    logger.finish(ctx, 'success');
    await logger.emit(ctx);

    const events = sink.getEvents();
    assert.ok(events.length > 0);
    const raw = events[0];
    const payload = typeof raw === 'string' ? JSON.parse(raw) : raw;
    assert.equal(payload.incident_id, 'inc-emit-test');
  });

  it('omits incident_id when not set', async () => {
    const sink = new MemorySink();
    const logger = createLoxa({
      service: 'test-svc',
      environment: 'test',
      sink,
    });

    const ctx = logger.startEvent({ event: 'test.no-incident', kind: 'log' });
    logger.finish(ctx, 'success');
    await logger.emit(ctx);

    const events = sink.getEvents();
    assert.ok(events.length > 0);
    const raw = events[0];
    const payload = typeof raw === 'string' ? JSON.parse(raw) : raw;
    assert.equal(payload.incident_id, undefined);
  });

  it('IncidentID attr constructor uses incident_id key', () => {
    const attr = IncidentID('inc-attr-test');
    assert.equal(attr.key, 'incident_id');
    assert.equal(attr.value, 'inc-attr-test');
  });
});

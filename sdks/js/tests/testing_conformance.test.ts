import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers testing and conformance helpers', async () => {
  const events = await loza.capture(async (logger) => {
    const event = logger.startEvent({ event: 'capture.event' });
    logger.append(event, loza.string('family', 'testkit'));
    logger.checkpoint(event, 'captured');
    logger.finish(event, 'success');
    await logger.emit(event);
  });
  assert.equal(events.length, 1);
  loza.assertEvent(events[0], 'attrs.family', 'testkit');
  loza.assertHasCheckpoint(events[0], 'captured');
  loza.snapshotEvent(events[0], JSON.stringify(JSON.parse(events[0])));

  const mock = new loza.MockSink();
  await mock.write('{"ok":true}');
  assert.equal(mock.getEvents().length, 1);
  const clock = new loza.FakeClock(1000);
  clock.advance(50);
  assert.equal(clock.now(), 1050);
  loza.setIdGenerator(() => 'evt_fixed');
  const deterministicLogger = loza.createLoza({ service: 'idgen', sink: new loza.MemorySink() });
  const deterministic = deterministicLogger.startEvent({ event: 'deterministic.id' });
  assert.equal(deterministic.eventId, 'evt_fixed');
  loza.resetForTest();
});

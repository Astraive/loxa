import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers testing and conformance helpers', async () => {
  const events = await loxa.capture(async (logger) => {
    const event = logger.startEvent({ event: 'capture.event' });
    logger.append(event, loxa.string('family', 'testkit'));
    logger.checkpoint(event, 'captured');
    logger.finish(event, 'success');
    await logger.emit(event);
  });
  assert.equal(events.length, 1);
  loxa.assertEvent(events[0], 'attrs.family', 'testkit');
  loxa.assertHasCheckpoint(events[0], 'captured');
  loxa.snapshotEvent(events[0], JSON.stringify(JSON.parse(events[0])));

  const mock = new loxa.MockSink();
  await mock.write('{"ok":true}');
  assert.equal(mock.getEvents().length, 1);
  const clock = new loxa.FakeClock(1000);
  clock.advance(50);
  assert.equal(clock.now(), 1050);
  loxa.setIdGenerator(() => 'evt_fixed');
  const deterministicLogger = loxa.createLoxa({ service: 'idgen', sink: new loxa.MemorySink() });
  const deterministic = deterministicLogger.startEvent({ event: 'deterministic.id' });
  assert.equal(deterministic.eventId, 'evt_fixed');
  loxa.resetForTest();
});

import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers sink, queue, flush, and shutdown helpers', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(loxa.test('catalog').withSink(sink));
  const logger = loxa.createLoxa({ service: 'catalog', sink });

  logger.info('sink.event', { family: 'sink' });
  await logger.flush();
  assert.ok(sink.getLength() >= 1);

  const batchSink = loxa.httpBatchSink({ endpoint: 'http://127.0.0.1:65535/events', batchSize: 10, flushIntervalMs: 1000 });
  batchSink.pause();
  batchSink.resume();
  assert.equal(batchSink.queueSize(), 0);

  assert.ok(loxa.stdoutSink());
  assert.ok(loxa.noopSink());
  assert.ok(loxa.multiSink(sink, loxa.noopSink()));
  assert.ok(loxa.otlpSink('http://127.0.0.1:4318'));

  await loxa.flush();
  await logger.shutdown();
  await loxa.shutdown();
});

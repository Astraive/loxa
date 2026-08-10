import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers sink, queue, flush, and shutdown helpers', async () => {
  const sink = new loza.MemorySink();
  loza.configure(loza.test('catalog').withSink(sink));
  const logger = loza.createLoza({ service: 'catalog', sink });

  logger.info('sink.event', { family: 'sink' });
  await logger.flush();
  assert.ok(sink.getLength() >= 1);

  const batchSink = loza.httpBatchSink({ endpoint: 'http://127.0.0.1:65535/events', batchSize: 10, flushIntervalMs: 1000 });
  batchSink.pause();
  batchSink.resume();
  assert.equal(batchSink.queueSize(), 0);

  assert.ok(loza.stdoutSink());
  assert.ok(loza.noopSink());
  assert.ok(loza.multiSink(sink, loza.noopSink()));
  assert.ok(loza.otlpSink('http://127.0.0.1:4318'));

  await loza.flush();
  await logger.shutdown();
  await loza.shutdown();
});

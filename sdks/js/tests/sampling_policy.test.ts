import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers sampling and policy helpers', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(
    loxa.test('catalog')
      .withSink(sink)
      .withSampler(loxa.sampleByEvent('verification.sampled'))
      .withRedactor(loxa.redactKeys('password')),
  );
  const logger = loxa.createLoxa({
    ...loxa.test('catalog').withSink(sink).withSampler(loxa.sampleByEvent('verification.sampled')).withRedactor(loxa.redactKeys('password')),
  });
  const ctx = logger.startEvent({ event: 'verification.sampled' });
  logger.append(ctx, loxa.string('password', 'secret123'));
  logger.finish(ctx, 'success');
  const encoded = await logger.emit(ctx);
  assert.match(encoded!, /REDACTED/);
  assert.ok(loxa.sampleByOutcome('error'));
  assert.ok(loxa.allowFields('allowed'));
  assert.ok(loxa.blockFields('blocked'));
});

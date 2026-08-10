import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers sampling and policy helpers', async () => {
  const sink = new loza.MemorySink();
  loza.configure(
    loza.test('catalog')
      .withSink(sink)
      .withSampler(loza.sampleByEvent('verification.sampled'))
      .withRedactor(loza.redactKeys('password')),
  );
  const logger = loza.createLoza({
    ...loza.test('catalog').withSink(sink).withSampler(loza.sampleByEvent('verification.sampled')).withRedactor(loza.redactKeys('password')),
  });
  const ctx = logger.startEvent({ event: 'verification.sampled' });
  logger.append(ctx, loza.string('password', 'secret123'));
  logger.finish(ctx, 'success');
  const encoded = await logger.emit(ctx);
  assert.match(encoded!, /REDACTED/);
  assert.ok(loza.sampleByOutcome('error'));
  assert.ok(loza.allowFields('allowed'));
  assert.ok(loza.blockFields('blocked'));
});

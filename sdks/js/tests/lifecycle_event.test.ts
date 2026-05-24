import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers lifecycle event mutation and outcome helpers', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(loxa.test('catalog').withSink(sink));
  const logger = loxa.createLoxa({ service: 'catalog', sink });
  const ctx = logger.startEvent({ event: 'checkout.request', kind: 'http', route: '/checkout' });

  logger.append(ctx, loxa.userId('u_123'), loxa.tenantId('t_123'));
  logger.set(ctx, 'payment.provider', 'stripe');
  logger.merge(ctx, { 'cart.items': 3 });
  assert.equal(logger.get(ctx, 'payment.provider'), 'stripe');
  assert.equal(logger.getGroup(ctx, 'cart').items, 3);
  logger.delete(ctx, 'payment.provider');
  logger.checkpoint(ctx, 'validated');

  const cloned = loxa.cloneEvent(ctx);
  assert.equal(cloned.eventId, ctx.eventId);
  const linked = loxa.linkEvent(ctx, 'checkout.child', loxa.string('link.kind', 'child'));
  assert.equal(linked.event, 'checkout.child');
  assert.equal(linked.traceId, ctx.traceId);

  logger.finish(ctx, 'success');
  const encoded = await logger.emit(ctx);
  const payload = JSON.parse(encoded!);
  assert.equal(payload.outcome, 'success');

  const dropped = logger.startEvent({ event: 'drop.event' });
  await loxa.drop(dropped, 'capacity');
  const cancelled = logger.startEvent({ event: 'cancel.event' });
  await loxa.cancel(cancelled, 'user_cancelled');
  const abandoned = logger.startEvent({ event: 'abandon.event' });
  await loxa.abandon(abandoned, 'orphaned');
  const retried = logger.startEvent({ event: 'retry.event' });
  await loxa.retry(retried, loxa.int('attempt', 2));
  const partial = logger.startEvent({ event: 'partial.event' });
  await loxa.partial(partial, loxa.string('reason', 'timeout'));

  await loxa.runEvent({ event: 'wrapped.event' }, async (event) => {
    loxa.append(event, loxa.string('wrapped', 'true'));
  });
  await logger.flush();
  assert.ok(sink.getLength() >= 2);
});

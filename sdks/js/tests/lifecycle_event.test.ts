import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers lifecycle event mutation and outcome helpers', async () => {
  const sink = new loza.MemorySink();
  loza.configure(loza.test('catalog').withSink(sink));
  const logger = loza.createLoza({ service: 'catalog', sink });
  const ctx = logger.startEvent({ event: 'checkout.request', kind: 'http', route: '/checkout' });

  logger.append(ctx, loza.userId('u_123'), loza.tenantId('t_123'));
  logger.set(ctx, 'payment.provider', 'stripe');
  logger.merge(ctx, { 'cart.items': 3 });
  assert.equal(logger.get(ctx, 'payment.provider'), 'stripe');
  assert.equal(logger.getGroup(ctx, 'cart').items, 3);
  logger.delete(ctx, 'payment.provider');
  logger.checkpoint(ctx, 'validated');

  const cloned = loza.cloneEvent(ctx);
  assert.equal(cloned.eventId, ctx.eventId);
  const linked = loza.linkEvent(ctx, 'checkout.child', loza.string('link.kind', 'child'));
  assert.equal(linked.event, 'checkout.child');
  assert.equal(linked.traceId, ctx.traceId);

  logger.finish(ctx, 'success');
  const encoded = await logger.emit(ctx);
  const payload = JSON.parse(encoded!);
  assert.equal(payload.outcome, 'success');

  const dropped = logger.startEvent({ event: 'drop.event' });
  await loza.drop(dropped, 'capacity');
  const cancelled = logger.startEvent({ event: 'cancel.event' });
  await loza.cancel(cancelled, 'user_cancelled');
  const abandoned = logger.startEvent({ event: 'abandon.event' });
  await loza.abandon(abandoned, 'orphaned');
  const retried = logger.startEvent({ event: 'retry.event' });
  await loza.retry(retried, loza.int('attempt', 2));
  const partial = logger.startEvent({ event: 'partial.event' });
  await loza.partial(partial, loza.string('reason', 'timeout'));

  await loza.runEvent({ event: 'wrapped.event' }, async (event) => {
    loza.append(event, loza.string('wrapped', 'true'));
  });
  await logger.flush();
  assert.ok(sink.getLength() >= 2);
});

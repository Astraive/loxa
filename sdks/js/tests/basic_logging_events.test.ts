import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers basic logging and event facades', async () => {
  const sink = new loza.MemorySink();
  loza.configure(loza.production('catalog').withSink(sink));

  await loza.notice('notice event', loza.string('family', 'logs'));
  await loza.track('checkout.page_view', loza.string('page', '/checkout'));
  await loza.audit('user.login', loza.userId('u_123'));
  await loza.security('auth.failure', loza.errorCode('AUTH_BAD_PASSWORD'));
  await loza.metric('latency', 42, loza.string('unit', 'ms'));
  await loza.count('requests', 4);
  await loza.gauge('cpu', 0.72);
  await loza.histogram('payload.bytes', 512);
  await loza.breadcrumb('nav.click', loza.string('button', 'submit'));
  await loza.flush();

  assert.ok(sink.getLength() >= 9);
});

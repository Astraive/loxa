import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers basic logging and event facades', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(loxa.production('catalog').withSink(sink));

  await loxa.notice('notice event', loxa.string('family', 'logs'));
  await loxa.track('checkout.page_view', loxa.string('page', '/checkout'));
  await loxa.audit('user.login', loxa.userId('u_123'));
  await loxa.security('auth.failure', loxa.errorCode('AUTH_BAD_PASSWORD'));
  await loxa.metric('latency', 42, loxa.string('unit', 'ms'));
  await loxa.count('requests', 4);
  await loxa.gauge('cpu', 0.72);
  await loxa.histogram('payload.bytes', 512);
  await loxa.breadcrumb('nav.click', loxa.string('button', 'submit'));
  await loxa.flush();

  assert.ok(sink.getLength() >= 9);
});

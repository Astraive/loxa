import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers typed attribute helpers', () => {
  assert.equal(loxa.string('service.name', 'checkout').key, 'service.name');
  assert.equal(loxa.int('attempt', 2).key, 'attempt');
  assert.equal(loxa.float('ratio', 0.5).key, 'ratio');
  assert.equal(loxa.bool('cache.hit', true).key, 'cache.hit');
  assert.equal(loxa.json('payload', { ok: true }).key, 'payload');
  assert.equal(loxa.money('cart.total', 4999, 'USD').key, 'cart.total');
  assert.equal(loxa.percent('cpu', 87.5).key, 'cpu');
  assert.equal(loxa.bytes('payload.size', 2048).key, 'payload.size');
  assert.equal(loxa.httpStatus(200).key, 'status_code');
  assert.equal(loxa.bucket('user.tier', 'pro').key, 'user.tier');
  assert.equal(loxa.masked('card', '4111111111111111').key, 'card');
  assert.equal(loxa.url('url', 'https://example.com').key, 'url');
  assert.equal(loxa.emailHash('email.hash', 'User@Example.com').key, 'email.hash');
  assert.equal(loxa.ipHash('ip.hash', '127.0.0.1').key, 'ip.hash');
});

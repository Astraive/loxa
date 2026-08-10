import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers typed attribute helpers', () => {
  assert.equal(loza.string('service.name', 'checkout').key, 'service.name');
  assert.equal(loza.int('attempt', 2).key, 'attempt');
  assert.equal(loza.float('ratio', 0.5).key, 'ratio');
  assert.equal(loza.bool('cache.hit', true).key, 'cache.hit');
  assert.equal(loza.json('payload', { ok: true }).key, 'payload');
  assert.equal(loza.money('cart.total', 4999, 'USD').key, 'cart.total');
  assert.equal(loza.percent('cpu', 87.5).key, 'cpu');
  assert.equal(loza.bytes('payload.size', 2048).key, 'payload.size');
  assert.equal(loza.httpStatus(200).key, 'status_code');
  assert.equal(loza.bucket('user.tier', 'pro').key, 'user.tier');
  assert.equal(loza.masked('card', '4111111111111111').key, 'card');
  assert.equal(loza.url('url', 'https://example.com').key, 'url');
  assert.equal(loza.emailHash('email.hash', 'User@Example.com').key, 'email.hash');
  assert.equal(loza.ipHash('ip.hash', '127.0.0.1').key, 'ip.hash');
});

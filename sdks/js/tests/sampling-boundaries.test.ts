import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  sampleRandom, sampleRate, shouldSample, sampleSlowRequests,
  sampleUsers, sampleTenants, sampleFeatureFlag, sampleRateLimited,
  sampleByHeader, sampleByEvent, sampleByOutcome, allowFields, blockFields,
  maxAttrLength, maxEventBytes, maxAttrs, cardinalityPolicy,
} from '../src/sampling/sampler.ts';
import { Event } from '../src/core/event.ts';

function makeEvent(): Event {
  return new Event({ event: 'checkout.completed', service: 'checkout' }, 'checkout', 'development');
}

describe('Sampler boundaries', () => {
  it('supports random and rate aliases with deterministic decisions', () => {
    const originalRandom = Math.random;
    Math.random = () => 0.25;
    try {
      assert.equal(sampleRandom(0.5)(makeEvent()), true);
      assert.equal(sampleRate(0.1)(makeEvent()), false);
    } finally {
      Math.random = originalRandom;
    }
    const event = makeEvent();
    assert.equal(shouldSample(() => true, event), true);
  });

  it('matches event properties and fields', () => {
    const event = makeEvent();
    Object.assign(event, {
      durationMs: 120,
      statusCode: 201,
      route: '/checkout',
      path: '/checkout/123',
      outcome: 'success',
      attrs: {
        'user.id': 'u-1',
        'tenant.id': 't-1',
        'feature.beta': true,
        'http.header.x-request-id': 'req-1',
        'custom': 'present',
      },
    });
    assert.equal(sampleSlowRequests(100)(event), true);
    assert.equal(sampleUsers('u-1')(event), true);
    assert.equal(sampleTenants('t-1')(event), true);
    assert.equal(sampleFeatureFlag('beta', true)(event), true);
    assert.equal(sampleByHeader('X_Request_Id', 'req-1')(event), true);
    assert.equal(sampleByHeader('x-request-id')(event), true);
    assert.equal(sampleByEvent('checkout.completed')(event), true);
    assert.equal(sampleByOutcome('success')(event), true);
    assert.equal(allowFields('custom', 'durationMs')(event), true);
    assert.equal(blockFields('missing')(event), true);
    assert.equal(blockFields('custom')(event), false);
  });

  it('supports alternate header keys and empty header values', () => {
    const event = makeEvent();
    Object.assign(event, { attrs: { 'http.headers.x-forwarded-for': '10.0.0.1' } });
    assert.equal(sampleByHeader('X-Forwarded-For', '10.0.0.1')(event), true);
    assert.equal(sampleByHeader('X-Forwarded-For', '')(event), true);
    assert.equal(sampleByHeader('Missing', '')(event), false);
  });

  it('refills rate-limited tokens and enforces an empty bucket', () => {
    const originalNow = Date.now;
    let now = 1000;
    Date.now = () => now;
    try {
      const limited = sampleRateLimited(1, 1000);
      const event = makeEvent();
      assert.equal(limited(event), true);
      assert.equal(limited(event), false);
      now += 1000;
      assert.equal(limited(event), true);
    } finally {
      Date.now = originalNow;
    }
  });

  it('returns policy option objects without mutation', () => {
    const policy = { key: 'value' };
    assert.deepEqual(maxAttrLength(5), { maxAttrLength: 5 });
    assert.deepEqual(maxEventBytes(20), { maxEventBytes: 20 });
    assert.deepEqual(maxAttrs(3), { maxAttrs: 3 });
    assert.deepEqual(cardinalityPolicy(policy), policy);
  });
});

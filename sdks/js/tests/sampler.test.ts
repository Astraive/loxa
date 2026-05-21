import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  sampleAll, sampleNone, sampleRandom, sampleErrors,
  sampleStatusCodes, sampleRoutes, anySampler, allSampler, notSampler,
} from '../src/sampling/sampler.ts';
import { Event } from '../src/core/event.ts';

function makeEvent(overrides: Partial<Event> = {}): Event {
  const ev = new Event({ event: 'test', service: 'checkout' }, 'checkout', 'development');
  Object.assign(ev, overrides);
  return ev;
}

describe('Sampler', () => {
  it('sampleAll keeps everything', () => {
    assert.equal(sampleAll()(makeEvent()), true);
  });

  it('sampleNone drops everything', () => {
    assert.equal(sampleNone()(makeEvent()), false);
  });

  it('sampleErrors keeps error events', () => {
    const s = sampleErrors();
    assert.equal(s(makeEvent()), false);
    assert.equal(s(makeEvent({ outcome: 'error' } as any)), true);
  });

  it('sampleStatusCodes matches codes', () => {
    const s = sampleStatusCodes(200, 201);
    assert.equal(s(makeEvent({ statusCode: 200 } as any)), true);
    assert.equal(s(makeEvent({ statusCode: 500 } as any)), false);
  });

  it('sampleRoutes matches routes', () => {
    const s = sampleRoutes('/api/users');
    assert.equal(s(makeEvent({ route: '/api/users' } as any)), true);
    assert.equal(s(makeEvent({ route: '/api/orders' } as any)), false);
  });

  it('anySampler is OR', () => {
    const s = anySampler(sampleNone(), sampleAll());
    assert.equal(s(makeEvent()), true);
  });

  it('allSampler is AND', () => {
    const s = allSampler(sampleAll(), sampleNone());
    assert.equal(s(makeEvent()), false);
  });

  it('notSampler inverts', () => {
    const s = notSampler(sampleAll());
    assert.equal(s(makeEvent()), false);
  });
});

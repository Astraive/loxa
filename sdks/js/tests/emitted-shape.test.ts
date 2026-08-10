import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createLoza, MemorySink } from '../src/index.ts';

function loadFixture() {
  const paths = [
    resolve(import.meta.dirname, '../../../spec/fixtures/emitted-shape/structured_http_success.json'),
    resolve(import.meta.dirname, '../../../spec/examples/golden/emitted-shape/structured_http_success.json'),
  ];
  for (const p of paths) {
    try {
      return JSON.parse(readFileSync(p, 'utf-8'));
    } catch { /* try next */ }
  }
  throw new Error('fixture not found');
}

function lookupPath(obj: any, path: string): any {
  let current = obj;
  for (const seg of path.split('.')) {
    if (current == null || typeof current !== 'object') return undefined;
    current = current[seg];
  }
  return current;
}

describe('Emitted shape matches cortex contract', () => {
  it('produces the expected JSON shape', async () => {
    const fixture = loadFixture();
    const sink = new MemorySink();
    const logger = createLoza({
      service: fixture.params.service,
      environment: 'test',
      sink,
    });

    const ctx = logger.startEvent({
      event: fixture.params.event,
      kind: fixture.params.kind,
      service: fixture.params.service,
      method: fixture.params.method,
      path: fixture.params.path,
      route: fixture.params.route,
      statusCode: fixture.params.status_code,
    });

    for (const [key, value] of Object.entries(fixture.attrs)) {
      logger.enrich(ctx, { key, kind: 'any', value });
    }

    logger.finish(ctx, fixture.finish.outcome);
    await logger.emit(ctx);

    const events = sink.getEvents();
    assert.ok(events.length > 0, 'expected at least one emitted event');
    const raw = events[0];
    const payload = typeof raw === 'string' ? JSON.parse(raw) : raw;

    // Check present fields
    for (const path of fixture.expected.present) {
      const val = lookupPath(payload, path);
      assert.notStrictEqual(val, undefined, `expected ${path} to be present`);
    }

    // Check equal fields
    for (const [path, want] of Object.entries(fixture.expected.equals)) {
      const got = lookupPath(payload, path);
      assert.notStrictEqual(got, undefined, `expected ${path} to be present`);
      if (typeof want === 'number') {
        assert.ok(
          Number(got) === want || String(got) === String(want),
          `expected ${path}=${want}, got ${got}`
        );
      } else {
        assert.deepStrictEqual(got, want, `expected ${path}=${JSON.stringify(want)}, got ${JSON.stringify(got)}`);
      }
    }
  });
});

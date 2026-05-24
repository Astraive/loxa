import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {
  LOXA_SPEC_VERSION, LOXA_EVENT_VERSION, ALLOWED_KINDS, ALLOWED_LEVELS,
  buildIngestEnvelope, isCanonical,
} from '../src/generated/spec-contract.ts';
import { Logger } from '../src/core/logger.ts';
import { MemorySink } from '../src/sinks/standard-sinks.ts';
import { String as AttrString } from '../src/core/event.ts';

describe('Spec Conformance', () => {
  it('spec constants match expected values', () => {
    assert.equal(LOXA_SPEC_VERSION, 'v1');
    assert.equal(LOXA_EVENT_VERSION, 'v1');
  });

  it('allowed kinds match spec', () => {
    const expected = ['event', 'http', 'job', 'queue', 'cli', 'cron', 'log', 'checkpoint'];
    for (const kind of expected) {
      assert.ok(ALLOWED_KINDS.has(kind), `missing kind: ${kind}`);
    }
  });

  it('allowed levels match spec', () => {
    const expected = ['debug', 'info', 'warn', 'error', 'fatal'];
    for (const level of expected) {
      assert.ok(ALLOWED_LEVELS.has(level), `missing level: ${level}`);
    }
  });

  it('canonical fields include core fields', () => {
    const coreFields = ['event_id', 'timestamp', 'service', 'event', 'kind', 'level', 'outcome'];
    for (const field of coreFields) {
      assert.ok(isCanonical(field), `missing canonical field: ${field}`);
    }
  });

  it('buildIngestEnvelope creates valid envelope', () => {
    const envelope = buildIngestEnvelope('loxa-js', '0.0.2', 'checkout', [
      { event_id: 'evt_1', event: 'test' },
    ]);
    assert.equal(envelope.api_version, 'v1');
    assert.equal(envelope.source.sdk, 'loxa-js');
    assert.equal(envelope.source.version, '0.0.2');
    assert.equal(envelope.source.service, 'checkout');
    assert.equal(envelope.events.length, 1);
  });

  it('SDK emits spec-compliant event', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({ service: 'checkout', sink });
    const ctx = loxa.startEvent({ event: 'payment.completed' });
    loxa.enrich(ctx, AttrString('currency', 'USD'));
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);

    const payload = JSON.parse(sink.getEvents()[0]);
    assert.equal(payload.schema_version, 'v1');
    assert.equal(payload.event_version, 'v1');
    assert.equal(payload.service, 'checkout');
    assert.equal(payload.event, 'payment.completed');
    assert.equal(payload.kind, 'event');
    assert.ok(ALLOWED_KINDS.has(payload.kind));
    assert.ok(ALLOWED_LEVELS.has(payload.level));
  });

  it('public API source matches stable superset manifest', () => {
    const manifestPath = path.resolve('..', '..', 'spec', 'docs', 'sdk-parity-manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    const source = fs.readFileSync(path.resolve('src', 'index.ts'), 'utf8');
    const missing: string[] = [];

    for (const [key, values] of Object.entries(manifest)) {
      if (key === 'excluded_from_sdk' || key === 'sdks' || !Array.isArray(values)) continue;
      for (const value of values) {
        if (!source.includes(String(value))) missing.push(String(value));
      }
    }

    assert.deepEqual(missing, []);
  });
});

import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { createLoxa, HTTPBatchSink, MemorySink, CollectorClient, String as AttrString, Int as AttrInt } from '../src/index.ts';

const COLLECTOR_URL = process.env.LOXA_TEST_COLLECTOR_URL ?? 'http://127.0.0.1:9090';

describe('E2E: loxa-js → loxa-collector', () => {
  it('collector health check', async () => {
    const client = new CollectorClient({ url: COLLECTOR_URL });
    const healthy = await client.health();
    assert.equal(healthy, true, 'collector should be healthy');
  });

  it('collector readiness check', async () => {
    const client = new CollectorClient({ url: COLLECTOR_URL });
    const ready = await client.ready();
    assert.equal(ready, true, 'collector should be ready');
  });

  it('collector version endpoint', async () => {
    const client = new CollectorClient({ url: COLLECTOR_URL });
    const version = await client.version();
    assert.ok(version.version, 'should have version');
    assert.equal(version.ingest_api_version, 'v1', 'api version should be v1');
    console.log('  collector version:', version);
  });

  it('CollectorClient.sendBatch sends events to collector', async () => {
    const client = new CollectorClient({ url: COLLECTOR_URL });

    const events = [{
      schema_version: 'v1',
      event_version: 'v1',
      event_id: `e2e-batch-${Date.now()}`,
      timestamp: new Date().toISOString(),
      service: 'loxa-js-e2e',
      event: 'test.batch_send',
      kind: 'event',
      level: 'info',
      outcome: 'success',
    }];

    const response = await client.sendBatch(events);
    console.log('  collector response:', JSON.stringify(response));
    assert.equal(response.status, 'accepted', `expected accepted, got ${response.status}`);
    assert.ok(response.accepted >= 1, 'at least 1 event accepted');
  });

  it('HTTPBatchSink delivers events end-to-end', async () => {
    const acks: any[] = [];
    const sink = new HTTPBatchSink({
      endpoint: `${COLLECTOR_URL}/events`,
      batchSize: 1,        // flush immediately
      flushIntervalMs: 100,
      retries: 2,
      statsHandler: {
        onCollectorAck(data) {
          acks.push(data);
        },
      },
    });

    const loxa = createLoxa({
      service: 'loxa-js-e2e',
      sink,
    });

    // Create and emit an event
    const ctx = loxa.startEvent({
      event: 'e2e.http_batch',
      kind: 'event',
    });
    loxa.enrich(ctx,
      AttrString('test.id', `e2e-${Date.now()}`),
      AttrInt('test.run', 1),
    );
    loxa.finish(ctx, 'success');
    const encoded = await loxa.emit(ctx);

    assert.ok(encoded, 'should return encoded JSON');
    console.log('  emitted event:', encoded!.substring(0, 120) + '...');

    // Wait for async flush
    await new Promise(resolve => setTimeout(resolve, 500));

    // Verify ack was received
    assert.ok(acks.length > 0, 'should have received collector ack');
    console.log('  collector ack:', JSON.stringify(acks[0]));

    // Verify last response
    const lastResp = sink.lastCollectorResponse;
    assert.ok(lastResp, 'should have last collector response');
    assert.equal(lastResp!.status, 'accepted', 'response status should be accepted');

    await sink.flush();
  });

  it('HTTPBatchSink handles gzip compression', async () => {
    const sink = new HTTPBatchSink({
      endpoint: `${COLLECTOR_URL}/events`,
      enableCompression: true,
      batchSize: 1,
      flushIntervalMs: 100,
    });

    const loxa = createLoxa({ service: 'loxa-js-e2e-gzip', sink });
    const ctx = loxa.startEvent({ event: 'e2e.gzip' });
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);

    await new Promise(resolve => setTimeout(resolve, 500));

    const lastResp = sink.lastCollectorResponse;
    assert.ok(lastResp, 'should have response');
    assert.equal(lastResp!.status, 'accepted', 'gzip request should be accepted');
    console.log('  gzip response:', JSON.stringify(lastResp));
  });

  it('HTTPBatchSink handles NDJSON mode', async () => {
    const sink = new HTTPBatchSink({
      endpoint: `${COLLECTOR_URL}/events`,
      ndjson: true,
      batchSize: 1,
      flushIntervalMs: 100,
    });

    const loxa = createLoxa({ service: 'loxa-js-e2e-ndjson', sink });
    const ctx = loxa.startEvent({ event: 'e2e.ndjson' });
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);

    await new Promise(resolve => setTimeout(resolve, 500));

    const lastResp = sink.lastCollectorResponse;
    assert.ok(lastResp, 'should have response');
    assert.equal(lastResp!.status, 'accepted', 'ndjson request should be accepted');
    console.log('  ndjson response:', JSON.stringify(lastResp));
  });

  it('multiple events in single batch', async () => {
    const acks: any[] = [];
    const sink = new HTTPBatchSink({
      endpoint: `${COLLECTOR_URL}/events`,
      batchSize: 10,       // batch up to 10
      flushIntervalMs: 500,
      statsHandler: {
        onCollectorAck(data) { acks.push(data); },
      },
    });

    const loxa = createLoxa({ service: 'loxa-js-e2e-batch', sink });

    // Emit 5 events quickly
    for (let i = 0; i < 5; i++) {
      const ctx = loxa.startEvent({ event: `e2e.batch.${i}` });
      loxa.enrich(ctx, AttrInt('index', i));
      loxa.finish(ctx, 'success');
      await loxa.emit(ctx);
    }

    // Wait for flush
    await new Promise(resolve => setTimeout(resolve, 1000));
    await sink.flush();

    // All 5 should be in one batch
    let lastResp = sink.lastCollectorResponse;
    if (!lastResp) {
      await new Promise(resolve => setTimeout(resolve, 300));
      lastResp = sink.lastCollectorResponse;
    }
    assert.ok(lastResp, 'should have response');
    assert.ok(lastResp!.accepted >= 5, `expected >=5 accepted, got ${lastResp!.accepted}`);
    console.log('  batch response:', JSON.stringify(lastResp));
  });
});

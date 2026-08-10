import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers collector and cortex client families', async () => {
  const collectorServer = http.createServer((req, res) => {
    const url = new URL(req.url || '/', 'http://localhost');
    const path = url.pathname;
    const method = req.method || 'GET';
    const writeJson = (status: number, obj: any) => {
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (path === '/health' || path === '/ready') return writeJson(200, { status: 'ok' });
    if (path === '/version') return writeJson(200, { version: '0.2.0', ingest_api_version: 'v1', schema_version: 'v1', event_version: 'v1' });
    if (path === '/validate') return writeJson(200, { valid: true });
    if (path === '/events' && method === 'POST') return writeJson(200, { accepted: 1, rejected: 0, invalid: 0 });
    if (path === '/query') return writeJson(200, { rows: [] });
    if (path === '/tail' && method === 'GET') return writeJson(200, { events: [] });
    if (path === '/events' && method === 'DELETE') return writeJson(200, { deleted: 1 });
    if (path === '/replay') return writeJson(202, { replayed: 1 });
    if (path === '/dlq') return writeJson(200, { events: [] });
    if (path.match(/^\/dlq\/[^/]+$/) && !path.endsWith('/replay')) return writeJson(200, { entry: {} });
    if (path.match(/^\/dlq\/[^/]+\/replay$/)) return writeJson(200, { replayed: 1 });
    if (path === '/keys' && method === 'POST') return writeJson(201, { id: 'k_1' });
    if (path.match(/^\/keys\/[^/]+$/) && method === 'DELETE') return writeJson(200, { revoked: true });
    if (path.match(/^\/keys\/[^/]+\/rotate$/)) return writeJson(200, { rotated: true });
    if (path === '/sinks') return writeJson(200, { sinks: [] });
    if (path.match(/^\/sinks\/[^/]+\/test$/)) return writeJson(200, { status: 'healthy' });
    if (path === '/policy/validate') return writeJson(200, { valid: true, errors: [] });
    if (path === '/schema/check') return writeJson(200, { valid: true });
    if (path === '/schema/publish') return writeJson(200, { published: true });
    if (path === '/retention/apply') return writeJson(200, { applied: true });
    writeJson(404, { error: 'not_found' });
  });
  await new Promise<void>((resolve) => collectorServer.listen(0, resolve));
  const collectorPort = (collectorServer.address() as any).port;
  const collector = new loza.CollectorClient({ url: `http://127.0.0.1:${collectorPort}`, enableCompression: false });

  assert.equal(await collector.health(), true);
  assert.equal(await collector.ready(), true);
  assert.equal((await collector.version()).version, '0.2.0');
  assert.equal((await collector.validate({ event: 'catalog.validate' })).valid, true);
  await collector.ingest([{ event_id: 'evt_1', event: 'catalog.ingest' }]);
  await collector.query({ query: 'select 1' });
  await collector.tail({ limit: 1 });
  await collector.delete({ event: 'catalog.delete' });
  await collector.replay({ event_ids: ['evt_1'] });
  await collector.dlqList({ limit: 1 });
  await collector.dlqRead('dlq_1');
  await collector.dlqReplay('dlq_1');
  await collector.keysCreate('catalog');
  await collector.keysRevoke('key_1');
  await collector.keysRotate('key_1');
  await collector.sinksList();
  await collector.sinksTest('stdout');
  assert.equal((await collector.policyValidate({ sample_rate: 1.0 })).valid, true);
  assert.equal((await collector.schemaCheck({ event: 'verification' })).valid, true);
  assert.equal((await collector.schemaPublish({ schema: 'v1' })).published, true);
  assert.equal((await collector.retentionApply({ days: 7 })).applied, true);
  await new Promise<void>((resolve) => collectorServer.close(() => resolve()));

  const cortexServer = http.createServer((req, res) => {
    const path = req.url || '/';
    const writeJson = (status: number, obj: any) => {
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (path === '/healthz' || path === '/readyz') return writeJson(200, { status: 'ok', ready: true });
    if (path.startsWith('/graph/')) return writeJson(200, { nodes: [{ id: 'checkout' }], edges: [] });
    writeJson(200, {
      incident_id: 'inc_123',
      timestamp: '2026-05-23T00:00:00Z',
      confidence: 0.9,
      related_services: ['checkout'],
      related_events: [],
      causal_chain: [],
      similar_past_incidents: [],
      suggested_remediations: [],
      symptoms: [],
      explain: 'ok',
    });
  });
  await new Promise<void>((resolve) => cortexServer.listen(0, resolve));
  const cortexPort = (cortexServer.address() as any).port;
  const cortex = new loza.CortexClient({ url: `http://127.0.0.1:${cortexPort}` });

  assert.equal(await cortex.health(), true);
  assert.equal(await cortex.ready(), true);
  assert.equal((await cortex.reconstruct('inc_123')).incident_id, 'inc_123');
  assert.equal((await cortex.similarIncidents('inc_123')).length, 0);
  assert.equal((await cortex.serviceGraph('checkout')).nodes[0].id, 'checkout');
  assert.equal((await cortex.incidentGraph('inc_123')).nodes[0].id, 'checkout');
  await cortex.recordRemediation({ incident_id: 'inc_123', action: 'restart' });
  await cortex.recordFeedback({ incident_id: 'inc_123', outcome: 'success' });
  await cortex.ingestBatch([{ event: 'catalog' }]);
  await cortex.ingestJsonl([{ event: 'catalog' }]);
  await new Promise<void>((resolve) => cortexServer.close(() => resolve()));
});

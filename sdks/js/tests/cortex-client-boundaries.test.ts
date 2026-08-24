import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {
  CortexClient, normalizeIncidentContext, normalizeGraphView, normalizeRemediation,
  validateIncidentContext, validateGraphView, validateRemediation, validateFeedback,
} from '../src/cortex/client.ts';

async function startCortex(): Promise<{ server: http.Server; url: string }> {
  const server = http.createServer((req, res) => {
    res.statusCode = req.url === '/readyz' ? 503 : 200;
    res.setHeader('content-type', 'application/json');
    if (req.url === '/reconstruct') res.end(JSON.stringify({ incident_id: ' incident ', timestamp: '', confidence: 2, similar_incidents: [{ id: 1 }] }));
    else if (req.url?.startsWith('/graph/')) res.end(JSON.stringify({ nodes: [{ id: ' node ' }], edges: [{ source: 'node', target: 'node' }] }));
    else res.end('{}');
  });
  const gate = Promise.withResolvers<void>();
  server.once('error', gate.reject);
  server.listen(0, '127.0.0.1', () => gate.resolve());
  await gate.promise;
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('missing address');
  return { server, url: `http://127.0.0.1:${address.port}` };
}

async function stopCortex(server: http.Server): Promise<void> {
  const gate = Promise.withResolvers<void>();
  server.close(error => error ? gate.reject(error) : gate.resolve());
  await gate.promise;
}

describe('Cortex client boundaries', () => {
  it('normalizes and validates context, graph, remediation, and feedback values', () => {
    const context = { incident_id: ' id ', timestamp: '', confidence: 2, related_services: [' api ', ''] };
    normalizeIncidentContext(context);
    assert.equal(context.incident_id, 'id');
    assert.equal(context.confidence, 1);
    assert.deepEqual(context.related_services, ['api', '']);
    assert.ok(context.timestamp);
    normalizeIncidentContext({ incident_id: null, timestamp: null, confidence: 'unknown', related_services: {} });
    normalizeGraphView({ nodes: null });
    normalizeGraphView({ nodes: [{ id: 1 }] });
    assert.doesNotThrow(() => validateGraphView({ nodes: [], edges: [{ from_node_id: 'a', to_node_id: 'b' }] }));
    normalizeRemediation({ remediation_id: null, incident_id: null, action: null, operator: null, timestamp: null });
    assert.throws(() => validateIncidentContext(null), /nil/);
    assert.throws(() => validateIncidentContext({}), /incident_id/);
    assert.throws(() => validateIncidentContext({ incident_id: 'id' }), /timestamp/);
    assert.throws(() => validateIncidentContext({ incident_id: 'id', timestamp: 'now', confidence: 2 }), /confidence/);
    assert.throws(() => validateGraphView(null), /nil/);
    assert.throws(() => validateGraphView({ nodes: [{}], edges: [] }), /node 0/);
    assert.throws(() => validateGraphView({ nodes: [], edges: [{}] }), /source/);
    assert.throws(() => validateGraphView({ nodes: [], edges: [{ source: 'a' }] }), /target/);
    assert.throws(() => validateRemediation(null), /nil/);
    assert.throws(() => validateRemediation({}), /incident_id/);
    assert.throws(() => validateRemediation({ incident_id: 'id' }), /action/);
    assert.throws(() => validateFeedback(null), /nil/);
    assert.throws(() => validateFeedback({}), /incident_id/);
    assert.throws(() => validateFeedback({ incident_id: 'id' }), /outcome/);
  });

  it('calls health, readiness, ingest, graph, reconstruction, and feedback APIs', async () => {
    const { server, url } = await startCortex();
    try {
      const client = new CortexClient({ url: `${url}/`, apiKey: 'key', maxResponseBytes: 10000 });
      assert.equal(await client.health(), true);
      assert.equal(await client.ready(), false);
      await client.ingestBatch([{ event: 'one' }]);
      await client.ingestJsonl([{ event: 'one' }, { event: 'two' }]);
      const reconstruction = await client.reconstruct('incident');
      assert.equal(reconstruction.incident_id, 'incident');
      assert.equal(reconstruction.confidence, 1);
      assert.deepEqual(await client.similarIncidents('incident', 1), [{ id: 1 }]);
      assert.deepEqual(await client.serviceGraph('checkout'), { nodes: [{ id: 'node' }], edges: [{ source: 'node', target: 'node' }] });
      assert.deepEqual(await client.incidentGraph('incident id'), { nodes: [{ id: 'node' }], edges: [{ source: 'node', target: 'node' }] });
      await client.recordRemediation({ incident_id: 'id', action: 'fix' });
      await client.recordFeedback({ incident_id: 'id', outcome: 'resolved' });
      const unreachable = new CortexClient({ url: 'http://127.0.0.1:1', timeout: 10 });
      assert.equal(await unreachable.health(), false);
    } finally {
      await stopCortex(server);
    }
  });
});

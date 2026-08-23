import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { CollectorClient, LqlCompilationError } from '../src/collector/client.ts';

async function startCollector(handler: (req: http.IncomingMessage, res: http.ServerResponse) => void): Promise<{ server: http.Server; url: string }> {
  const server = http.createServer(handler);
  const gate = Promise.withResolvers<void>();
  server.once('error', gate.reject);
  server.listen(0, '127.0.0.1', () => gate.resolve());
  await gate.promise;
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('missing address');
  return { server, url: `http://127.0.0.1:${address.port}` };
}

async function stopCollector(server: http.Server): Promise<void> {
  const gate = Promise.withResolvers<void>();
  server.close(error => error ? gate.reject(error) : gate.resolve());
  await gate.promise;
}

describe('CollectorClient boundaries', () => {
  it('validates credentials, health, readiness, and version endpoints', async () => {
    assert.throws(() => new CollectorClient({ url: 'https://user:pass@example.com' }), /credentials/);
    assert.throws(() => new CollectorClient({ url: 'https://example.com', password: 'secret' }), /password requires/);
    assert.throws(() => new CollectorClient({ url: 'https://example.com', username: 'private' }), /require a password/);
    assert.throws(() => new CollectorClient({ url: 'http://example.com', username: 'u', password: 'p' }), /require HTTPS/);
    let requests = 0;
    const { server, url } = await startCollector((req, res) => {
      requests++;
      if (req.url?.endsWith('/health')) res.statusCode = 200;
      else if (req.url?.endsWith('/ready')) res.statusCode = 503;
      else res.statusCode = 200;
      res.setHeader('content-type', 'application/json');
      res.end(req.url?.endsWith('/version') ? JSON.stringify({ version: '1', ingest_api_version: 'v1', schema_version: 'v1', event_version: 'v1' }) : '{}');
    });
    try {
      const client = new CollectorClient({ url: `${url}/`, apiKey: 'key', authHeader: 'Authorization', collectorName: 'tenant one', enableCompression: false });
      assert.equal(await client.health(), true);
      assert.equal(await client.ready(), false);
      assert.deepEqual(await client.version(), { version: '1', ingest_api_version: 'v1', schema_version: 'v1', event_version: 'v1' });
      assert.equal(requests, 3);
      const unreachable = new CollectorClient({ url: 'http://127.0.0.1:1', timeout: 10 });
      assert.equal(await unreachable.health(), false);
      assert.equal(await unreachable.ready(), false);
    } finally {
      await stopCollector(server);
    }
  });

  it('sends batches, validates events, and invokes admin API endpoints', async () => {
    const paths: string[] = [];
    const { server, url } = await startCollector((req, res) => {
      paths.push(req.url || '');
      res.statusCode = 200;
      res.setHeader('content-type', 'application/json');
      const payload = req.url?.endsWith('/lql/query')
        ? { columns: ['event', 3], rows: [{ event: 'ok' }, null], duration_ms: 4 }
        : req.method === 'POST' && req.url?.endsWith('/events')
          ? { request_id: 'req', status: 'accepted', accepted: 1, rejected: 0, invalid: 0, acks: [] }
          : { ok: true };
      res.end(JSON.stringify(payload));
    });
    try {
      const client = new CollectorClient({ url, collectorName: 'tenant one', apiKey: 'key' });
      assert.equal((await client.sendBatch([{ event: 'one' }])).status, 'accepted');
      assert.deepEqual(await client.ingest([{ event: 'two' }]), { request_id: 'req', status: 'accepted', accepted: 1, rejected: 0, invalid: 0, acks: [] });
      assert.deepEqual(await client.validate({ event: 'one' }), { ok: true });
      assert.deepEqual(await client.query({ event: 'one' }), { ok: true });
      assert.deepEqual(await client.tail({ service: 'svc', limit: 2 }), { ok: true });
      assert.deepEqual(await client.tail(), { ok: true });
      assert.deepEqual(await client.delete({ service: 'svc' }), { ok: true });
      assert.deepEqual(await client.replay({ id: 'id' }), { ok: true });
      assert.deepEqual(await client.dlqList({ limit: 2 }), { ok: true });
      assert.deepEqual(await client.dlqList(), { ok: true });
      assert.deepEqual(await client.dlqRead('id with space'), { ok: true });
      assert.deepEqual(await client.dlqReplay('id with space'), { ok: true });
      assert.deepEqual(await client.keysCreate('name'), { ok: true });
      assert.deepEqual(await client.keysCreate('name', ['read']), { ok: true });
      assert.deepEqual(await client.keysRevoke('key id'), { ok: true });
      assert.deepEqual(await client.keysRotate('key id'), { ok: true });
      assert.deepEqual(await client.sinksList(), { ok: true });
      assert.deepEqual(await client.sinksTest('sink id'), { ok: true });
      assert.deepEqual(await client.policyValidate({}), { ok: true });
      assert.deepEqual(await client.schemaCheck({}), { ok: true });
      assert.deepEqual(await client.schemaPublish({}), { ok: true });
      assert.deepEqual(await client.retentionApply(), { ok: true });
      assert.deepEqual(await client.retentionApply({ days: 7 }), { ok: true });
      assert.ok(paths.some(path => path.includes('/collectors/tenant%20one/events')));
      assert.ok(paths.some(path => path.includes('/dlq/id%20with%20space')));
    } finally {
      await stopCollector(server);
    }
  });

  it('normalizes LQL limits and maps malformed and failed compiler responses', async () => {
    let queryCalls = 0;
    const { server, url } = await startCollector((req, res) => {
      if (req.url === '/lql/query') {
        queryCalls++;
        res.statusCode = queryCalls === 1 ? 200 : 400;
        res.setHeader('content-type', 'application/json');
        if (queryCalls === 1) res.end(JSON.stringify({ columns: ['event', 4], rows: [{ event: 'ok' }, null], duration_ms: 10 }));
        else res.end(JSON.stringify({ error: 'syntax error', diagnostics: [{ line: 1 }] }));
        return;
      }
      res.statusCode = 200;
      res.end('not-json');
    });
    try {
      const client = new CollectorClient({ url, enableCompression: false });
      assert.deepEqual(await client.queryLql('select', {}, 0), { columns: ['event'], rows: [{ event: 'ok' }], duration_ms: 10 });
      await assert.rejects(() => client.queryLql('bad', {}, 1001), (error: unknown) => {
        assert.ok(error instanceof LqlCompilationError);
        assert.equal(error.status, 400);
        assert.equal(error.message, 'syntax error');
        assert.deepEqual(error.diagnostics, [{ line: 1 }]);
        return true;
      });
    } finally {
      await stopCollector(server);
    }

    const { server: malformedServer, url: malformedUrl } = await startCollector((_req, res) => {
      res.statusCode = 200;
      res.end('invalid');
    });
    try {
      const client = new CollectorClient({ url: malformedUrl });
      await assert.rejects(() => client.queryLql('bad'), (error: unknown) => error instanceof LqlCompilationError && error.status === 200);
    } finally {
      await stopCollector(malformedServer);
    }
    const { server: emptyServer, url: emptyUrl } = await startCollector((_req, res) => {
      res.statusCode = 200;
      res.end('null');
    });
    try {
      const client = new CollectorClient({ url: emptyUrl });
      assert.deepEqual(await client.queryLql('empty', undefined, Number.NaN), {
        columns: [], rows: [], duration_ms: undefined,
      });
    } finally {
      await stopCollector(emptyServer);
    }
  });
});

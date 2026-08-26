import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { test } from 'node:test';
import { CollectorClient } from '../src/collector/client.ts';

test('public Basic credentials reach a collector-scoped endpoint without URL userinfo', async () => {
  const capability = 'lz_pub_6DJvd3D0izOaQx3n5BhKqN';
  let requestPath = '';
  let authorization = '';
  const server = createServer((request, response) => {
    requestPath = request.url ?? '';
    authorization = request.headers.authorization ?? '';
    response.writeHead(202, { 'content-type': 'application/json' });
    response.end('{"request_id":"req_1","status":"accepted","accepted":1,"rejected":0,"invalid":0,"acks":[]}');
  });

  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (address === null || typeof address === 'string') {
    throw new Error('test server did not expose a TCP address');
  }
  const { port } = address;

  try {
    const client = new CollectorClient({
      url: `http://127.0.0.1:${port}`,
      collectorName: 'public-collector',
      username: capability,
      password: '',
      enableCompression: false,
    });
    await client.sendBatch([{ event: 'checkout.completed' }]);
  } finally {
    server.close();
    await once(server, 'close');
  }

  assert.equal(requestPath, '/collectors/public-collector/events');
  assert.equal(authorization, `Basic ${Buffer.from(`${capability}:`, 'utf8').toString('base64')}`);
  assert.equal(requestPath.includes(capability), false);
});

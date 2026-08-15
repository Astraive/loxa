import { strict as assert } from 'node:assert';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { Client, ErrorCategory } from '../src/index.ts';

test('query uses scoped route, typed parameters, and bearer precedence', async () => {
  const server = createServer((request, response) => {
    let body = '';
    request.on('data', chunk => { body += chunk; });
    request.on('end', () => {
      assert.equal(request.url, '/collectors/demo/lql/query');
      assert.equal(request.headers.authorization, 'Bearer api');
      assert.equal(request.headers['x-loza-env'], 'prod');
      assert.equal(request.headers['x-loza-service'], 'cli');
      const parsed = JSON.parse(body);
      assert.equal(parsed.parameters.id.type, 'string');
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify({ columns: [{ name: 'event_id', type: 'string' }], rows: [{ event_id: 'evt-1' }], duration_ms: 2, row_count: 1 }));
    });
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  assert.ok(address && typeof address === 'object');
  try {
    const client = new Client({ endpoint: `http://127.0.0.1:${address.port}`, collector: 'demo', apiKey: 'api', username: 'user', password: 'pass', env: 'prod', service: 'cli' });
    const result = await client.query('from events | where event_id = $id', { id: { type: 'string', value: 'evt-1' } }, 10);
    assert.equal(result.rowCount, 1);
  } finally {
    server.close();
  }
});

test('invalid configuration exposes stable category', () => {
  assert.throws(() => new Client({ endpoint: 'http://remote.example', collector: '', username: 'u', password: 'p' }), error => {
    if (!(error instanceof Error) || !('category' in error)) return false;
    return error.category === ErrorCategory.InvalidConfiguration;
  });
});

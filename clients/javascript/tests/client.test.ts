import { strict as assert } from 'node:assert';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { Client, ErrorCategory, QueryError } from '../src/index.ts';

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

test('query encodes inferred parameter types and basic authorization', async () => {
  let request: { url: string; init: RequestInit } | undefined;
  const client = new Client({
    endpoint: 'http://localhost:9308',
    collector: 'demo',
    username: 'user',
    password: 'pass',
    fetch: async (url, init) => {
      request = { url, init };
      return new Response(JSON.stringify({ columns: ['id'], rows: [{ id: 1 }] }), { status: 200 });
    },
  });
  const result = await client.query('from events', {
    nil: null,
    flag: true,
    count: 2,
    ratio: 1.5,
    text: 'hello',
    typed: { type: 'bytes', value: 'raw' },
  }, 5000);
  assert.equal(result.rowCount, 1);
  assert.ok(request);
  assert.equal(request.url, 'http://localhost:9308/collectors/demo/lql/query');
  assert.equal((request.init.headers as Record<string, string>).Authorization, 'Basic dXNlcjpwYXNz');
  const body = JSON.parse(String(request.init.body));
  assert.equal(body.limit, 1000);
  assert.deepEqual(body.parameters, {
    nil: { type: 'null', value: null },
    flag: { type: 'bool', value: true },
    count: { type: 'int', value: 2 },
    ratio: { type: 'float', value: 1.5 },
    text: { type: 'string', value: 'hello' },
    typed: { type: 'bytes', value: 'raw' },
  });
});

test('query decodes columns and applies result defaults', async () => {
  const client = new Client({
    endpoint: 'https://api.example',
    collector: 'demo',
    apiKey: 'key',
    fetch: async () => new Response(JSON.stringify({
      columns: ['id', { name: 'value', type: 42, nullable: 'yes' }, { name: 'flag', type: 'bool', nullable: true }],
      rows: [{ id: 1 }],
    })),
  });
  const result = await client.query('from events');
  assert.deepEqual(result.columns, [
    { name: 'id' },
    { name: 'value', type: undefined, nullable: undefined },
    { name: 'flag', type: 'bool', nullable: true },
  ]);
  assert.equal(result.durationMs, 0);
  assert.equal(result.rowCount, 1);
});

test('query accepts a DSN and derives endpoint, credentials, and headers', async () => {
  let requestURL = '';
  let requestHeaders: Record<string, string> | undefined;
  const client = new Client({
    dsn: 'loza://u%40ser:p%40ss@localhost/demo?tls=false&env=staging&service=worker',
    fetch: async (url, init) => {
      requestURL = url;
      requestHeaders = init.headers as Record<string, string>;
      return new Response(JSON.stringify({ columns: [], rows: [], duration_ms: 4, row_count: 0 }));
    },
  });
  await client.query('from events');
  assert.equal(requestURL, 'http://localhost:9308/collectors/demo/lql/query');
  assert.equal(requestHeaders?.Authorization, 'Basic dUBzZXI6cEBzcw==');
  assert.equal(requestHeaders?.['X-Loza-Env'], 'staging');
  assert.equal(requestHeaders?.['X-Loza-Service'], 'worker');
});

test('query accepts public DSN credentials with automatic TLS', async () => {
  let requestURL = '';
  const client = new Client({
    dsn: 'loza://lz_pub_token@api.example/demo',
    fetch: async url => {
      requestURL = url;
      return new Response(JSON.stringify({ columns: [], rows: [] }));
    },
  });
  await client.query('from events');
  assert.equal(requestURL, 'https://api.example:443/collectors/demo/lql/query');
});

test('rejects malformed DSNs and invalid DSN credentials', () => {
  for (const dsn of ['not a dsn', 'loza:///demo', 'loza://user@localhost/demo', 'loza://localhost/demo?tls=maybe']) {
    assert.throws(() => new Client({ dsn }), error => error instanceof QueryError && error.category === ErrorCategory.InvalidConfiguration);
  }
});

test('rejects invalid endpoint and authentication configurations', () => {
  const invalidConfigurations = [
    { collector: 'demo' },
    { endpoint: 'ftp://localhost', collector: 'demo' },
    { endpoint: 'http://localhost', collector: 'bad slug' },
    { endpoint: 'http://localhost', collector: 'demo', username: 'user' },
    { endpoint: 'http://remote.example', collector: 'demo', username: 'user', password: 'pass' },
  ];
  for (const config of invalidConfigurations) {
    assert.throws(() => new Client(config), error => error instanceof QueryError && error.category === ErrorCategory.InvalidConfiguration);
  }
});

test('rejects an empty query source before making a request', async () => {
  const client = new Client({ endpoint: 'http://localhost', collector: 'demo', fetch: async () => {
    throw new Error('request should not be made');
  } });
  await assert.rejects(client.query('   '), error => error instanceof QueryError && error.category === ErrorCategory.InvalidConfiguration);
});

test('maps HTTP errors to stable categories, messages, and diagnostics', async () => {
  const responses: Array<[number, string, string, Record<string, unknown>[], string]> = [
    [400, JSON.stringify({ error: 'bad query', diagnostics: [{ line: 1 }, 'invalid'] }), 'bad query', [], ErrorCategory.Diagnostics],
    [401, JSON.stringify({ message: 'unauthorized', diagnostics: [{ code: 'auth' }] }), 'unauthorized', [{ code: 'auth' }], ErrorCategory.Authentication],
    [403, 'not json', 'LQL query failed with HTTP 403', [], ErrorCategory.Scope],
    [503, JSON.stringify({}), 'LQL query failed with HTTP 503', [], ErrorCategory.CompilerUnavailable],
    [500, JSON.stringify({ error: 42 }), 'LQL query failed with HTTP 500', [], ErrorCategory.Execution],
  ];
  for (const [status, body, message, diagnostics, category] of responses) {
    const client = new Client({
      endpoint: 'https://api.example',
      collector: 'demo',
      fetch: async () => new Response(body, { status }),
    });
    await assert.rejects(client.query('from events'), error => {
      assert.ok(error instanceof QueryError);
      assert.equal(error.category, category);
      assert.equal(error.message, message);
      assert.deepEqual(error.diagnostics, diagnostics);
      assert.equal(error.status, status);
      return true;
    });
  }
});

test('rejects oversized responses and malformed result envelopes', async () => {
  const cases: Array<{ body: string; maxResponseBytes?: number }> = [
    { body: '{not json' },
    { body: '{}' },
    { body: JSON.stringify({ columns: [null], rows: [] }) },
    { body: JSON.stringify({ columns: [], rows: [1] }) },
    { body: JSON.stringify({ columns: [], rows: [] }), maxResponseBytes: 1 },
  ];
  for (const { body, maxResponseBytes } of cases) {
    const client = new Client({
      endpoint: 'https://api.example',
      collector: 'demo',
      maxResponseBytes,
      fetch: async () => new Response(body),
    });
    await assert.rejects(client.query('from events'), error => error instanceof QueryError && error.category === ErrorCategory.MalformedResponse);
  }
});

test('maps transport failures and aborts to transport and timeout errors', async () => {
  const transportClient = new Client({
    endpoint: 'https://api.example',
    collector: 'demo',
    fetch: async () => {
      throw new Error('offline');
    },
  });
  await assert.rejects(transportClient.query('from events'), error => error instanceof QueryError && error.category === ErrorCategory.Transport);

  const timeoutClient = new Client({
    endpoint: 'https://api.example',
    collector: 'demo',
    timeoutMs: 1,
    fetch: async (_url, init) => new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
    }),
  });
  await assert.rejects(timeoutClient.query('from events'), error => error instanceof QueryError && error.category === ErrorCategory.Timeout);
});

test('exposes QueryError metadata', () => {
  const error = new QueryError('failed', ErrorCategory.Scope, 403, [{ field: 'scope' }]);
  assert.equal(error.name, 'QueryError');
  assert.equal(error.category, ErrorCategory.Scope);
  assert.equal(error.status, 403);
  assert.deepEqual(error.diagnostics, [{ field: 'scope' }]);
});

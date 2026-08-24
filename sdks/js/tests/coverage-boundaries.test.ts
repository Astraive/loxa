import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { EventView } from '../src/core/event-view.ts';
import {
  DefaultSchema,
  OTelLogSchema,
  ECSchema,
  DatadogSchema,
  FlatSchema,
} from '../src/core/schema.ts';
import { HTTPBatchSink, MemorySink, RotatingFileSink } from '../src/sinks/standard-sinks.ts';
import { Logger, fromRequest } from '../src/core/logger.ts';
import { run } from '../src/core/logger.ts';
import { Event, String as AttrString } from '../src/core/event.ts';
import { lozaMiddleware } from '../src/middleware/express.ts';
import { lozaFastifyPlugin } from '../src/middleware/fastify.ts';
import { lozaHonoMiddleware } from '../src/middleware/hono.ts';
import { lozaKoaMiddleware } from '../src/middleware/koa.ts';
import { lozaNextMiddleware } from '../src/middleware/next.ts';
import { sanitizeEvent } from '../src/core/sanitize.ts';
import { setPath, getPath, deletePath } from '../src/core/paths.ts';
import {
  normalizeIncidentContext,
  normalizeGraphView,
  normalizeRemediation,
  validateGraphView,
} from '../src/cortex/client.ts';
import { parse as parseDSN } from '../src/config/dsn.ts';
import { defaultConfig, ConfigBuilder, fromEnv } from '../src/config/config.ts';
import { mergeFileConfig } from '../src/config/config-file.ts';
import type { CollectorResponse } from '../src/generated/spec-contract.ts';

type SinkInternals = {
  classifyOutcome(statusCode: number, response: CollectorResponse): string;
  parseRetryAfter(value: string | null | undefined): number | null;
  notifyCollectorAck(response: CollectorResponse): void;
  post(body: Buffer): Promise<{ statusCode: number; retryAfterHeader: string | null; response: CollectorResponse }>;
};

type RotatingInternals = { pruneBackups(): void };

function response(overrides: Partial<CollectorResponse> = {}): CollectorResponse {
  return {
    request_id: '', status: 'accepted', accepted: 0, rejected: 0, invalid: 0, acks: [],
    ...overrides,
  };
}

describe('Coverage boundaries', () => {
  it('encodes every optional schema field and nested attribute shape', () => {
    const view = {
      state: 'emitting', outcome: 'success', schemaVersion: '1', eventVersion: '1',
      eventId: 'event-id', requestId: 'request-id', timestamp: '2025-01-01T00:00:00.000Z',
      service: 'service', event: 'event', kind: 'http', level: 'info', message: 'message',
      version: 'version', environment: 'production', deploymentId: 'deployment', region: 'region',
      host: 'host', runtime: 'node', durationMs: 10, traceId: 'trace', spanId: 'span',
      incidentId: 'incident', parentId: 'parent', finishedAt: 1735689600000,
      method: 'GET', path: '/', route: '/', statusCode: 200,
      attrs: {
        'user.id': 'user', 'tenant.id': 'tenant', 'http.agent': 'agent',
        'resource.name': 'resource', 'nested.value': 1, plain: true,
      },
      checkpoints: [{ name: 'checkpoint' }], processes: [{ name: 'process' }],
      groups: [{ name: 'group' }], timers: [{ name: 'timer' }],
      error: { type: 'Error', message: 'failed', code: 'E_FAIL', stack: 'stack', retryable: true },
    } as unknown as EventView;

    const encoded = new DefaultSchema().encode(view);
    assert.equal(encoded.event_state, 'finished');
    assert.equal(encoded.deployment_id, 'deployment');
    assert.deepEqual(encoded.user, { id: 'user' });
    assert.deepEqual(encoded.tenant, { id: 'tenant' });
    assert.deepEqual(encoded.resource, { name: 'resource' });
    assert.deepEqual(new OTelLogSchema().encode(view).error, { type: 'Error', message: 'failed', code: 'E_FAIL', retryable: true });
    assert.deepEqual(new ECSchema().encode(view).error?.code, 'E_FAIL');
    assert.deepEqual(new DatadogSchema().encode(view).error?.code, 'E_FAIL');
    assert.equal(new FlatSchema().encode(view)['nested.value'], 1);

    const nestedCollision = {
      ...view,
      attrs: { a: 'scalar', 'a.b': 1, 'http.x': 'x' },
      method: '', path: '', route: '', statusCode: 0,
      checkpoints: [], processes: [], groups: [], timers: [], error: undefined,
    } as unknown as EventView;
    const collision = new DefaultSchema().encode(nestedCollision);
    assert.deepEqual(collision.attrs, { a: { b: 1 } });

    const sparse = {
      state: 'created', outcome: '', schemaVersion: '1', eventVersion: '1', eventId: 'id',
      requestId: '', timestamp: '2025-01-01T00:00:00.000Z', service: '', event: 'event', kind: '',
      level: 'info', message: '', version: '', environment: '', deploymentId: '', region: '',
      host: '', runtime: '', durationMs: 0, traceId: '', spanId: '', incidentId: '', parentId: '',
      finishedAt: 0, method: '', path: '', route: '', statusCode: 0, attrs: {},
      checkpoints: [], processes: [], groups: [], timers: [], error: undefined,
    } as unknown as EventView;
    assert.equal('http' in new DefaultSchema().encode(sparse), false);
    assert.equal('error' in new OTelLogSchema().encode(sparse), false);
    assert.equal('service' in new ECSchema().encode(sparse), false);
    assert.equal('request_id' in new DatadogSchema().encode(sparse), false);
  });

  it('covers sink classification, retry parsing, acknowledgements, rotation, and retries', async () => {
    const sink = new HTTPBatchSink({ endpoint: 'http://localhost/events', retries: 1, baseDelay: 0, maxDelay: 0, enableCompression: false });
    const internals = sink as unknown as SinkInternals;
    const classify = (statusCode: number, value: CollectorResponse) => internals.classifyOutcome(statusCode, value);
    assert.equal(classify(200, response({ acks: [{ index: 0, event_id: '', status: 'retryable', retryable: true }] })), 'retryable');
    assert.equal(classify(200, response({ errors: [{ index: 0, event_id: '', code: '', message: '', retryable: true }] })), 'retryable');
    assert.equal(classify(429, response()), 'retryable');
    assert.equal(classify(503, response()), 'retryable');
    assert.equal(classify(500, response()), 'permanent');
    assert.equal(classify(200, response({ status: 'rejected', accepted: 1 })), 'retryable');
    assert.equal(classify(200, response({ status: 'rejected', accepted: 0 })), 'permanent');
    assert.equal(classify(200, response()), 'success');

    const parseRetryAfter = (value: string | null | undefined) => internals.parseRetryAfter(value);
    assert.equal(parseRetryAfter(undefined), null);
    assert.equal(parseRetryAfter('   '), null);
    assert.equal(parseRetryAfter('1.5'), 1500);
    assert.equal(parseRetryAfter('not-a-date'), null);
    assert.equal(parseRetryAfter(new Date(Date.now() + 60_000).toUTCString()) >= 0, true);
    assert.equal(parseRetryAfter(new Date(Date.now() - 60_000).toUTCString()), 0);

    let callbackCalls = 0;
    const acknowledged = new HTTPBatchSink({
      endpoint: 'http://localhost/events', enableCompression: false,
      statsHandler: { onCollectorAck() { callbackCalls++; } },
    });
    const ackInternals = acknowledged as unknown as SinkInternals;
    ackInternals.notifyCollectorAck(response({ request_id: 'request', deduped: 1, acks: [], errors: [] }));
    ackInternals.notifyCollectorAck({ ...response(), acks: undefined } as unknown as CollectorResponse);
    assert.equal(callbackCalls, 2);
    const throwingAck = new HTTPBatchSink({
      endpoint: 'http://localhost/events', enableCompression: false,
      statsHandler: { onCollectorAck() { throw new Error('ignored'); } },
    });
    assert.doesNotThrow(() => (throwingAck as unknown as SinkInternals).notifyCollectorAck(response()));

    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-coverage-'));
    try {
      const file = path.join(tempDir, 'events.ndjson');
      fs.writeFileSync(`${file}.old`, 'old');
      fs.writeFileSync(`${file}.new`, 'new');
      fs.writeFileSync(path.join(tempDir, 'unrelated.txt'), 'ignore');
      const rotating = new RotatingFileSink(file, 1, 1);
      (rotating as unknown as RotatingInternals).pruneBackups();
      assert.equal(fs.existsSync(`${file}.old`) || fs.existsSync(`${file}.new`), true);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }

    let attempts = 0;
    const retrying = new HTTPBatchSink({ endpoint: 'http://localhost/events', retries: 1, baseDelay: 0, maxDelay: 0, enableCompression: false, batchSize: 1 });
    const retryInternals = retrying as unknown as SinkInternals;
    retryInternals.post = async () => {
      attempts++;
      if (attempts === 1) throw new Error('transient');
      return { statusCode: 200, retryAfterHeader: null, response: response({ accepted: 1 }) };
    };
    await retrying.write('{"event":"retry"}');
    assert.equal(attempts, 2);
    const primitiveFailure = new HTTPBatchSink({ endpoint: 'http://localhost/events', retries: 0, enableCompression: false, batchSize: 1 });
    const primitiveInternals = primitiveFailure as unknown as SinkInternals;
    primitiveInternals.post = async () => { throw 'primitive failure'; };
    const server = http.createServer((_request, res) => {
      res.statusCode = 200;
      res.end(JSON.stringify({ status: 'accepted', accepted: 1, rejected: 0, invalid: 0, acks: [] }));
    });
    await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('missing test server address');
    const customAuth = new HTTPBatchSink({
      endpoint: `http://127.0.0.1:${address.port}/events`,
      apiKey: 'key', authHeader: 'X-Custom-Key', enableCompression: false, batchSize: 1,
    });
    await customAuth.write('{"event":"custom-auth"}');
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    await assert.rejects(() => primitiveFailure.write('{"event":"primitive"}'), /primitive failure/);
  });

  it('covers logger fallback fields, idempotent emission, and empty requests', async () => {
    const logger = new Logger({ service: 'service', version: 'version', environment: 'production', sink: new MemorySink() });
    const event = logger.startEvent({ event: 'event' });
    assert.equal(event.service, 'service');
    assert.equal(event.version, 'version');
    assert.equal(event.environment, 'production');
    await logger.emit(event);
    assert.equal(await logger.emit(event), null);
    const emptyRequest = fromRequest({}, logger);
    assert.equal(emptyRequest.path, '/');
    const fallback = logger.startEvent({ event: 'fallback', service: '', version: '', environment: '' });
    assert.equal(fallback.service, 'service');
    assert.equal(fallback.version, 'version');
    assert.equal(fallback.environment, 'production');
    logger.linkEvent(fallback, 'linked');
    await run(logger.startEvent({ event: 'run' }), () => {});
    await run(logger.startEvent({ event: 'run-error' }), () => { throw new Error('run'); });
    assert.equal(fromRequest({}).path, '/');
    const alreadyFinished = logger.startEvent({ event: 'already-finished' });
    alreadyFinished.finish('success');
    await run(alreadyFinished, () => {});
  });
  it('covers framework middleware success, fallback, and error paths', async () => {
    let nextCalls = 0;
    let endCalls = 0;
    const response = {
      statusCode: 200,
      end(..._args: unknown[]) { endCalls++; },
      on(_event: string, _handler: (error: Error) => void) {},
    };
    const express = lozaMiddleware({ routeExtractor: () => '' });
    express({ method: 'GET', path: '/express', headers: {}, socket: {} }, response, () => { nextCalls++; });
    response.end();
    let errorHandler: ((error: Error) => void) | undefined;
    const errorResponse = {
      statusCode: 500,
      end(..._args: unknown[]) {},
      on(_event: string, handler: (error: Error) => void) { errorHandler = handler; },
    };
    express({ method: 'GET', path: '/express-error', headers: {}, socket: {} }, errorResponse, () => {});
    errorHandler?.(new Error('express'));

    const fastify = lozaFastifyPlugin();
    await fastify(
      { method: 'GET', url: '/fastify', headers: {}, socket: {} },
      { statusCode: 200, then: (ok: () => void) => Promise.resolve().then(ok) },
    );
    await fastify(
      { method: 'GET', url: '/fastify-error', headers: {}, socket: {} },
      { statusCode: 500, then: (_ok: () => void, error: (reason: Error) => void) => Promise.resolve().then(() => error(new Error('fastify'))) },
    );

    const hono = lozaHonoMiddleware();
    const honoContext = {
      req: { method: 'GET', path: '/hono', header: (_name: string) => undefined },
      res: { status: 200 },
    };
    await hono(honoContext, async () => {});
    await hono({ ...honoContext, res: { status: 500 } }, async () => { throw new Error('hono'); });

    const koa = lozaKoaMiddleware();
    await koa({ method: 'GET', path: '/koa', headers: {}, status: 200 }, async () => {});
    await koa({ method: 'GET', path: '/koa-error', headers: {}, status: 0 }, async () => { throw new Error('koa'); });

    const next = lozaNextMiddleware();
    await next({ url: '/next', headers: { get: (_name: string) => undefined } }, async () => ({ status: 200 }));
    await assert.rejects(
      () => next({ url: '/next-error', headers: { get: (_name: string) => undefined } }, async () => { throw new Error('next'); }),
      /next/,
    );
    assert.equal(nextCalls, 1);
    assert.equal(endCalls, 1);
  });

  it('covers path and sanitization fallbacks', () => {
    const root: Record<string, unknown> = { value: 'scalar', list: [] };
    setPath(root, 'value.child', 1);
    setPath(root, 'list.child', 2);
    assert.equal(getPath(root, 'value.child'), 1);
    assert.equal(getPath(root, 'missing.child'), undefined);
    deletePath(root, 'value.child');
    deletePath({ value: 'scalar' }, 'value.child');

    const event = new Event({ event: 'sanitize' }, 'service', 'test');
    event.enrich(
      { ...AttrString('secret', 'hidden'), sensitive: true },
      { ...AttrString('hash', 'hashed'), hashValue: true },
      { ...AttrString('not-string', 'value'), hashValue: true },
    );
    const sanitized = sanitizeEvent(event);
    assert.equal(sanitized.attrs.secret, '[REDACTED]');
    assert.notEqual(sanitized.attrs.hash, 'hashed');
  });
  it('covers explicit DSN transport and TLS branches', () => {
    assert.equal(parseDSN('loza://localhost/project?tls=auto&transport=http').tls, false);
    assert.equal(parseDSN('loza://example.com/project?tls=true&transport=otlp').transport, 'otlp');
    assert.equal(parseDSN('loza://example.com/project?tls=false&transport=grpc').transport, 'grpc');
  });
  it('covers credentialed config overlays and empty file configuration', () => {
    const credentialed = mergeFileConfig(defaultConfig(), {
      collector_url: 'loza://key:secret@example.com/project',
    });
    assert.equal(credentialed.password, 'secret');
    const publicConfig = mergeFileConfig(defaultConfig(), {
      collector_url: 'loza://lx_pub_capability:@example.com/project',
    });
    assert.equal(publicConfig.password, '');
    const builder = new ConfigBuilder(defaultConfig());
    builder.withOtelBridge(false);
    builder.withOtelBridge(true);
    assert.equal(builder.build().async.enabled, true);

    const previousCwd = process.cwd();
    const previousDefaults = process.env.LOZA_JS_DEFAULTS;
    const previousConfig = process.env.LOZA_JS_CONFIG;
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-empty-config-'));
    try {
      process.chdir(tempDir);
      process.env.LOZA_JS_DEFAULTS = tempDir;
      process.env.LOZA_JS_CONFIG = tempDir;
      delete process.env.LOZA_DSN;
      fromEnv();
    } finally {
      process.chdir(previousCwd);
      if (previousDefaults === undefined) delete process.env.LOZA_JS_DEFAULTS;
      else process.env.LOZA_JS_DEFAULTS = previousDefaults;
      if (previousConfig === undefined) delete process.env.LOZA_JS_CONFIG;
      else process.env.LOZA_JS_CONFIG = previousConfig;
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it('covers sparse Cortex normalization and graph edge aliases', () => {
    normalizeIncidentContext(null);
    const incident: { incident_id: string; timestamp: string; related_services: string[] } = {
      incident_id: ' id ', timestamp: '', related_services: ['', ' api '],
    };
    normalizeIncidentContext(incident);
    assert.deepEqual(incident.related_services, ['', 'api']);
    normalizeGraphView(null);
    const graph = { nodes: [{ id: ' n ' }], edges: [{ from_node_id: 'n', to_node_id: 'm' }] };
    normalizeGraphView(graph);
    validateGraphView({ nodes: [], edges: [{ source: 's', target: 't' }] });
    normalizeRemediation(null);
    assert.equal(graph.nodes[0].id, 'n');
  });
});

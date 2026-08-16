import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { lozaHttpMiddleware } from '../src/middleware/http.ts';
import { lozaMiddleware } from '../src/middleware/express.ts';
import { lozaKoaMiddleware } from '../src/middleware/koa.ts';
import { lozaFastifyPlugin } from '../src/middleware/fastify.ts';
import { lozaHonoMiddleware } from '../src/middleware/hono.ts';
import { lozaNextMiddleware } from '../src/middleware/next.ts';

type HttpResponse = {
  statusCode: number;
  end: (...args: unknown[]) => unknown;
  on: (name: string, handler: (err: Error) => void) => void;
};

describe('framework middleware', () => {
  it('records HTTP middleware success and error responses', async () => {
    let errorHandler: ((err: Error) => void) | undefined;
    const errorResponse: HttpResponse = {
      statusCode: 200,
      end() { return 'ignored'; },
      on(name, handler) { if (name === 'error') errorHandler = handler; },
    };
    const req = { method: 'GET', url: '/health', headers: {}, socket: {} };
    let nextCalls = 0;
    const middleware = lozaHttpMiddleware({ service: 'edge' });
    middleware(req, errorResponse, () => { nextCalls++; });
    assert.equal(nextCalls, 1);
    assert.ok(errorHandler);
    errorHandler(new Error('socket failed'));
    await Promise.resolve();

    let ended = false;
    const successResponse: HttpResponse = {
      statusCode: 200,
      end() { ended = true; return 'ended'; },
      on() {},
    };
    lozaHttpMiddleware({ service: 'edge' })(req, successResponse, () => {});
    assert.equal(successResponse.end('body'), 'ended');
    assert.equal(ended, true);

    const res500: HttpResponse = {
      statusCode: 500,
      end() {},
      on() {},
    };
    lozaHttpMiddleware()(req, res500, () => {});
    res500.end();
  });

  it('records Express routes from extractor, route path, and request path', () => {
    const responses: number[] = [];
    const makeRes = (statusCode: number): HttpResponse => ({
      statusCode,
      end() { responses.push(statusCode); },
      on() {},
    });
    let nextCalls = 0;
    const extractor = lozaMiddleware({ service: 'api', routeExtractor: () => '/custom' });
    const first = makeRes(200);
    extractor(
      { method: 'POST', path: '/ignored', route: { path: '/route' }, headers: { 'user-agent': 'ua' }, ip: '1.2.3.4' },
      first,
      () => { nextCalls++; },
    );
    first.end();
    const routeResponse = makeRes(404);
    lozaMiddleware()(
      { method: 'GET', path: '/path', route: { path: '' } },
      routeResponse,
      () => { nextCalls++; },
    );
    routeResponse.end();
    const fallbackResponse = makeRes(500);
    lozaMiddleware()(
      { method: 'GET', path: '/path', headers: {}, socket: {} },
      fallbackResponse,
      () => { nextCalls++; },
    );
    fallbackResponse.end();
    assert.equal(nextCalls, 3);
    assert.deepEqual(responses, [200, 404, 500]);
  });

  it('finishes Koa events after success and error', async () => {
    let successRan = false;
    await lozaKoaMiddleware({ service: 'koa' })(
      { method: 'GET', path: '/ok', route: { path: '/ok' }, headers: {}, socket: {}, status: 201 },
      async () => { successRan = true; },
    );
    assert.equal(successRan, true);
    await assert.doesNotReject(() => lozaKoaMiddleware()(
      { method: 'GET', path: '/bad', headers: {}, socket: {}, status: 500 },
      async () => { throw new Error('boom'); },
    ));
  });

  it('finishes Fastify reply promises on fulfillment and rejection', async () => {
    const success = lozaFastifyPlugin({ service: 'fastify' });
    let fulfilled = false;
    const reply = Object.assign(Promise.resolve().then(() => { fulfilled = true; }), { statusCode: 200 });
    await success(
      { method: 'GET', url: '/ok', routeOptions: { url: '/users/:id' }, headers: {}, socket: {}, ip: '' },
      reply,
    );
    assert.equal(fulfilled, true);

    const failure = lozaFastifyPlugin();
    const failedReply = Object.assign(Promise.reject(new Error('failed')), { statusCode: 503 });
    failedReply.catch(() => {});
    await failure({ method: 'POST', url: '/bad', headers: {}, socket: {}, ip: '127.0.0.1' }, failedReply);
  });

  it('finishes Hono events on success and thrown errors', async () => {
    const successContext = {
      req: {
        method: 'GET', path: '/ok', routePath: '/route',
        header(name: string) { return name === 'user-agent' ? 'ua' : '10.0.0.1'; },
      },
      res: { status: 200 },
    };
    let called = false;
    await lozaHonoMiddleware({ service: 'hono' })(successContext, async () => { called = true; });
    assert.equal(called, true);
    await lozaHonoMiddleware()(
      { req: { method: 'POST', path: '/bad', header: () => '' }, res: { status: 500 } },
      async () => { throw new Error('bad'); },
    );
  });

  it('returns Next responses and rethrows failures', async () => {
    const middleware = lozaNextMiddleware({ service: 'next' });
    const request = {
      method: '',
      url: '/fallback',
      headers: { get: (name: string) => name === 'user-agent' ? 'ua' : '' },
    };
    const response = { status: 200 };
    assert.equal(await middleware(request, async () => response), response);
    const nextRequest = {
      method: 'POST', nextUrl: { pathname: '/users' },
      headers: { get: () => '10.0.0.2' },
    };
    await assert.rejects(() => middleware(nextRequest, async () => { throw new Error('request failed'); }), /request failed/);
  });
});

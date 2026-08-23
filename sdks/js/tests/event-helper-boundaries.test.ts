import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import * as attrs from '../src/core/event.ts';

describe('Typed attribute helper boundaries', () => {
  it('constructs primitive, semantic, domain, and AI/RAG attrs', () => {
    const values = [
      attrs.String('s', 'v'), attrs.Int('i', 1), attrs.Float64('f', 1), attrs.Bool('b', true),
      attrs.Null('n'), attrs.Any('a', { value: 1 }), attrs.Json('j', { value: 1 }), attrs.Group('g', []),
      attrs.SensitiveString('ss', 'v'), attrs.HashString('hs', 'v'), attrs.MarkSensitive(attrs.String('m', 'v')),
      attrs.Int64('i64', 1), attrs.Uint64('u64', 1), attrs.Time('time', new Date(0)), attrs.Duration('d', 1),
      attrs.UserID('u'), attrs.TenantID('t'), attrs.WorkspaceID('w'), attrs.OrganizationID('o'), attrs.SessionID('s'),
      attrs.RequestID('r'), attrs.TraceID('t'), attrs.SpanID('s'), attrs.IncidentID('i'), attrs.FeatureFlag('f', true),
      attrs.FeatureFlagBool('f', true), attrs.Experiment('e', 'a'), attrs.OrderID('o'), attrs.CartID('c'),
      attrs.ProductID('p'), attrs.CustomerID('c'), attrs.Plan('pro'), attrs.Currency('USD'), attrs.Amount(2),
      attrs.PaymentID('p'), attrs.SubscriptionID('s'), attrs.InvoiceID('i'), attrs.JobID('j'), attrs.MessageID('m'),
      attrs.Percent('percent', 1), attrs.Bytes('bytes', 2), attrs.HttpStatus(200), attrs.HttpStatus('status', 201), attrs.HttpStatus('missing'),
      attrs.StatusCode(200), attrs.StatusCode('status', 201), attrs.StatusCode('missing'), attrs.ErrorCodeExt('code', 'E'), attrs.List('list', 1, 2),
      attrs.MapAttr('map', { key: 'value' }), attrs.Enum('enum', 'value', 'value'), attrs.ID('id', 'id'), attrs.Hash('hash', 'v'),
      attrs.Redacted('redacted'), attrs.AccountID('account'), attrs.DeploymentID('deployment'), attrs.HttpRoute('/route'),
      attrs.HttpMethod('get'), attrs.HttpPath('/path'), attrs.HttpUserAgent('ua'), attrs.HttpReferer('/ref?query=1'),
      attrs.HttpRequest({ method: 'GET', path: '/path', route: { path: '/route' } }),
      attrs.HttpRequest({ httpMethod: 'POST', url: '/string-url', route: '/string-route' }),
      attrs.HttpRequest({ method: 'GET', url: { pathname: '/url' }, urlPattern: '/pattern' }),
      attrs.HttpRequest({}),
      attrs.HttpResponse({ statusCode: 201 }),
      attrs.HttpResponse({ status: 202 }),
      attrs.HttpResponse({}),
      attrs.Bucket('bucket', 'one'), attrs.Tags('tags', 'a', 'b'), attrs.Masked('masked', 'v'), attrs.Url('url', 'v'),
      attrs.EmailHash('email', 'v'), attrs.IpHash('ip', 'v'), attrs.RegionEx('us-east'),
      attrs.PaymentMethod('card'), attrs.PaymentIntentId('intent'), attrs.PaymentFailureCode('declined'), attrs.PaymentRetryAttempt(1),
      attrs.BillingSubscriptionId('sub'), attrs.BillingInvoiceId('invoice'), attrs.BillingAmount(123), attrs.BillingInterval('month'),
      attrs.AgentName('agent'), attrs.AgentProvider('provider'), attrs.AgentRunType('tool'), attrs.AgentToolName('search'),
      attrs.AgentToolOutcome('ok'), attrs.AgentInputTokens(1), attrs.AgentOutputTokens(2), attrs.AgentCost(3),
      attrs.RagEmbeddingModel('model'), attrs.RagChunksRetrieved(2), attrs.RagTopScore(0.9), attrs.RagQueryHash('hash'),
      attrs.RagCitationCount(1), attrs.RagRetrievalLatency(4), attrs.AppVersion('1'), attrs.ErrorType('Error'),
      attrs.ErrorMessage('message'), attrs.ErrorStack('stack'),
    ];
    assert.equal(attrs.HttpMethod('get').value, 'GET');
    assert.equal(attrs.HttpUserAgent('x'.repeat(513)).value.length, 512);
    assert.equal(attrs.HttpReferer('/path?secret=true').value, '/path');
  });

  it('covers event aliases, clock controls, clone, and terminal transitions', () => {
    const previous = attrs.setClock(() => 123);
    try {
      assert.equal(attrs._now(), 123);
      const event = new attrs.Event({ event: 'aliases', service: 'svc' }, 'svc', 'test');
      event.enrich(attrs.String('key', 'value'));
      event.process('process').finish();
      event.timer('timer').stop();
      event.group('group').finish();
      const fullyAttributed = new attrs.Event({
        event: 'full',
        service: 'svc',
        eventId: 'event-id',
        requestId: 'request-id',
        traceId: 'trace-id',
        spanId: 'span-id',
        incidentId: 'incident-id',
        parentId: 'parent-id',
        outcome: 'success',
        custom: [attrs.String('custom', 'value')],
        userId: 'user',
        tenantId: 'tenant',
        workspaceId: 'workspace',
        organizationId: 'organization',
        sessionId: 'session',
        accountId: 'account',
        customerId: 'customer',
        deploymentId: 'deployment',
        region: 'region',
        host: 'host',
        runtime: 'runtime',
        method: 'GET',
        path: '/path',
        route: '/route',
        statusCode: 200,
      }, 'svc', 'test');
      assert.match(fullyAttributed.eventId, /^[0-9a-f-]{36}$/);
      const namedEvent = new attrs.Event({ name: 'named', service: 'svc' }, 'svc', 'test');
      assert.equal(namedEvent.event, 'named');
      assert.equal(new attrs.Event({ event: '', service: 'svc' }, 'svc', 'test').event, '');
      assert.equal(event.clone().event, 'aliases');
      event.finish('success');
      event.markEmitted();
      event.markEmittedDone();
      assert.equal(event.emitted, true);
    } finally {
      attrs.setClock(previous);
      attrs.resetClock();
    }
  });
});

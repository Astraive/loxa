import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers identity and domain helpers', () => {
  assert.equal(loza.userId('u_123').key, 'user.id');
  assert.equal(loza.tenantId('t_123').key, 'tenant.id');
  assert.equal(loza.sessionId('s_123').key, 'session.id');
  assert.equal(loza.requestId('req_123').key, 'request_id');
  assert.equal(loza.traceId('trace_123').key, 'trace_id');
  assert.equal(loza.spanId('span_123').key, 'span_id');
  assert.equal(loza.orderId('ord_123').key, 'order.id');
  assert.equal(loza.cartId('cart_123').key, 'cart.id');
  assert.equal(loza.paymentId('pay_123').key, 'payment.id');
  assert.equal(loza.subscriptionId('sub_123').key, 'subscription.id');
  assert.equal(loza.invoiceId('inv_123').key, 'invoice.id');
  assert.equal(loza.jobId('job_123').key, 'job.id');
  assert.equal(loza.messageId('msg_123').key, 'message.id');
  assert.equal(loza.correlationId('corr_123').key, 'correlation.id');
  assert.equal(loza.commitSha('abc123').key, 'deployment.commit_sha');
  assert.equal(loza.release('1.2.3').key, 'release');
  assert.equal(loza.checkoutCartItemCount(3).key, 'checkout.cart_item_count');
  assert.equal(loza.paymentProvider('stripe').key, 'payment.provider');
  assert.equal(loza.billingPlan('pro').key, 'billing.plan');
  assert.equal(loza.agentModel('gpt-5.5').key, 'agent.model');
  assert.equal(loza.ragIndex('docs').key, 'rag.index');
});

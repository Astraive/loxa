import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers identity and domain helpers', () => {
  assert.equal(loxa.userId('u_123').key, 'user.id');
  assert.equal(loxa.tenantId('t_123').key, 'tenant.id');
  assert.equal(loxa.sessionId('s_123').key, 'session.id');
  assert.equal(loxa.requestId('req_123').key, 'request_id');
  assert.equal(loxa.traceId('trace_123').key, 'trace_id');
  assert.equal(loxa.spanId('span_123').key, 'span_id');
  assert.equal(loxa.orderId('ord_123').key, 'order.id');
  assert.equal(loxa.cartId('cart_123').key, 'cart.id');
  assert.equal(loxa.paymentId('pay_123').key, 'payment.id');
  assert.equal(loxa.subscriptionId('sub_123').key, 'subscription.id');
  assert.equal(loxa.invoiceId('inv_123').key, 'invoice.id');
  assert.equal(loxa.jobId('job_123').key, 'job.id');
  assert.equal(loxa.messageId('msg_123').key, 'message.id');
  assert.equal(loxa.correlationId('corr_123').key, 'correlation.id');
  assert.equal(loxa.commitSha('abc123').key, 'deployment.commit_sha');
  assert.equal(loxa.release('1.2.3').key, 'release');
  assert.equal(loxa.checkoutCartItemCount(3).key, 'checkout.cart_item_count');
  assert.equal(loxa.paymentProvider('stripe').key, 'payment.provider');
  assert.equal(loxa.billingPlan('pro').key, 'billing.plan');
  assert.equal(loxa.agentModel('gpt-5.5').key, 'agent.model');
  assert.equal(loxa.ragIndex('docs').key, 'rag.index');
});

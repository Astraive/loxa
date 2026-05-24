import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers process, group, timer, and stopwatch helpers', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(loxa.test('catalog').withSink(sink));
  const logger = loxa.createLoxa({ service: 'catalog', sink });
  const ctx = logger.startEvent({ event: 'checkout.request', kind: 'http', route: '/checkout' });

  const process = loxa.process(ctx, 'authorize_payment');
  process.finish(loxa.string('payment.status', 'approved'));
  const explicitProcess = loxa.startProcess(ctx, 'reserve_inventory');
  loxa.finishProcess(explicitProcess, loxa.httpStatus(201));
  const failedProcess = loxa.startProcess(ctx, 'notify_customer');
  loxa.finishProcessError(failedProcess, new Error('smtp_down'));

  const grouped = loxa.startGroup(ctx, 'payment_flow_2');
  loxa.finishGroup(grouped, loxa.string('phase', 'ok'));
  const group = loxa.startGroup(ctx, 'payment_flow');
  group.finish(loxa.string('phase', 'done'));
  const failedGroup = loxa.startGroup(ctx, 'payment_flow_failed');
  loxa.finishGroupError(failedGroup, new Error('group_err'));

  const timerAlias = loxa.timer(ctx, 'db.alias');
  loxa.stopTimer(timerAlias, loxa.httpStatus(204));
  const timer = loxa.startTimer(ctx, 'db.lookup');
  timer.stop(loxa.string('cache', 'miss'));

  loxa.withProcess(ctx, 'process.wrap', () => {});
  loxa.withGroup(ctx, 'group.wrap', () => {});
  loxa.withTimer(ctx, 'timer.wrap', () => {});
  const measured = loxa.measure();
  assert.ok(measured.elapsed() >= 0);
  const stopwatch = loxa.stopwatch();
  assert.ok(stopwatch.elapsed() >= 0);
  loxa.step(ctx, 'step.wrap', () => {});
  loxa.phase(ctx, 'phase.wrap', () => {});
  loxa.span(ctx, 'span.wrap', () => {});

  logger.finish(ctx, 'success', loxa.duration('encode.ms', 1));
  const encoded = await logger.emit(ctx);
  const payload = JSON.parse(encoded!);
  assert.ok(payload.processes.length >= 4);
  assert.ok(payload.groups.length >= 4);
  assert.ok(payload.timers.length >= 3);
});

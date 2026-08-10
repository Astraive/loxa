import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers process, group, timer, and stopwatch helpers', async () => {
  const sink = new loza.MemorySink();
  loza.configure(loza.test('catalog').withSink(sink));
  const logger = loza.createLoza({ service: 'catalog', sink });
  const ctx = logger.startEvent({ event: 'checkout.request', kind: 'http', route: '/checkout' });

  const process = loza.process(ctx, 'authorize_payment');
  process.finish(loza.string('payment.status', 'approved'));
  const explicitProcess = loza.startProcess(ctx, 'reserve_inventory');
  loza.finishProcess(explicitProcess, loza.httpStatus(201));
  const failedProcess = loza.startProcess(ctx, 'notify_customer');
  loza.finishProcessError(failedProcess, new Error('smtp_down'));

  const grouped = loza.startGroup(ctx, 'payment_flow_2');
  loza.finishGroup(grouped, loza.string('phase', 'ok'));
  const group = loza.startGroup(ctx, 'payment_flow');
  group.finish(loza.string('phase', 'done'));
  const failedGroup = loza.startGroup(ctx, 'payment_flow_failed');
  loza.finishGroupError(failedGroup, new Error('group_err'));

  const timerAlias = loza.timer(ctx, 'db.alias');
  loza.stopTimer(timerAlias, loza.httpStatus(204));
  const timer = loza.startTimer(ctx, 'db.lookup');
  timer.stop(loza.string('cache', 'miss'));

  loza.withProcess(ctx, 'process.wrap', () => {});
  loza.withGroup(ctx, 'group.wrap', () => {});
  loza.withTimer(ctx, 'timer.wrap', () => {});
  const measured = loza.measure();
  assert.ok(measured.elapsed() >= 0);
  const stopwatch = loza.stopwatch();
  assert.ok(stopwatch.elapsed() >= 0);
  loza.step(ctx, 'step.wrap', () => {});
  loza.phase(ctx, 'phase.wrap', () => {});
  loza.span(ctx, 'span.wrap', () => {});

  logger.finish(ctx, 'success', loza.duration('encode.ms', 1));
  const encoded = await logger.emit(ctx);
  const payload = JSON.parse(encoded!);
  assert.ok(payload.processes.length >= 4);
  assert.ok(payload.groups.length >= 4);
  assert.ok(payload.timers.length >= 3);
});

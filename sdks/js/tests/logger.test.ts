import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Logger } from '../src/core/logger.ts';
import { MemorySink, HTTPBatchSink } from '../src/sinks/standard-sinks.ts';
import { String as AttrString, SensitiveString } from '../src/core/event.ts';

describe('Logger', () => {
  it('creates logger with config', () => {
    const loxa = new Logger({ service: 'test' });
    const cfg = loxa.getConfig();
    assert.equal(cfg.service, 'test');
  });

  it('startEvent creates event in context', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'payment.completed' });
    assert.equal(ctx.event, 'payment.completed');
    assert.equal(ctx.service, 'checkout');
  });

  it('enrich adds attrs to event', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.enrich(ctx, AttrString('key', 'value'));
    assert.equal(ctx.attrs['key'], 'value');
  });

  it('append adds attrs to event', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.append(ctx, AttrString('key', 'value'));
    assert.equal(ctx.attrs['key'], 'value');
  });

  it('set/get/delete/getGroup work on event', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.set(ctx, 'user.id', 'u123');
    loxa.set(ctx, 'user.name', 'Alice');
    assert.equal(loxa.get(ctx, 'user.id'), 'u123');
    const group = loxa.getGroup(ctx, 'user');
    assert.equal(group.id, 'u123');
    assert.equal(group.name, 'Alice');
    loxa.delete(ctx, 'user.id');
    assert.equal(loxa.get(ctx, 'user.id'), undefined);
  });

  it('finish sets outcome and duration', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.finish(ctx, 'success');
    assert.equal(ctx.outcome, 'success');
    assert.ok(ctx.durationMs >= 0);
  });

  it('emit delivers to sink', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({ service: 'checkout', sink });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.finish(ctx, 'success');
    const encoded = await loxa.emit(ctx);
    assert.ok(encoded);
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.event, 'test');
    assert.equal(parsed.outcome, 'success');
  });

  it('emit is idempotent', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({ service: 'checkout', sink });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.finish(ctx, 'success');
    const first = await loxa.emit(ctx);
    const second = await loxa.emit(ctx);
    assert.ok(first);
    assert.equal(second, null);
    assert.equal(sink.getLength(), 1);
  });

  it('runEvent wraps lifecycle', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({ service: 'checkout', sink });
    await loxa.runEvent({ event: 'test' }, (ctx) => {
      loxa.enrich(ctx, AttrString('key', 'value'));
    });
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.key, 'value');
    assert.equal(parsed.outcome, 'success');
  });

  it('supports process/group/timer compatibility methods', () => {
    const loxa = new Logger({ service: 'checkout' });
    const ctx = loxa.startEvent({ event: 'test' });
    const process = loxa.startProcess(ctx, 'step');
    loxa.finishProcess(process, AttrString('k', 'v'));
    const failedProcess = loxa.process(ctx, 'step.error');
    loxa.finishProcessError(failedProcess, new Error('boom'));
    const group = loxa.group(ctx, 'phase');
    loxa.finishGroup(group, AttrString('g', 'ok'));
    const failedGroup = loxa.startGroup(ctx, 'phase.error');
    loxa.finishGroupError(failedGroup, new Error('group-fail'));
    const timer = loxa.timer(ctx, 'lookup');
    loxa.stopTimer(timer, AttrString('cache', 'hit'));
    assert.equal(ctx.processes.length, 2);
    assert.equal(ctx.groups.length, 2);
    assert.equal(ctx.timers.length, 1);
  });

  it('sampler drops events', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({
      service: 'checkout',
      sink,
      sampler: () => false, // drop all
    });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);
    assert.equal(sink.getLength(), 0);
  });

  it('redactor transforms payload', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({
      service: 'checkout',
      sink,
      redactor: (payload) => {
        const p = { ...payload };
        if (p.attrs) p.attrs = { ...p.attrs, password: '[REDACTED]' };
        return p;
      },
    });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.enrich(ctx, AttrString('password', 'secret123'));
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.password, '[REDACTED]');
  });

  it('sensitive attrs are redacted in emit output', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({ service: 'checkout', sink });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.enrich(ctx, SensitiveString('secret_key', 'my-secret-value'));
    loxa.enrich(ctx, AttrString('normal_key', 'visible'));
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.secret_key, '[REDACTED]');
    assert.equal(parsed.attrs.normal_key, 'visible');
  });

  it('auto-creates HTTPBatchSink when collectorUrl is set', () => {
    const loxa = new Logger({
      service: 'checkout',
      collectorUrl: 'http://localhost:9090/events',
    });
    const cfg = loxa.getConfig();
    assert.equal(cfg.collectorUrl, 'http://localhost:9090/events');
  });

  it('explicit sink takes precedence over collectorUrl', async () => {
    const sink = new MemorySink();
    const loxa = new Logger({
      service: 'checkout',
      sink,
      collectorUrl: 'http://localhost:9090/events',
    });
    const ctx = loxa.startEvent({ event: 'test' });
    loxa.finish(ctx, 'success');
    await loxa.emit(ctx);
    assert.equal(sink.getLength(), 1, 'explicit sink should receive events');
  });

  it('shutdown aliases close', async () => {
    const loxa = new Logger({ service: 'test' });
    await loxa.shutdown(); // should not throw
  });
});

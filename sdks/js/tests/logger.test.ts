import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Logger } from '../src/core/logger.ts';
import { MemorySink, HTTPBatchSink } from '../src/sinks/standard-sinks.ts';
import { String as AttrString, SensitiveString } from '../src/core/event.ts';

describe('Logger', () => {
  it('creates logger with config', () => {
    const loza = new Logger({ service: 'test' });
    const cfg = loza.getConfig();
    assert.equal(cfg.service, 'test');
  });

  it('startEvent creates event in context', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'payment.completed' });
    assert.equal(ctx.event, 'payment.completed');
    assert.equal(ctx.service, 'checkout');
  });

  it('enrich adds attrs to event', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'test' });
    loza.enrich(ctx, AttrString('key', 'value'));
    assert.equal(ctx.attrs['key'], 'value');
  });

  it('append adds attrs to event', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'test' });
    loza.append(ctx, AttrString('key', 'value'));
    assert.equal(ctx.attrs['key'], 'value');
  });

  it('set/get/delete/getGroup work on event', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'test' });
    loza.set(ctx, 'user.id', 'u123');
    loza.set(ctx, 'user.name', 'Alice');
    assert.equal(loza.get(ctx, 'user.id'), 'u123');
    const group = loza.getGroup(ctx, 'user');
    assert.equal(group.id, 'u123');
    assert.equal(group.name, 'Alice');
    loza.delete(ctx, 'user.id');
    assert.equal(loza.get(ctx, 'user.id'), undefined);
  });

  it('finish sets outcome and duration', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'test' });
    loza.finish(ctx, 'success');
    assert.equal(ctx.outcome, 'success');
    assert.ok(ctx.durationMs >= 0);
  });

  it('emit delivers to sink', async () => {
    const sink = new MemorySink();
    const loza = new Logger({ service: 'checkout', sink });
    const ctx = loza.startEvent({ event: 'test' });
    loza.finish(ctx, 'success');
    const encoded = await loza.emit(ctx);
    assert.ok(encoded);
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.event, 'test');
    assert.equal(parsed.outcome, 'success');
  });

  it('emit is idempotent', async () => {
    const sink = new MemorySink();
    const loza = new Logger({ service: 'checkout', sink });
    const ctx = loza.startEvent({ event: 'test' });
    loza.finish(ctx, 'success');
    const first = await loza.emit(ctx);
    const second = await loza.emit(ctx);
    assert.ok(first);
    assert.equal(second, null);
    assert.equal(sink.getLength(), 1);
  });

  it('runEvent wraps lifecycle', async () => {
    const sink = new MemorySink();
    const loza = new Logger({ service: 'checkout', sink });
    await loza.runEvent({ event: 'test' }, (ctx) => {
      loza.enrich(ctx, AttrString('key', 'value'));
    });
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.key, 'value');
    assert.equal(parsed.outcome, 'success');
  });

  it('supports process/group/timer compatibility methods', () => {
    const loza = new Logger({ service: 'checkout' });
    const ctx = loza.startEvent({ event: 'test' });
    const process = loza.startProcess(ctx, 'step');
    loza.finishProcess(process, AttrString('k', 'v'));
    const failedProcess = loza.process(ctx, 'step.error');
    loza.finishProcessError(failedProcess, new Error('boom'));
    const group = loza.group(ctx, 'phase');
    loza.finishGroup(group, AttrString('g', 'ok'));
    const failedGroup = loza.startGroup(ctx, 'phase.error');
    loza.finishGroupError(failedGroup, new Error('group-fail'));
    const timer = loza.timer(ctx, 'lookup');
    loza.stopTimer(timer, AttrString('cache', 'hit'));
    assert.equal(ctx.processes.length, 2);
    assert.equal(ctx.groups.length, 2);
    assert.equal(ctx.timers.length, 1);
  });

  it('sampler drops events', async () => {
    const sink = new MemorySink();
    const loza = new Logger({
      service: 'checkout',
      sink,
      sampler: () => false, // drop all
    });
    const ctx = loza.startEvent({ event: 'test' });
    loza.finish(ctx, 'success');
    await loza.emit(ctx);
    assert.equal(sink.getLength(), 0);
  });

  it('redactor transforms payload', async () => {
    const sink = new MemorySink();
    const loza = new Logger({
      service: 'checkout',
      sink,
      redactor: (payload) => {
        const p = { ...payload };
        if (p.attrs) p.attrs = { ...p.attrs, password: '[REDACTED]' };
        return p;
      },
    });
    const ctx = loza.startEvent({ event: 'test' });
    loza.enrich(ctx, AttrString('password', 'secret123'));
    loza.finish(ctx, 'success');
    await loza.emit(ctx);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.password, '[REDACTED]');
  });

  it('sensitive attrs are redacted in emit output', async () => {
    const sink = new MemorySink();
    const loza = new Logger({ service: 'checkout', sink });
    const ctx = loza.startEvent({ event: 'test' });
    loza.enrich(ctx, SensitiveString('secret_key', 'my-secret-value'));
    loza.enrich(ctx, AttrString('normal_key', 'visible'));
    loza.finish(ctx, 'success');
    await loza.emit(ctx);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.attrs.secret_key, '[REDACTED]');
    assert.equal(parsed.attrs.normal_key, 'visible');
  });

  it('auto-creates HTTPBatchSink when collectorUrl is set', () => {
    const loza = new Logger({
      service: 'checkout',
      collectorUrl: 'http://localhost:9308/events',
    });
    const cfg = loza.getConfig();
    assert.equal(cfg.collectorUrl, 'http://localhost:9308/events');
  });

  it('explicit sink takes precedence over collectorUrl', async () => {
    const sink = new MemorySink();
    const loza = new Logger({
      service: 'checkout',
      sink,
      collectorUrl: 'http://localhost:9308/events',
    });
    const ctx = loza.startEvent({ event: 'test' });
    loza.finish(ctx, 'success');
    await loza.emit(ctx);
    assert.equal(sink.getLength(), 1, 'explicit sink should receive events');
  });

  it('shutdown aliases close', async () => {
    const loza = new Logger({ service: 'test' });
    await loza.shutdown(); // should not throw
  });
});

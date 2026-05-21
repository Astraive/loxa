import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import {
  configure, reset, startEvent, append, enrich, finish, finishError, emit,
  checkpoint, set, get, del, getGroup, merge, shutdown,
  defaultLogger, Logger, MemorySink,
  production, dev, test as testPreset,
  stdoutSink, memorySink, noopSink,
  userId, cartId, featureFlag, int,
  EventView, ConfigBuilder,
} from '../src/index.ts';

describe('Facade', () => {
  afterEach(() => {
    reset();
  });

  it('configure + startEvent + append + finish + emit works', async () => {
    const sink = memorySink();
    configure(production('checkout').withSink(sink));

    const ctx = startEvent({ event: 'checkout.request' });
    append(ctx, userId('u_123'), cartId('cart_456'));
    finish(ctx, 'success', int('status_code', 200));
    const encoded = await emit(ctx);

    assert.ok(encoded);
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.event, 'checkout.request');
    assert.equal(parsed.outcome, 'success');
    assert.equal(parsed.user.id, 'u_123');
    assert.equal(parsed.attrs.cart.id, 'cart_456');
    assert.equal(parsed.attrs.status_code, 200);
  });

  it('enrich works like append', async () => {
    const sink = memorySink();
    configure(dev('test').withSink(sink));

    const ctx = startEvent({ event: 'test' });
    enrich(ctx, userId('u1'));
    finish(ctx, 'success');
    await emit(ctx);

    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.user.id, 'u1');
  });

  it('set/get/delete/merge/getGroup work', () => {
    configure(dev('test'));
    const ctx = startEvent({ event: 'test' });
    set(ctx, 'user.id', 'u123');
    set(ctx, 'user.name', 'Alice');
    assert.equal(get(ctx, 'user.id'), 'u123');
    const group = getGroup(ctx, 'user');
    assert.equal(group.id, 'u123');
    assert.equal(group.name, 'Alice');
    del(ctx, 'user.id');
    assert.equal(get(ctx, 'user.id'), undefined);
    merge(ctx, { 'extra': 'value' });
    assert.equal(get(ctx, 'extra'), 'value');
  });

  it('checkpoint works', async () => {
    const sink = memorySink();
    configure(dev('test').withSink(sink));

    const ctx = startEvent({ event: 'test' });
    checkpoint(ctx, 'step1');
    checkpoint(ctx, 'step2', { key: 'value' });
    finish(ctx, 'success');
    await emit(ctx);

    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.checkpoints.length, 2);
    assert.equal(parsed.checkpoints[0].name, 'step1');
    assert.equal(parsed.checkpoints[1].name, 'step2');
  });

  it('finishError works', async () => {
    const sink = memorySink();
    configure(dev('test').withSink(sink));

    const ctx = startEvent({ event: 'test' });
    finishError(ctx, new Error('boom'));
    await emit(ctx);

    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.outcome, 'error');
    assert.ok(parsed.error);
    assert.equal(parsed.error.message, 'boom');
  });

  it('shutdown works', async () => {
    configure(dev('test'));
    await shutdown(); // should not throw
  });

  it('defaultLogger returns Logger instance', () => {
    configure(dev('test'));
    const logger = defaultLogger();
    assert.ok(logger instanceof Logger);
  });

  it('production preset supports fluent builder', () => {
    const cfg = production('checkout')
      .withVersion('1.2.0')
      .withEnvironment('prod')
      .withSink(memorySink())
      .build();
    assert.equal(cfg.service, 'checkout');
    assert.equal(cfg.version, '1.2.0');
    assert.equal(cfg.environment, 'prod');
  });

  it('ConfigBuilder implements Config', () => {
    const builder = dev('test');
    assert.ok(builder instanceof ConfigBuilder);
    // Can be passed directly to configure()
    configure(builder);
    const logger = defaultLogger();
    assert.equal(logger.getConfig().service, 'test');
  });

  it('sink factories work', () => {
    assert.equal(stdoutSink().name(), 'stdout');
    assert.equal(noopSink().name(), 'noop');
    assert.equal(memorySink().name(), 'memory');
  });

  it('EventView is exported', () => {
    assert.ok(EventView);
  });
});

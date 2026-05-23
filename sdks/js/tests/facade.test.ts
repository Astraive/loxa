import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import {
  loxa, configure, reset, startEvent, append, enrich, finish, finishError, emit,
  checkpoint, set, get, del, getGroup, merge, shutdown,
  defaultLogger, Logger, MemorySink,
  production, dev, test as testPreset,
  stdoutSink, memorySink, noopSink,
  userId, cartId, featureFlag, int,
  EventView, ConfigBuilder,
  createLoxa, alias, New,
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

describe('loxa default instance', () => {
  afterEach(() => {
    reset();
  });

  it('loxa is exported as a Logger instance', () => {
    assert.ok(loxa instanceof Logger);
  });

  it('loxa.configure + loxa.startEvent + loxa.finish + loxa.emit works', async () => {
    const sink = memorySink();
    configure(production('checkout').withSink(sink));

    const ctx = loxa.startEvent({ event: 'checkout.request' });
    loxa.append(ctx, userId('u_123'));
    loxa.finish(ctx, 'success', int('status_code', 200));
    const encoded = await loxa.emit(ctx);

    assert.ok(encoded);
    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.event, 'checkout.request');
    assert.equal(parsed.outcome, 'success');
    assert.equal(parsed.user.id, 'u_123');
  });

  it('loxa.info works for immediate logs', async () => {
    const sink = memorySink();
    configure(dev('test').withSink(sink));

    await loxa.info('server started');

    assert.equal(sink.getLength(), 1);
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.event, 'server started');
  });

  it('loxa.createLoxa returns independent instance', () => {
    const logger = createLoxa({ service: 'custom' });
    assert.ok(logger instanceof Logger);
    assert.equal(logger.getConfig().service, 'custom');
    assert.notEqual(logger, loxa);
  });

  it('loxa.alias creates same-service child with alias metadata', () => {
    configure(production('api').withSink(memorySink()));
    const audit = loxa.alias('audit');
    assert.ok(audit instanceof Logger);
    assert.equal(audit.getConfig().service, 'api');
    assert.equal(audit.getConfig().alias, 'audit');
    assert.notEqual(audit, loxa);
  });

  it('configure updates loxa instance in-place', () => {
    configure(dev('first'));
    assert.equal(loxa.getConfig().service, 'first');
    configure(dev('second'));
    assert.equal(loxa.getConfig().service, 'second');
  });
});

describe('createLoxa and alias', () => {
  afterEach(() => {
    reset();
  });

  it('createLoxa returns independent Logger', () => {
    const logger = createLoxa({ service: 'test-svc' });
    assert.ok(logger instanceof Logger);
    assert.equal(logger.getConfig().service, 'test-svc');
  });

  it('alias creates same-service Logger with alias metadata', () => {
    configure(production('api').withSink(memorySink()));
    const audit = alias('audit');
    assert.ok(audit instanceof Logger);
    assert.equal(audit.getConfig().service, 'api');
    assert.equal(audit.getConfig().alias, 'audit');
  });

  it('Logger.alias preserves service and does not mutate original', () => {
    const logger = new Logger({ service: 'api' });
    const child = logger.alias('child-svc');
    assert.equal(child.getConfig().service, 'api');
    assert.equal(child.getConfig().alias, 'child-svc');
    assert.equal(logger.getConfig().service, 'api');
    assert.equal(logger.getConfig().alias, '');
  });

  it('alias emits loxa.alias without changing service', async () => {
    const sink = memorySink();
    const logger = new Logger({ service: 'api', sink });
    const audit = logger.alias('audit');
    await audit.info('permission changed');
    const parsed = JSON.parse(sink.getEvents()[0]);
    assert.equal(parsed.service, 'api');
    assert.equal(parsed.attrs.loxa.alias, 'audit');
  });
});

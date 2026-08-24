import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  Logger, New, bindEvent, wrap, fromRequest, run, TryNew, Default,
  configure, reset,
} from '../src/core/logger.ts';
import { Event, String, Int } from '../src/core/event.ts';
import { MemorySink } from '../src/sinks/standard-sinks.ts';
import { sampleNone } from '../src/sampling/sampler.ts';
import { redact } from '../src/redaction/redactor.ts';

class FullSink {
  events: string[] = [];
  paused = false;
  name() { return 'full'; }
  write(encoded: string) { this.events.push(encoded); }
  flush() {}
  close() {}
  drain() {}
  pause() { this.paused = true; }
  resume() { this.paused = false; }
  queueSize() { return this.events.length; }
  health() { return this.events.length >= 0; }
}

class FailingSink {
  name() { return 'failing'; }
  write() { throw new Error('write failed'); }
  flush() {}
  close() {}
}

describe('Logger boundaries', () => {
  it('covers child aliases, event accessors, timing handles, and sink controls', async () => {
    const sink = new FullSink();
    const logger = new Logger({ service: 'svc', sink });
    const child = logger.child({ alias: 'child', version: '1' });
    const aliased = logger.alias('alias');
    assert.equal(child.getConfig().alias, 'child');
    assert.equal(aliased.getConfig().alias, 'alias');
    const ctx = logger.startEvent({ event: 'event', custom: [String('custom', 'value')] });
    logger.enrich(ctx, String('k.key', 'value'));
    logger.append(ctx, String('append', 'value'));
    logger.set(ctx, 'set', true);
    logger.merge(ctx, { merged: 1 });
    assert.equal(logger.get(ctx, 'k.key'), 'value');
    assert.deepEqual(logger.getGroup(ctx, 'k'), { key: 'value' });
    logger.delete(ctx, 'set');
    const process = logger.startProcess(ctx, 'process');
    logger.finishProcess(process, Int('status_code', 200));
    const group = logger.startGroup(ctx, 'group');
    logger.finishGroupError(group, new Error('group error'));
    const timer = logger.startTimer(ctx, 'timer');
    logger.stopTimer(timer);
    logger.finish(ctx, 'success');
    const encoded = await logger.emit(ctx);
    assert.ok(encoded);
    assert.equal(sink.events.length, 1);
    assert.equal(logger.currentEvent(), ctx);
    await logger.flush();
    await logger.drain();
    logger.pause();
    assert.equal(sink.paused, true);
    logger.resume();
    assert.equal(sink.paused, false);
    assert.equal(logger.queueSize(), 1);
    assert.equal(await logger.health(), true);
    await logger.close();
    await logger.shutdown();
  });

  it('covers runEvent, run, lifecycle outcomes, logging helpers, and cloning', async () => {
    const sink = new MemorySink();
    const logger = new Logger({ service: 'svc', sink, redactor: redact('secret') });
    await logger.debug('debug');
    await logger.info('info');
    await logger.notice('notice');
    await logger.warn('warn');
    await logger.error('error');
    await logger.fatal('fatal');
    await logger.event('event');
    await logger.track('track');
    await logger.audit('audit');
    await logger.security('security');
    await logger.metric('metric', 1);
    await logger.count('count', 2);
    await logger.gauge('gauge', 3);
    await logger.histogram('histogram', 4);
    await logger.breadcrumb('breadcrumb');

    await logger.runEvent({ event: 'success' }, event => { event.enrich(String('secret', 'value')); });
    await logger.runEvent({ event: 'failure' }, () => { throw new Error('failure'); });
    const finished = logger.startEvent({ event: 'finished' });
    finished.finish('success');
    await logger.runEvent({ event: 'already' }, event => { event.finish('success'); });
    await logger.run(finished, () => {});
    const existingError = logger.startEvent({ event: 'existing-error' });
    await logger.run(existingError, () => { throw new Error('run failure'); });

    const source = logger.startEvent({ event: 'source', traceId: 'trace' });
    const cloned = logger.cloneEvent(source);
    const linked = logger.linkEvent(source, 'linked', String('key', 'value'));
    assert.notEqual(cloned, source);
    assert.equal(linked.event, 'linked');
    await logger.drop(logger.startEvent({ event: 'drop' }), 'test');
    await logger.cancel(logger.startEvent({ event: 'cancel' }), 'test');
    await logger.abandon(logger.startEvent({ event: 'abandon' }), 'test');
    await logger.retry(logger.startEvent({ event: 'retry' }));
    await logger.partial(logger.startEvent({ event: 'partial' }));
    assert.ok(sink.getLength() >= 20);
  });

  it('covers request extraction and static/global helper paths', async () => {
    reset();
    const sink = new MemorySink();
    const logger = new Logger({ service: 'svc', sink });
    const requestEvent = fromRequest({
      method: 'post', path: '/checkout', route: { path: '/checkout/:id' },
      headers: {
        'x-request-id': 'req', 'traceparent': 'trace', 'user-agent': 'ua', 'referer': '/from?x=1',
      },
    }, logger);
    assert.equal(requestEvent.method, 'POST');
    assert.equal(requestEvent.route, '/checkout/:id');
    const fallbackEvent = fromRequest({ httpMethod: 'get', url: '/fallback', headers: { referrer: '/ref?x=1' } }, logger);
    assert.equal(fallbackEvent.path, '/fallback');
    await run(requestEvent, () => {});
    await run(fallbackEvent, () => { throw 'primitive'; });
    await Logger.bindEvent({ event: 'bound' }, () => {});
    await Logger.wrap('wrapped', () => {});
    await bindEvent({ event: 'bound-function' }, () => {});
    await wrap('wrapped-function', () => {});
    assert.ok(New({ service: 'new' }) instanceof Logger);
    assert.ok(TryNew({ service: 'try' }) instanceof Logger);
    configure(logger.getConfig());
    assert.ok(Default() instanceof Logger);
    reset();
  });

  it('marks sink delivery failures and handles sampler drops and no sinks', async () => {
    const failingEventLogger = new Logger({ service: 'svc', sink: new FailingSink() });
    const failed = failingEventLogger.startEvent({ event: 'failed' });
    failed.finish('success');
    assert.equal(await failingEventLogger.emit(failed), null);
    assert.equal(failed.getEventState(), 'delivery_failed');
    const droppedLogger = new Logger({ sampler: sampleNone(), sink: new MemorySink() });
    const dropped = droppedLogger.startEvent({ event: 'dropped' });
    dropped.finish('success');
    assert.equal(await droppedLogger.emit(dropped), null);
    const noSink = new Logger();
    assert.equal(await noSink.health(), true);
    await noSink.flush();
    await noSink.close();
    await noSink.drain();
    noSink.pause();
    noSink.resume();
    assert.equal(noSink.queueSize(), 0);
  });
});

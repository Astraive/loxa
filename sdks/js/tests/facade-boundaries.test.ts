import { afterEach, describe, it } from 'node:test';
import assert from 'node:assert/strict';
import * as facade from '../src/loza.ts';
import { test as testConfig } from '../src/config/config.ts';
import { MemorySink } from '../src/sinks/standard-sinks.ts';
import { String, Int } from '../src/core/event.ts';

afterEach(() => {
  facade.reset();
});

describe('Loza facade boundaries', () => {
  it('covers event lifecycle, timing, and mutation wrappers', async () => {
    const sink = new MemorySink();
    facade.configure(testConfig('facade').withSink(sink));
    const event = facade.startEvent({ event: 'facade.event' });
    facade.append(event, String('k.key', 'value'));
    facade.enrich(event, String('other', 'value'));
    facade.set(event, 'set', true);
    facade.merge(event, { merged: 1 });
    facade.del(event, 'set');
    assert.equal(facade.get(event, 'other'), 'value');
    assert.deepEqual(facade.getGroup(event, 'k'), { key: 'value' });
    facade.checkpoint(event, 'start');
    const process = facade.process(event, 'process');
    facade.finishProcess(process, Int('status_code', 200));
    const processAlias = facade.startProcess(event, 'process-alias');
    facade.finishProcessError(processAlias, new Error('process error'));
    const group = facade.group(event, 'group');
    facade.finishGroup(group, Int('status_code', 201));
    const groupAlias = facade.startGroup(event, 'group-alias');
    facade.finishGroupError(groupAlias, new Error('group error'));
    const timer = facade.timer(event, 'timer');
    facade.stopTimer(timer);
    const timerAlias = facade.startTimer(event, 'timer-alias');
    facade.stopTimer(timerAlias);
    facade.finish(event, 'success');
    assert.ok(await facade.emit(event));
    assert.equal(facade.currentEvent(), event);
    assert.equal(facade.queueSize(), 0);
    assert.equal(await facade.health(), true);
    await facade.flush();
    await facade.drain();
    facade.pause();
    facade.resume();
    await facade.shutdown();
    assert.ok(facade.defaultLogger());
    assert.ok(facade.createLoza({ service: 'child' }));
    assert.ok(facade.alias('alias'));
  });

  it('covers immediate, run, lifecycle, clone/link, and event-kind wrappers', async () => {
    facade.configure(testConfig('facade').withSink(new MemorySink()));
    const kinds = [
      facade.startHttpEvent({ event: 'http' }),
      facade.startJobEvent({ event: 'job' }),
      facade.startQueueEvent({ event: 'queue' }),
      facade.startCliEvent({ event: 'cli' }),
      facade.startCronEvent({ event: 'cron' }),
    ];
    for (const event of kinds) {
      facade.finish(event, 'success');
      await facade.emit(event);
    }
    await facade.debug('debug');
    await facade.info('info');
    await facade.notice('notice');
    await facade.warn('warn');
    await facade.error('error');
    await facade.fatal('fatal');
    await facade.event('event');
    await facade.track('track');
    await facade.audit('audit');
    await facade.security('security');
    await facade.metric('metric', 1);
    await facade.count('count', 2);
    await facade.gauge('gauge', 3);
    await facade.histogram('histogram', 4);
    await facade.breadcrumb('breadcrumb');
    await facade.drop(facade.startEvent({ event: 'drop' }), 'reason');
    await facade.cancel(facade.startEvent({ event: 'cancel' }), 'reason');
    await facade.abandon(facade.startEvent({ event: 'abandon' }), 'reason');
    await facade.retry(facade.startEvent({ event: 'retry' }));
    await facade.partial(facade.startEvent({ event: 'partial' }));
    const source = facade.startEvent({ event: 'source', traceId: 'trace' });
    assert.notEqual(facade.cloneEvent(source), source);
    assert.equal(facade.linkEvent(source, 'linked', String('x', 'y')).event, 'linked');
    await facade.runEvent({ event: 'run-event' }, () => {});
    const runContext = facade.startEvent({ event: 'run' });
    await facade.run(runContext, () => {});
    const errorContext = facade.startEvent({ event: 'run-error' });
    await facade.run(errorContext, () => { throw new Error('run error'); });
    assert.ok(facade.sanitizeEvent(source));
    assert.ok(facade.SecurityLimiter);
  });
});

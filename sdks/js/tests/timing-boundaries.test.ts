import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { Event, String, Int, setClock } from '../src/core/event.ts';
import {
  ProcessHandle, TimerHandle, GroupHandle, withProcess, withGroup, withTimer,
  measure, phase, span, step, stopwatch,
} from '../src/core/timing.ts';

function withClock(run: (setNow: (value: number) => void) => void): void {
  const originalNow = Date.now;
  let now = 1000;
  const previousClock = setClock(() => now);
  Date.now = () => now;
  try {
    run(value => { now = value; });
  } finally {
    Date.now = originalNow;
    setClock(previousClock);
  }
}

describe('Timing boundaries', () => {
  it('records process, group, and timer entries with attrs and status codes', () => {
    withClock(setNow => {
      const event = new Event({ event: 'timed', service: 'svc' }, 'svc', 'test');
      setNow(1010);
      const process = event.startProcess('step');
      assert.equal(process instanceof ProcessHandle, true);
      setNow(1030);
      process.finish(String('detail', 'value'), Int('status_code', 201));
      const group = event.startGroup('phase');
      assert.equal(group instanceof GroupHandle, true);
      setNow(1050);
      group.finish(String('detail', 'group'), Int('status_code', 202));
      const timer = event.startTimer('span');
      assert.equal(timer instanceof TimerHandle, true);
      setNow(1080);
      timer.stop(String('detail', 'timer'), Int('status_code', 203));
      assert.equal(process.duration(), 70);
      assert.equal(group.duration(), 50);
      assert.equal(timer.duration(), 30);
      assert.equal(event.processes[0].status_code, 201);
      assert.equal(event.processes[0].attrs?.detail, 'value');
      assert.equal(event.groups[0].status_code, 202);
      assert.equal(event.timers[0].status_code, 203);
      timer.stopTimer();
      assert.equal(event.timers.length, 2);
    });
  });

  it('records errors and supports timing aliases and standalone stopwatches', () => {
    withClock(setNow => {
      const event = new Event({ event: 'timed', service: 'svc' }, 'svc', 'test');
      setNow(1010);
      step(event, 'step').finishError('failed');
      phase(event, 'phase').finishGroupError(new Error('group failed'));
      span(event, 'span').stop();
      withProcess(event, 'wrapped', () => {}, String('k', 'v'));
      withGroup(event, 'wrapped-group', () => {});
      withTimer(event, 'wrapped-timer', () => {});
      assert.equal(event.processes.length, 2);
      assert.equal(event.groups.length, 2);
      assert.equal(event.timers.length, 2);
      assert.throws(() => withProcess(event, 'bad', () => { throw new Error('process'); }), /process/);
      assert.throws(() => withGroup(event, 'bad-group', () => { throw new Error('group'); }), /group/);
      assert.throws(() => withTimer(event, 'bad-timer', () => { throw new Error('timer'); }), /timer/);

      setNow(1050);
      const measured = measure();
      const stopped = stopwatch();
      setNow(1060);
      assert.equal(measured.elapsed(), 10);
      assert.equal(stopped.elapsed(), 10);
    });
  });
  it('handles timing adapters with empty event state and omitted attrs', () => {
    withClock(setNow => {
      const processEvent: Record<string, any> = {};
      const process = new ProcessHandle(processEvent, 'empty-process', 1, 1000);
      setNow(1010);
      process.finish(null, { key: 'status_code', value: 0 });
      assert.equal(processEvent.processes.length, 1);

      const groupEvent: Record<string, any> = {};
      const group = new GroupHandle(groupEvent, 'empty-group', 1000);
      setNow(1020);
      group.finish(null, { key: 'status_code', value: 0 });
      assert.equal(groupEvent.groups.length, 1);

      const timerEvent: Record<string, any> = {};
      const timer = new TimerHandle(timerEvent, 'empty-timer', 1000);
      setNow(1030);
      timer.stop(null, { key: 'status_code', value: 0 });
      assert.equal(timerEvent.timers.length, 1);
    });
  });
});

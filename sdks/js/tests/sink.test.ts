import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { MemorySink, NoopSink } from '../src/sinks/standard-sinks.ts';

describe('Sinks', () => {
  it('MemorySink captures events', () => {
    const sink = new MemorySink();
    sink.write('{"event":"test1"}');
    sink.write('{"event":"test2"}');
    assert.equal(sink.getLength(), 2);
    assert.deepEqual(sink.getEvents(), [
      '{"event":"test1"}',
      '{"event":"test2"}',
    ]);
  });

  it('MemorySink clear resets', () => {
    const sink = new MemorySink();
    sink.write('{"event":"test"}');
    sink.clear();
    assert.equal(sink.getLength(), 0);
  });

  it('NoopSink discards events', () => {
    const sink = new NoopSink();
    sink.write('{"event":"test"}');
    assert.equal(sink.name(), 'noop');
  });
});

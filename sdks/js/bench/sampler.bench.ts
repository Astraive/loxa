import { bench, describe } from 'vitest';
import {
  Logger, production, memorySink,
  sampleErrors, sampleRandom, sampleAll,
} from '../src';

describe('sampler', () => {
  const errorsLogger = new Logger(
    production('bench').withSink(memorySink()).withSampler(sampleErrors())
  );
  const randomLogger = new Logger(
    production('bench').withSink(memorySink()).withSampler(sampleRandom(0.5))
  );
  const allLogger = new Logger(
    production('bench').withSink(memorySink()).withSampler(sampleAll())
  );

  bench('SampleErrors - error event', () => {
    const ctx = errorsLogger.startEvent({ event: 'bench.sample-error' });
    errorsLogger.finishError(ctx, new Error('boom'));
    errorsLogger.emit(ctx);
  });

  bench('SampleErrors - success event', () => {
    const ctx = errorsLogger.startEvent({ event: 'bench.sample-ok' });
    errorsLogger.finish(ctx, 'success');
    errorsLogger.emit(ctx);
  });

  bench('SampleRandom(0.5)', () => {
    const ctx = randomLogger.startEvent({ event: 'bench.sample-random' });
    randomLogger.finish(ctx, 'success');
    randomLogger.emit(ctx);
  });

  bench('SampleAll', () => {
    const ctx = allLogger.startEvent({ event: 'bench.sample-all' });
    allLogger.finish(ctx, 'success');
    allLogger.emit(ctx);
  });
});

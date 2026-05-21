import { bench, describe } from 'vitest';
import { Logger, production, memorySink } from '../src';

describe('emit', () => {
  const logger = new Logger(production('bench').withSink(memorySink()));

  bench('emit cycle', () => {
    const ctx = logger.startEvent({ event: 'bench.test', kind: 'http' });
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });
});

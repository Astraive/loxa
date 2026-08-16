import { bench, describe } from 'vitest';
import { createLoza, production, memorySink } from '../src';

describe('emit', () => {
  const logger = createLoza(production('bench').withSink(memorySink()));

  bench('emit cycle', () => {
    const ctx = logger.startEvent({ event: 'bench.test', kind: 'http' });
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });
});

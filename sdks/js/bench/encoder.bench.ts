import { bench, describe } from 'vitest';
import { createLoza, production, memorySink, string, int, float64, bool } from '../src';

describe('encoder', () => {
  const logger = createLoza(production('bench').withSink(memorySink()));

  bench('emit with 4 enriched attrs', () => {
    const ctx = logger.startEvent({ event: 'bench.encode', kind: 'http' });
    logger.enrich(ctx,
      string('user.id', 'u-abc123'),
      int('status_code', 200),
      float64('duration_ms', 42.5),
      bool('cache_hit', true),
    );
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });

  bench('emit with 12 enriched attrs', () => {
    const ctx = logger.startEvent({ event: 'bench.encode-heavy', kind: 'http' });
    logger.enrich(ctx,
      string('user.id', 'u-abc123'),
      string('tenant.id', 'tenant-acme'),
      string('session.id', 'sess-xyz'),
      string('request.id', 'req-789'),
      string('trace.id', 'trace-000'),
      string('span.id', 'span-111'),
      int('status_code', 200),
      float64('duration_ms', 42.5),
      bool('cache_hit', true),
      string('http.method', 'GET'),
      string('http.path', '/api/users'),
      string('http.user_agent', 'Mozilla/5.0'),
    );
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });
});

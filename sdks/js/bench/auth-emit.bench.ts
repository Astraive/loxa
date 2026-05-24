import { bench, describe } from 'vitest';
import { createLoxa, memorySink } from '../src';

const API_KEY = 'lx_sec_live_kBenchKey_bench_secret_value';

describe('Emit with Auth', () => {
  const sink = memorySink();
  const logger = createLoxa({
    service: 'bench',
    sink,
    apiKey: API_KEY,
  });

  bench('emit_auth', () => {
    const ctx = logger.startEvent({ event: 'bench.auth.emit' });
    ctx.finish('success');
    ctx.emit();
  });

  bench('emit_auth_5_attrs', () => {
    const ctx = logger.startEvent({ event: 'bench.auth.attrs' });
    ctx.set('http.method', 'POST');
    ctx.set('http.path', '/api/payments');
    ctx.set('http.status', 200);
    ctx.set('payment.amount', 99.99);
    ctx.set('payment.success', true);
    ctx.finish('success');
    ctx.emit();
  });
});

describe('Emit Baseline (No Auth)', () => {
  const sink = memorySink();
  const logger = createLoxa({
    service: 'bench',
    sink,
  });

  bench('emit_no_auth', () => {
    const ctx = logger.startEvent({ event: 'bench.baseline' });
    ctx.finish('success');
    ctx.emit();
  });
});

describe('Emit with Sampler + Auth', () => {
  const sink = memorySink();
  const logger = createLoxa({
    service: 'bench',
    sink,
    apiKey: API_KEY,
    sampler: { type: 'random', rate: 0.5 },
  });

  bench('emit_sampler_auth', () => {
    const ctx = logger.startEvent({ event: 'bench.sampler.auth' });
    ctx.finish('success');
    ctx.emit();
  });
});

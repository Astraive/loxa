# Benchmarking

How to run and interpret benchmarks for the LOZA JS SDK (`loza`).

## Running Benchmarks

### Using vitest bench

```bash
cd sdks/js
bunx vitest bench
```

Run a specific benchmark file:

```bash
bunx vitest bench bench/emit.bench.ts
```

### Running individual benchmark files

```bash
cd sdks/js
bunx vitest bench bench/emit.bench.ts
bunx vitest bench bench/encoder.bench.ts
bunx vitest bench bench/sampler.bench.ts
bunx vitest bench bench/middleware.bench.ts
```

## What Is Measured

| Benchmark | File | Measures |
|-----------|------|----------|
| Emit cycle | `emit.bench.ts` | Full emit cycle: startEvent, finish, encode, deliver to MemorySink. |
| Encoder | `encoder.bench.ts` | Emit cycle with enrichment fields. Measures JSON encoding overhead. |
| Sampler | `sampler.bench.ts` | Sampler decisions (SampleErrors, SampleRandom). Measures decision + emit. |
| Middleware | `middleware.bench.ts` | Express/HTTP middleware overhead. Measures request capture + event lifecycle. |

## Expected Results

Typical results on Bun 1.3.14+:

| Benchmark | ops/sec | ns/op | Notes |
|-----------|---------|-------|-------|
| Emit cycle | 50,000-100,000 | 10,000-20,000 | MemorySink, no network. |
| Encoder | 30,000-60,000 | 16,000-33,000 | With 2 enriched attributes. |
| Sampler | 60,000-120,000 | 8,000-16,000 | SampleErrors, error event. |
| Middleware | 10,000-30,000 | 33,000-100,000 | Express middleware overhead. |

Results vary significantly by Bun version and system load.

## Benchmark Configuration

Benchmarks use `vitest` with the configuration in `vitest.config.ts`. The bench mode:

- Runs each benchmark function repeatedly for a fixed duration.
- Reports ops/sec, average time, and margin of error.
- Uses `memorySink()` to avoid network I/O.

## Adding New Benchmarks

Create a new file `sdks/js/bench/<name>.bench.ts`:

```typescript
import { bench, describe } from 'vitest';
import { createLoza, production, memorySink, string } from '../src';

describe('my feature', () => {
  const logger = createLoza({ service: 'bench', sink: memorySink() });

  bench('my benchmark', () => {
    const ctx = logger.startEvent({ event: 'bench.test' });
    logger.enrich(ctx, string('key', 'value'));
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });
});
```

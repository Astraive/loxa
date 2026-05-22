# Benchmarking

How to run and interpret benchmarks for the LOXA JS SDK (loxa-js).

## Running Benchmarks

### Using vitest bench

```bash
cd sdks/js
npx vitest bench
```

Run a specific benchmark file:

```bash
npx vitest bench bench/emit.bench.ts
```

### Using the bench files directly

```bash
cd sdks/js
npx tsx bench/emit.bench.ts
npx tsx bench/encoder.bench.ts
npx tsx bench/sampler.bench.ts
npx tsx bench/middleware.bench.ts
```

## What Is Measured

| Benchmark | File | Measures |
|-----------|------|----------|
| Emit cycle | `emit.bench.ts` | Full emit cycle: startEvent, finish, encode, deliver to MemorySink. |
| Encoder | `encoder.bench.ts` | Emit cycle with enrichment fields. Measures JSON encoding overhead. |
| Sampler | `sampler.bench.ts` | Sampler decisions (SampleErrors, SampleRandom). Measures decision + emit. |
| Middleware | `middleware.bench.ts` | Express/HTTP middleware overhead. Measures request capture + event lifecycle. |

## Expected Results

Typical results on modern hardware (Node.js 22+):

| Benchmark | ops/sec | ns/op | Notes |
|-----------|---------|-------|-------|
| Emit cycle | 50,000-100,000 | 10,000-20,000 | MemorySink, no network. |
| Encoder | 30,000-60,000 | 16,000-33,000 | With 2 enriched attributes. |
| Sampler | 60,000-120,000 | 8,000-16,000 | SampleErrors, error event. |
| Middleware | 10,000-30,000 | 33,000-100,000 | Express middleware overhead. |

Results vary significantly by Node.js version, V8 optimizations, and system load.

## Benchmark Configuration

Benchmarks use `vitest` with the configuration in `vitest.config.ts`. The bench mode:

- Runs each benchmark function repeatedly for a fixed duration.
- Reports ops/sec, average time, and margin of error.
- Uses `memorySink()` to avoid network I/O.

## Adding New Benchmarks

Create a new file `sdks/js/bench/<name>.bench.ts`:

```typescript
import { bench, describe } from 'vitest';
import { createLoxa, production, memorySink, string } from '../src';

describe('my feature', () => {
  const logger = createLoxa({ service: 'bench', sink: memorySink() });

  bench('my benchmark', () => {
    const ctx = logger.startEvent({ event: 'bench.test' });
    logger.enrich(ctx, string('key', 'value'));
    logger.finish(ctx, 'success');
    logger.emit(ctx);
  });
});
```

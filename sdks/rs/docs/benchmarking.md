# Benchmarking

How to run and interpret benchmarks for the LOXA Rust SDK.

## Running Benchmarks

From the repository root:

```bash
cd sdks/rs
cargo bench
```

Run a specific benchmark:

```bash
cargo bench --bench emit_bench
```

## What Is Measured

| Benchmark | File | Measures |
|-----------|------|----------|
| Emit cycle | `emit_bench.rs` | Full emit cycle: start_event, finish, encode, deliver to sink. |
| Encoder | `encoder_bench.rs` | Event encoding with default schema. Measures serialization overhead. |
| Sampler | `sampler_bench.rs` | Sampler decision with `SampleErrors`. Measures decision + emit. |
| Middleware | `middleware_bench.rs` | Tower middleware HTTP capture. Measures request-to-event overhead. |

## Benchmark Files

Located in `sdks/rs/bench/`:

```
bench/
  emit_bench.rs      -- Emit cycle benchmark
  encoder_bench.rs   -- JSON encoding benchmark
  sampler_bench.rs   -- Sampler decision benchmark
  middleware_bench.rs -- Tower middleware benchmark
```

## Expected Results

Typical results on modern hardware (Apple M-series or AMD Zen 4):

| Benchmark | ns/op | Notes |
|-----------|-------|-------|
| Emit cycle | 400-800 | MemorySink, no network. |
| Encoder | 500-1000 | Default schema, minimal attrs. |
| Sampler | 300-700 | SampleErrors, error event. |
| Middleware | 800-1500 | Tower capture_request helper. |

Rust benchmarks are significantly faster than Go/Python/JS due to zero-cost abstractions and no GC overhead.

## Adding New Benchmarks

Create a new file `sdks/rs/bench/<name>_bench.rs`:

```rust
use loxa::{Config, Logger, Params};

pub fn my_benchmark_iteration() -> usize {
    let logger = Logger::new(Config::test("bench"));
    let mut ctx = logger.start_event(Params::new("bench.test"));
    logger.finish(&mut ctx, "success");
    logger.emit(&ctx).unwrap_or_default().len()
}
```

Then add a benchmark harness in `benches/` or use `criterion` for statistical benchmarking.

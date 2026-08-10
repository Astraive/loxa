# Benchmarking

How to run and interpret benchmarks for the LOZA Go SDK.

## Running Benchmarks

```bash
cd sdks/go/bench && go test -bench=. -benchmem
```

Run a specific benchmark:

```bash
go test -bench=BenchmarkEmit -benchmem -count=5
```

Compare with baseline using `benchstat`:

```bash
go test -bench=. -benchmem -count=10 > new.txt
benchstat old.txt new.txt
```

## What Is Measured

| Benchmark | Function | Measures |
|-----------|----------|----------|
| `BenchmarkEmit` | `loza.Emit` | Full emit cycle: startEvent, finish, encode, deliver to MemorySink. |
| `BenchmarkEncoder` | `loza.Emit` with enrichment | Emit cycle with 2 enriched attributes (string + int). Measures JSON encoding overhead. |
| `BenchmarkSampler` | `loza.SampleRandom(0.5)` | Emit cycle with a 50% random sampler. Measures sampler decision + drop overhead. |
| `BenchmarkNetHTTPMiddleware` | `lozahttp.Middleware` | Full HTTP round-trip through net/http middleware, including request capture, event creation, finish, and emit. |

## Expected Results

Typical results on modern hardware (Apple M-series or AMD Zen 4):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| BenchmarkEmit | 1200-1800 | 400-600 | 8-12 |
| BenchmarkEncoder | 1800-2500 | 600-900 | 12-16 |
| BenchmarkSampler | 1000-1600 | 350-550 | 7-10 |
| BenchmarkNetHTTPMiddleware | 8000-15000 | 2000-4000 | 30-50 |

Results vary by hardware, Go version, and system load. Use `benchstat` for statistically meaningful comparisons.

## Interpreting Results

- **ns/op**: Lower is better. Measures wall-clock time per operation.
- **B/op**: Lower is better. Measures bytes allocated per operation.
- **allocs/op**: Lower is better. Measures number of heap allocations per operation.

A spike in `allocs/op` between versions usually indicates a regression in hot-path object creation. A spike in `B/op` may indicate buffer sizing issues.

## Adding New Benchmarks

Create a new file `sdks/go/bench/<name>_bench_test.go`:

```go
package bench

import (
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func BenchmarkMyFeature(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// your benchmark logic here
	}
}
```

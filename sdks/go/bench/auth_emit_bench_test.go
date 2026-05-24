package bench

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

// BenchmarkEmitWithAuth measures emit latency with API key configured.
func BenchmarkEmitWithAuth(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.auth.emit"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

// BenchmarkEmitWithAuthAndAttrs measures emit with auth + enriched attributes.
func BenchmarkEmitWithAuthAndAttrs(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.auth.attrs"})
		loxa.Set(ctx, loxa.String("http.method", "POST"))
		loxa.Set(ctx, loxa.String("http.path", "/api/payments"))
		loxa.Set(ctx, loxa.Int("http.status", 200))
		loxa.Set(ctx, loxa.Float64("payment.amount", 99.99))
		loxa.Set(ctx, loxa.Bool("payment.success", true))
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

// BenchmarkEmitNoAuthBaseline measures emit latency without auth (baseline).
func BenchmarkEmitNoAuthBaseline(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.baseline"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

// BenchmarkEmitWithSamplerAuth measures emit with sampler + auth.
func BenchmarkEmitWithSamplerAuth(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value").
		WithSampler(loxa.SampleRandom(0.5))
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.sampler.auth"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

// BenchmarkEmitBatch10 measures throughput of emitting 10 events in sequence.
func BenchmarkEmitBatch10(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.batch"})
			loxa.Set(ctx, loxa.Int("batch.index", j))
			loxa.Finish(ctx, "success")
			_ = loxa.Emit(ctx)
		}
	}
}

// BenchmarkEmitBatch100 measures throughput of emitting 100 events in sequence.
func BenchmarkEmitBatch100(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.batch100"})
			loxa.Set(ctx, loxa.Int("batch.index", j))
			loxa.Finish(ctx, "success")
			_ = loxa.Emit(ctx)
		}
	}
}

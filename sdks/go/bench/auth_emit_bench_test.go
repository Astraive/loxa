package bench

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

// BenchmarkEmitWithAuth measures emit latency with API key configured.
func BenchmarkEmitWithAuth(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.auth.emit"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

// BenchmarkEmitWithAuthAndAttrs measures emit with auth + enriched attributes.
func BenchmarkEmitWithAuthAndAttrs(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.auth.attrs"})
		loza.Set(ctx, loza.String("http.method", "POST"))
		loza.Set(ctx, loza.String("http.path", "/api/payments"))
		loza.Set(ctx, loza.Int("http.status", 200))
		loza.Set(ctx, loza.Float64("payment.amount", 99.99))
		loza.Set(ctx, loza.Bool("payment.success", true))
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

// BenchmarkEmitNoAuthBaseline measures emit latency without auth (baseline).
func BenchmarkEmitNoAuthBaseline(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.baseline"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

// BenchmarkEmitWithSamplerAuth measures emit with sampler + auth.
func BenchmarkEmitWithSamplerAuth(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value").
		WithSampler(loza.SampleRandom(0.5))
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.sampler.auth"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

// BenchmarkEmitBatch10 measures throughput of emitting 10 events in sequence.
func BenchmarkEmitBatch10(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.batch"})
			loza.Set(ctx, loza.Int("batch.index", j))
			loza.Finish(ctx, "success")
			_ = loza.Emit(ctx)
		}
	}
}

// BenchmarkEmitBatch100 measures throughput of emitting 100 events in sequence.
func BenchmarkEmitBatch100(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().
		WithSink(sink).
		WithAPIKey("lx_sec_live_kBenchKey_bench_secret_value")
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.batch100"})
			loza.Set(ctx, loza.Int("batch.index", j))
			loza.Finish(ctx, "success")
			_ = loza.Emit(ctx)
		}
	}
}

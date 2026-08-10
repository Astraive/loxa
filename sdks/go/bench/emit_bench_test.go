package bench

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
	"github.com/astraive/loza/sdks/go/src/core"
)

// BenchmarkEmit measures basic emit latency (no auth, no sampler).
func BenchmarkEmit(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.emit"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

// --- Trace Context Benchmarks ---

func BenchmarkTraceContextGenerate(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := core.GenerateTraceContext()
		_ = tc.TraceID
		_ = tc.SpanID
	}
}

func BenchmarkTraceContextGenerateTraceID(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = core.GenerateTraceID()
	}
}

func BenchmarkTraceContextGenerateSpanID(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = core.GenerateSpanID()
	}
}

func BenchmarkEmitWithExistingTraceContext(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{
			Event:   "bench.emit",
			TraceID: "0af7651916cd43dd8448eb211c80319c",
			SpanID:  "00f067aa0ba902b7",
		})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

func BenchmarkEmitWithGeneratedTraceContext(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.emit"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

func BenchmarkEmitSampledOutNoTraceGeneration(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink).WithSampler(loza.SampleNone())
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.emit"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

func BenchmarkEmitBatch100WithTraceContext(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.emit"})
			loza.Finish(ctx, "success")
			_ = loza.Emit(ctx)
		}
	}
}

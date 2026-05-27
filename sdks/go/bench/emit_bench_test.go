package bench

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
	"github.com/astraive/loxa/sdks/go/src/core"
)

// BenchmarkEmit measures basic emit latency (no auth, no sampler).
func BenchmarkEmit(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.emit"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
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
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{
			Event:   "bench.emit",
			TraceID: "0af7651916cd43dd8448eb211c80319c",
			SpanID:  "00f067aa0ba902b7",
		})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

func BenchmarkEmitWithGeneratedTraceContext(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.emit"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

func BenchmarkEmitSampledOutNoTraceGeneration(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink).WithSampler(loxa.SampleNone())
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.emit"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

func BenchmarkEmitBatch100WithTraceContext(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.emit"})
			loxa.Finish(ctx, "success")
			_ = loxa.Emit(ctx)
		}
	}
}

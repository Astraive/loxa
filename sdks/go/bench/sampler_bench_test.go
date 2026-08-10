package bench

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func BenchmarkSampler(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	cfg.Sampler = loza.SampleRandom(0.5)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.sample"})
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

package bench

import (
	"context"
	"testing"

	"github.com/astraive/loxa-go"
)

func BenchmarkSampler(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	cfg.Sampler = loxa.SampleRandom(0.5)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.sample"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

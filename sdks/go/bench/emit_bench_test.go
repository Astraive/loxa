package bench

import (
	"context"
	"testing"

	"github.com/astraive/loxa-go"
)

func BenchmarkEmit(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.emit"})
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

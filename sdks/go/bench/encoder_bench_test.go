package bench

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func BenchmarkEncoder(b *testing.B) {
	sink, _ := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	_ = loxa.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "bench.encode"})
		loxa.Enrich(ctx, loxa.String("user.id", "u-1"), loxa.Int("status_code", 200))
		loxa.Finish(ctx, "success")
		_ = loxa.Emit(ctx)
	}
}

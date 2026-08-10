package bench

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func BenchmarkEncoder(b *testing.B) {
	sink, _ := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	_ = loza.Configure(cfg)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := loza.StartEvent(context.Background(), loza.Params{Event: "bench.encode"})
		loza.Enrich(ctx, loza.String("user.id", "u-1"), loza.Int("status_code", 200))
		loza.Finish(ctx, "success")
		_ = loza.Emit(ctx)
	}
}

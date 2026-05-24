package core

import (
	"context"
	"testing"
)

func TestRunEmitsEvent(t *testing.T) {
	sink, store := MemorySink()
	if err := Configure(Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	err := Run(context.Background(), Params{Event: "core.run"}, func(ctx context.Context) error {
		_ = Default().Enrich(ctx, String("module", "core"))
		return nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected one event, got %d", store.Len())
	}
}

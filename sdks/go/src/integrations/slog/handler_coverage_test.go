package slog

import (
	"context"
	stdslog "log/slog"
	"testing"
	"time"

	loza "github.com/astraive/loza/sdks/go"
)

func TestSlogHandlerConvertsAllValueKindsAndGroups(t *testing.T) {
	h := Handler()
	values := []stdslog.Value{
		stdslog.StringValue("text"),
		stdslog.Int64Value(-2),
		stdslog.Uint64Value(3),
		stdslog.Float64Value(1.5),
		stdslog.BoolValue(true),
		stdslog.DurationValue(time.Second),
		stdslog.TimeValue(time.Unix(1, 0)),
		stdslog.GroupValue(stdslog.String("child", "value")),
		stdslog.AnyValue(struct{ Name string }{"custom"}),
	}
	for i, value := range values {
		attr := h.toAttr(stdslog.Any("field", value))
		if attr.Key != "field" || attr.Value == nil {
			t.Fatalf("value %d converted to %#v", i, attr)
		}
	}
	if got := wrapAttrsWithGroups(nil, []string{"outer"}); got != nil {
		t.Fatalf("empty grouped attrs = %#v", got)
	}
}

func TestSlogHandlerHandlesLevelsAndActiveEvents(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	h := Handler().WithAttrs([]stdslog.Attr{stdslog.String("source", "test")}).(*SlogHandler)
	if !h.Enabled(context.Background(), stdslog.LevelDebug) || !h.Enabled(context.Background(), stdslog.LevelError) {
		t.Fatal("handler unexpectedly disabled level")
	}
	for _, level := range []stdslog.Level{stdslog.LevelDebug, stdslog.LevelInfo, stdslog.LevelWarn, stdslog.LevelError} {
		rec := stdslog.NewRecord(time.Now(), level, "message", 0)
		rec.AddAttrs(stdslog.String("key", "value"))
		if err := h.Handle(context.Background(), rec); err != nil {
			t.Fatalf("Handle level %v: %v", level, err)
		}
	}
	ctx := loza.StartEvent(context.Background(), loza.Params{Event: "slog.active"})
	rec := stdslog.NewRecord(time.Now(), stdslog.LevelInfo, "active", 0)
	if err := h.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle active: %v", err)
	}
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("Finish active: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("Emit active: %v", err)
	}
	if store.Len() != 5 {
		t.Fatalf("events = %d, want 5", store.Len())
	}
}

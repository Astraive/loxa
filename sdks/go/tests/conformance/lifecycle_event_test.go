package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestLifecycleEventHelpers(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loza.StartEvent(context.Background(), loza.Params{
		Service: "verification",
		Event:   "checkout.request",
		Kind:    "http",
		Method:  "POST",
		Path:    "/checkout",
		Route:   "/checkout",
	})
	if err := loza.Append(ctx, loza.UserID("u_123"), loza.TenantID("t_123")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := loza.Set(ctx, loza.String("payment.provider", "stripe")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := loza.Merge(ctx, "cart", loza.Int("items", 3)); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got, ok := loza.Get(ctx, "payment.provider"); !ok || got != "stripe" {
		t.Fatalf("expected payment.provider=stripe, got %v ok=%v", got, ok)
	}
	if group, ok := loza.GetGroup(ctx, "cart"); !ok || group["items"] != 3 {
		t.Fatalf("expected merged cart group, got %#v ok=%v", group, ok)
	}
	if err := loza.Delete(ctx, "payment.provider"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := loza.Checkpoint(ctx, "validated", loza.String("stage", "validation")); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	linkedCtx, err := loza.LinkEvent(ctx, "evt_parent", loza.String("link.kind", "parent"))
	if err != nil {
		t.Fatalf("link event: %v", err)
	}
	if _, ok := loza.CurrentEvent(linkedCtx); !ok {
		t.Fatalf("expected current event on linked context")
	}
	cloned, err := loza.CloneEvent(ctx)
	if err != nil {
		t.Fatalf("clone event: %v", err)
	}
	if cloned.EventID != loza.EventID(ctx) {
		t.Fatalf("expected clone to preserve event id")
	}
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	dropCtx := loza.StartEvent(context.Background(), loza.Params{Event: "drop.event"})
	if err := loza.Drop(dropCtx, "capacity"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	cancelCtx := loza.StartEvent(context.Background(), loza.Params{Event: "cancel.event"})
	if err := loza.Cancel(cancelCtx, "user_cancelled"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	abandonCtx := loza.StartEvent(context.Background(), loza.Params{Event: "abandon.event"})
	if err := loza.Abandon(abandonCtx, "orphaned"); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	retryCtx := loza.StartEvent(context.Background(), loza.Params{Event: "retry.event"})
	if err := loza.Retry(retryCtx, loza.Int("attempt", 2)); err != nil {
		t.Fatalf("retry: %v", err)
	}
	partialCtx := loza.StartEvent(context.Background(), loza.Params{Event: "partial.event"})
	if err := loza.Partial(partialCtx, loza.String("reason", "timeout")); err != nil {
		t.Fatalf("partial: %v", err)
	}
	if err := loza.Wrap("wrapped.event", func() error { return nil }); err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected lifecycle helpers to emit events")
	}
}

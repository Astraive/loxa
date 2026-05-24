package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestLifecycleEventHelpers(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Service: "verification",
		Event:   "checkout.request",
		Kind:    "http",
		Method:  "POST",
		Path:    "/checkout",
		Route:   "/checkout",
	})
	if err := loxa.Append(ctx, loxa.UserID("u_123"), loxa.TenantID("t_123")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := loxa.Set(ctx, loxa.String("payment.provider", "stripe")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := loxa.Merge(ctx, "cart", loxa.Int("items", 3)); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got, ok := loxa.Get(ctx, "payment.provider"); !ok || got != "stripe" {
		t.Fatalf("expected payment.provider=stripe, got %v ok=%v", got, ok)
	}
	if group, ok := loxa.GetGroup(ctx, "cart"); !ok || group["items"] != 3 {
		t.Fatalf("expected merged cart group, got %#v ok=%v", group, ok)
	}
	if err := loxa.Delete(ctx, "payment.provider"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := loxa.Checkpoint(ctx, "validated", loxa.String("stage", "validation")); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	linkedCtx, err := loxa.LinkEvent(ctx, "evt_parent", loxa.String("link.kind", "parent"))
	if err != nil {
		t.Fatalf("link event: %v", err)
	}
	if _, ok := loxa.CurrentEvent(linkedCtx); !ok {
		t.Fatalf("expected current event on linked context")
	}
	cloned, err := loxa.CloneEvent(ctx)
	if err != nil {
		t.Fatalf("clone event: %v", err)
	}
	if cloned.EventID != loxa.EventID(ctx) {
		t.Fatalf("expected clone to preserve event id")
	}
	if err := loxa.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := loxa.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	dropCtx := loxa.StartEvent(context.Background(), loxa.Params{Event: "drop.event"})
	if err := loxa.Drop(dropCtx, "capacity"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	cancelCtx := loxa.StartEvent(context.Background(), loxa.Params{Event: "cancel.event"})
	if err := loxa.Cancel(cancelCtx, "user_cancelled"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	abandonCtx := loxa.StartEvent(context.Background(), loxa.Params{Event: "abandon.event"})
	if err := loxa.Abandon(abandonCtx, "orphaned"); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	retryCtx := loxa.StartEvent(context.Background(), loxa.Params{Event: "retry.event"})
	if err := loxa.Retry(retryCtx, loxa.Int("attempt", 2)); err != nil {
		t.Fatalf("retry: %v", err)
	}
	partialCtx := loxa.StartEvent(context.Background(), loxa.Params{Event: "partial.event"})
	if err := loxa.Partial(partialCtx, loxa.String("reason", "timeout")); err != nil {
		t.Fatalf("partial: %v", err)
	}
	if err := loxa.Wrap("wrapped.event", func() error { return nil }); err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected lifecycle helpers to emit events")
	}
}

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestEmitIncludesContractFieldsAndStructuredAttrs(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test()
	cfg.Service = "checkout"
	cfg.Sinks = []Sink{sink}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{
		Event:      "checkout.request",
		Kind:       "http",
		Service:    "checkout",
		Method:     "POST",
		Path:       "/checkout",
		Route:      "/checkout",
		StatusCode: 200,
		UserID:     "user_001",
		TenantID:   "tenant_001",
	})
	_ = l.Enrich(ctx,
		String("cart.id", "cart_001"),
		String("http.client_ip", "127.0.0.1"),
		Group("resource", String("id", "order_123")),
	)
	_ = l.Finish(ctx, "success")
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if store.Len() != 1 {
		t.Fatalf("expected 1 emitted event, got %d", store.Len())
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(store.Raw()[0]), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload["schema_version"] != LOZA_SPEC_VERSION {
		t.Fatalf("expected schema_version %q, got %#v", LOZA_SPEC_VERSION, payload["schema_version"])
	}
	if payload["event_version"] != LOZA_EVENT_VERSION {
		t.Fatalf("expected event_version %q, got %#v", LOZA_EVENT_VERSION, payload["event_version"])
	}
	if payload["kind"] != "http" {
		t.Fatalf("expected kind http, got %#v", payload["kind"])
	}

	httpObj, ok := payload["http"].(map[string]any)
	if !ok {
		t.Fatalf("expected http object")
	}
	if httpObj["client_ip"] != "127.0.0.1" {
		t.Fatalf("expected http.client_ip to be preserved")
	}

	tenantObj, ok := payload["tenant"].(map[string]any)
	if !ok || tenantObj["id"] != "tenant_001" {
		t.Fatalf("expected tenant.id in structured payload")
	}

	resourceObj, ok := payload["resource"].(map[string]any)
	if !ok || resourceObj["id"] != "order_123" {
		t.Fatalf("expected resource.id in structured payload")
	}

	attrsObj, ok := payload["attrs"].(map[string]any)
	if !ok {
		t.Fatalf("expected attrs object")
	}
	cartObj, ok := attrsObj["cart"].(map[string]any)
	if !ok || cartObj["id"] != "cart_001" {
		t.Fatalf("expected attrs.cart.id in structured payload")
	}
}

func TestCheckpointEmitImmediatelyProducesStandaloneEvent(t *testing.T) {
	sink, store := MemorySink()
	cfg := Test()
	cfg.Service = "checkout"
	cfg.Sinks = []Sink{sink}
	cfg.Checkpoints.EmitImmediately = true

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{
		Event:   "checkout.request",
		Kind:    "http",
		Service: "checkout",
	})
	_ = l.Checkpoint(ctx, "db.started", String("phase", "db"))
	_ = l.Finish(ctx, "success")
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if store.Len() != 2 {
		t.Fatalf("expected checkpoint event plus final event, got %d", store.Len())
	}

	var checkpointPayload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(store.Raw()[0]), &checkpointPayload); err != nil {
		t.Fatalf("unmarshal checkpoint payload: %v", err)
	}
	if checkpointPayload["kind"] != "checkpoint" {
		t.Fatalf("expected checkpoint event kind, got %#v", checkpointPayload["kind"])
	}
	if checkpointPayload["event"] != "checkpoint.db.started" {
		t.Fatalf("unexpected checkpoint event name: %#v", checkpointPayload["event"])
	}
}

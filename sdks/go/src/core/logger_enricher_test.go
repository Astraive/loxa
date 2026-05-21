package core

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

type enricherCtxKey string

func TestEmitAppliesContextEnricher(t *testing.T) {
	sink := &captureSink{}
	cfg := Test()
	cfg.Sinks = []Sink{sink}
	cfg.Enricher = func(ctx context.Context) []Attr {
		v, _ := ctx.Value(enricherCtxKey("tenant")).(string)
		return []Attr{String("tenant.id", v)}
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := context.WithValue(context.Background(), enricherCtxKey("tenant"), "tenant-1")
	ctx = l.StartEvent(ctx, Params{Event: "enricher.test"})
	l.Finish(ctx, "success")
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(sink.last), &payload); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	tenantObj, ok := payload["tenant"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant object in payload")
	}
	if tenantObj["id"] != "tenant-1" {
		t.Fatalf("expected tenant.id from enricher")
	}
}


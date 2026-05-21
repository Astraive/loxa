package core

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestEmitEnforcesMaxAttrCountWithTruncatedMarker(t *testing.T) {
	sink := &captureSink{}
	cfg := Test()
	cfg.Sinks = []Sink{sink}
	cfg.Security.MaxAttrCount = 2

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "security.max_attr_count"})
	l.Finish(ctx, "success",
		String("first", "one"),
		String("second", "two"),
		String("third", "three"),
	)
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	payload := decodePayload(t, sink.last)
	attrs, ok := payload["attrs"].(map[string]any)
	if !ok {
		t.Fatalf("expected attrs object, got %#v", payload["attrs"])
	}
	if attrs["first"] != "one" || attrs["second"] != "two" {
		t.Fatalf("expected first two attrs preserved, got %#v", payload)
	}
	if _, ok := attrs["third"]; ok {
		t.Fatalf("expected attr beyond MaxAttrCount to be dropped")
	}
	truncated, ok := attrs["_truncated"].(bool)
	if !ok || !truncated {
		t.Fatalf("expected _truncated=true marker, got %#v", attrs["_truncated"])
	}
}

func TestEmitTruncatesStringFieldsAtMaxFieldBytes(t *testing.T) {
	sink := &captureSink{}
	cfg := Test()
	cfg.Sinks = []Sink{sink}
	cfg.Security.MaxFieldBytes = 4

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "security.max_field_bytes"})
	l.Finish(ctx, "success",
		String("top", "abcdefgh"),
		Group("meta", String("note", "wxyz1234"), Int("count", 7)),
	)
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	payload := decodePayload(t, sink.last)
	attrs, ok := payload["attrs"].(map[string]any)
	if !ok {
		t.Fatalf("expected attrs object, got %#v", payload["attrs"])
	}
	if got := attrs["top"]; got != "abcd" {
		t.Fatalf("expected top field to be truncated to 4 bytes, got %#v", got)
	}

	meta, ok := attrs["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta group object, got %#v", attrs["meta"])
	}
	if got := meta["note"]; got != "wxyz" {
		t.Fatalf("expected nested string to be truncated to 4 bytes, got %#v", got)
	}
	if got := meta["count"]; got != float64(7) {
		t.Fatalf("expected non-string nested field unchanged, got %#v", got)
	}
}

func TestEmitDropsOversizedEventWhenConfigured(t *testing.T) {
	sink := &captureSink{}
	stats := &statsProbe{}
	cfg := Test()
	cfg.Sinks = []Sink{sink}
	cfg.StatsHandler = stats
	cfg.Security.MaxEventBytes = 1
	cfg.Security.DropOversizedEvents = true

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "security.oversized_event"})
	l.Finish(ctx, "success", String("payload", "this guarantees the encoded event is oversized"))
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if len(sink.last) != 0 {
		t.Fatalf("expected oversized event to be dropped and not written to sink")
	}
	if len(stats.drops) != 1 || stats.drops[0] != "oversized_event" {
		t.Fatalf("expected oversized_event drop reason, got %#v", stats.drops)
	}
	if stats.emitCount != 0 {
		t.Fatalf("expected no emit callback for dropped oversized event")
	}
}

func decodePayload(t *testing.T, encoded []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(encoded), &payload); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	return payload
}

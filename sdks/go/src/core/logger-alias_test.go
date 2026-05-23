package core

import (
	"encoding/json"
	"testing"
)

func TestAliasCreatesNewLoggerWithAliasMetadata(t *testing.T) {
	cfg := Dev()
	cfg.Service = "api"
	cfg.Sink = NoopSink()
	logger, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := logger.Alias("audit")
	if err != nil {
		t.Fatal(err)
	}
	if aliased.cfg.Service != "api" {
		t.Errorf("expected service 'api', got %q", aliased.cfg.Service)
	}
	if aliased.cfg.Alias != "audit" {
		t.Errorf("expected alias 'audit', got %q", aliased.cfg.Alias)
	}
	if logger.cfg.Alias == "audit" {
		t.Error("original logger alias should not be mutated")
	}
}

func TestAliasPreservesConfig(t *testing.T) {
	cfg := Dev()
	cfg.Sink = NoopSink()
	cfg.Level = LevelWarn
	logger, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := logger.Alias("audit")
	if err != nil {
		t.Fatal(err)
	}
	if aliased.cfg.Level != LevelWarn {
		t.Errorf("expected level 'warn', got %q", aliased.cfg.Level)
	}
}

func TestAliasEmitsMetadataWithoutChangingService(t *testing.T) {
	sink, store := MemorySink()
	cfg := Dev()
	cfg.Service = "api"
	cfg.Sink = sink
	logger, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := logger.Alias("audit")
	if err != nil {
		t.Fatal(err)
	}
	aliased.Info("permission changed")
	raw := store.Raw()
	if len(raw) != 1 {
		t.Fatalf("expected 1 emitted event, got %d", len(raw))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw[0], &payload); err != nil {
		t.Fatal(err)
	}
	if payload["service"] != "api" {
		t.Fatalf("expected service api, got %#v", payload["service"])
	}
	attrs := payload["attrs"].(map[string]any)
	loxaMeta := attrs["loxa"].(map[string]any)
	if loxaMeta["alias"] != "audit" {
		t.Fatalf("expected loxa.alias audit, got %#v", loxaMeta["alias"])
	}
}

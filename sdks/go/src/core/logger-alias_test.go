package core

import "testing"

func TestAliasCreatesNewLoggerWithDifferentService(t *testing.T) {
	cfg := Dev()
	cfg.Sink = NoopSink()
	logger, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := logger.Alias("audit")
	if err != nil {
		t.Fatal(err)
	}
	if aliased.cfg.Service != "audit" {
		t.Errorf("expected service 'audit', got %q", aliased.cfg.Service)
	}
	if logger.cfg.Service == "audit" {
		t.Error("original logger service should not be mutated")
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

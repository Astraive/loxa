package core

import "testing"

func TestNewRespectsDisableExpandDotKeysOnPresetEncoder(t *testing.T) {
	cfg := Production()
	cfg.FieldNaming.ExpandDotKeys = false

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	enc, ok := l.cfg.Encoder.(*JSONEventEncoder)
	if !ok {
		t.Fatalf("expected JSONEventEncoder, got %T", l.cfg.Encoder)
	}
	if enc.ExpandDotKeys {
		t.Fatalf("expected ExpandDotKeys=false when FieldNaming.ExpandDotKeys=false")
	}
}

func TestNewDoesNotDuplicateSinkWhenSinkAlsoInSinks(t *testing.T) {
	primary, _ := MemorySink()
	cfg := Test()
	cfg.Sink = primary
	cfg.Sinks = []Sink{primary}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	if got := len(l.cfg.Sinks); got != 1 {
		t.Fatalf("expected 1 sink, got %d", got)
	}
}

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

type captureSink struct {
	last []byte
}

func (s *captureSink) Name() string { return "capture" }

func (s *captureSink) WriteEvent(_ context.Context, encoded []byte, _ *Event) error {
	s.last = append(s.last[:0], encoded...)
	return nil
}

func (s *captureSink) Flush(_ context.Context) error { return nil }

func (s *captureSink) Close(_ context.Context) error { return nil }

func TestRunEventPanicStackRespectsIncludeSource(t *testing.T) {
	cases := []struct {
		name          string
		includeSource bool
		wantStack     bool
	}{
		{name: "exclude by default", includeSource: false, wantStack: false},
		{name: "include when enabled", includeSource: true, wantStack: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &captureSink{}
			cfg := Test()
			cfg.Sinks = []Sink{sink}
			cfg.IncludeSource = tc.includeSource
			cfg.PanicRecovery = true

			l, err := New(cfg)
			if err != nil {
				t.Fatalf("new logger: %v", err)
			}

			prev := Default()
			SetDefault(l)
			defer SetDefault(prev)

			err = RunEvent(context.Background(), Params{Event: "panic.test"}, func(context.Context) error {
				panic("boom")
			})
			if err == nil {
				t.Fatalf("expected panic recovery error")
			}

			if len(sink.last) == 0 {
				t.Fatalf("expected emitted event")
			}

			var payload map[string]any
			if uerr := json.Unmarshal(bytes.TrimSpace(sink.last), &payload); uerr != nil {
				t.Fatalf("unmarshal event: %v", uerr)
			}

			errObj, ok := payload["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected error object in payload")
			}
			_, hasStack := errObj["stack"]
			if hasStack != tc.wantStack {
				t.Fatalf("stack presence mismatch: got=%v want=%v", hasStack, tc.wantStack)
			}
		})
	}
}


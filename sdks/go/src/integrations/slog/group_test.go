package slog

import (
	"context"
	stdslog "log/slog"
	"testing"

	"github.com/astraive/loxa-go"
)

func TestWithGroupBuildsNestedAttrs(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().WithSink(sink)
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	logger := stdslog.New(Handler().WithGroup("http"))
	logger.InfoContext(context.Background(), "request", stdslog.String("method", "GET"))

	if store.Len() != 1 {
		t.Fatalf("expected one emitted event, got %d", store.Len())
	}
	ev := store.Events()[0]
	group, ok := findGroup(ev.Attrs, "http")
	if !ok {
		t.Fatalf("expected grouped attrs under http")
	}
	if got, ok := findString(group, "method"); !ok || got != "GET" {
		t.Fatalf("expected grouped http.method attr")
	}
}

func findGroup(attrs []loxa.Attr, key string) ([]loxa.Attr, bool) {
	for _, a := range attrs {
		if a.Key != key {
			continue
		}
		children, ok := a.Value.([]loxa.Attr)
		if ok {
			return children, true
		}
	}
	return nil, false
}

func findString(attrs []loxa.Attr, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key != key {
			continue
		}
		v, ok := a.Value.(string)
		return v, ok
	}
	return "", false
}


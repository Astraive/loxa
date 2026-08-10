package slog

import (
	"context"
	stdslog "log/slog"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestWithGroupBuildsNestedAttrs(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithSink(sink)
	if err := loza.Configure(cfg); err != nil {
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

func findGroup(attrs []loza.Attr, key string) ([]loza.Attr, bool) {
	for _, a := range attrs {
		if a.Key != key {
			continue
		}
		children, ok := a.Value.([]loza.Attr)
		if ok {
			return children, true
		}
	}
	return nil, false
}

func findString(attrs []loza.Attr, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key != key {
			continue
		}
		v, ok := a.Value.(string)
		return v, ok
	}
	return "", false
}


package core

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type emittedShapeFixture struct {
	Params struct {
		Service    string `json:"service"`
		Event      string `json:"event"`
		Kind       string `json:"kind"`
		Method     string `json:"method"`
		Path       string `json:"path"`
		Route      string `json:"route"`
		StatusCode int    `json:"status_code"`
	} `json:"params"`
	Attrs  map[string]any `json:"attrs"`
	Finish struct {
		Outcome string `json:"outcome"`
	} `json:"finish"`
	Expected struct {
		Present []string       `json:"present"`
		Equals  map[string]any `json:"equals"`
	} `json:"expected"`
}

func firstFixturePath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return paths[0]
}

func TestSharedEmittedShapeFixture(t *testing.T) {
	fixture := loadEmittedShapeFixture(t)

	sink, store := MemorySink()
	cfg := Test()
	cfg.Service = fixture.Params.Service
	cfg.Sinks = []Sink{sink}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{
		Event:      fixture.Params.Event,
		Kind:       fixture.Params.Kind,
		Service:    fixture.Params.Service,
		Method:     fixture.Params.Method,
		Path:       fixture.Params.Path,
		Route:      fixture.Params.Route,
		StatusCode: fixture.Params.StatusCode,
	})
	for key, value := range fixture.Attrs {
		l.Enrich(ctx, Any(key, value))
	}
	l.Finish(ctx, fixture.Finish.Outcome)
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	raw := bytes.TrimSpace(store.Raw()[0])
	t.Logf("RAW PAYLOAD: %s", string(raw))
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	t.Logf("event_state = %v", payload["event_state"])
	assertFixtureShape(t, payload, fixture)
}

func loadEmittedShapeFixture(t *testing.T) emittedShapeFixture {
	t.Helper()
	path := firstFixturePath(
		filepath.Join("..", "..", "..", "..", "spec", "fixtures", "emitted-shape", "structured_http_success.json"),
		filepath.Join("..", "..", "..", "..", "spec", "examples", "golden", "emitted-shape", "structured_http_success.json"),
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture emittedShapeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return fixture
}

func assertFixtureShape(t *testing.T, payload map[string]any, fixture emittedShapeFixture) {
	t.Helper()
	for _, path := range fixture.Expected.Present {
		if _, ok := lookupFixturePath(payload, path); !ok {
			t.Fatalf("expected %s to be present in payload", path)
		}
	}
	for path, want := range fixture.Expected.Equals {
		got, ok := lookupFixturePath(payload, path)
		if !ok {
			t.Fatalf("expected %s to be present", path)
		}
		if !fixtureValueEqual(got, want) {
			t.Fatalf("expected %s=%#v, got %#v", path, want, got)
		}
	}
}

func lookupFixturePath(value any, path string) (any, bool) {
	current := value
	for _, segment := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := obj[segment]
		if !exists {
			return nil, false
		}
		current = next
	}
	return current, true
}

func fixtureValueEqual(got, want any) bool {
	switch wantValue := want.(type) {
	case float64:
		switch gotValue := got.(type) {
		case float64:
			return gotValue == wantValue
		case int:
			return float64(gotValue) == wantValue
		case int64:
			return float64(gotValue) == wantValue
		case json.Number:
			parsed, err := gotValue.Float64()
			return err == nil && parsed == wantValue
		case string:
			parsed, err := strconv.ParseFloat(gotValue, 64)
			return err == nil && parsed == wantValue
		default:
			return false
		}
	default:
		return got == want
	}
}

package loki

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
)

func TestNewRejectsInvalidURL(t *testing.T) {
	_, err := New(Config{URL: "://bad"})
	if err == nil {
		t.Fatalf("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlushPushesLokiPayload(t *testing.T) {
	var receivedBody []byte
	var receivedTenant string
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		receivedTenant = r.Header.Get("X-Scope-OrgID")
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		receivedBody = append([]byte(nil), body...)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	rawSink, err := New(Config{
		URL:           srv.URL + "/loki/api/v1/push",
		TenantID:      "tenant-a",
		Labels:        map[string]string{"service": "checkout"},
		BatchSize:     10,
		FlushInterval: time.Hour,
		Client:        srv.Client(),
	})
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	s := rawSink.(*sink)
	defer func() { _ = s.Close(context.Background()) }()

	ev := &collectorevent.Event{Timestamp: time.Unix(1, 42)}
	if err := s.WriteEvent(context.Background(), []byte(`{"event":"checkout.request"}`), ev); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if reqCount.Load() != 0 {
		t.Fatalf("expected no push before flush, got %d", reqCount.Load())
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if reqCount.Load() != 1 {
		t.Fatalf("expected 1 push after flush, got %d", reqCount.Load())
	}
	if receivedTenant != "tenant-a" {
		t.Fatalf("unexpected tenant header: %q", receivedTenant)
	}

	var payload lokiPushPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Streams) != 1 {
		t.Fatalf("expected one stream, got %d", len(payload.Streams))
	}
	if payload.Streams[0].Stream["service"] != "checkout" {
		t.Fatalf("unexpected stream labels: %+v", payload.Streams[0].Stream)
	}
	if len(payload.Streams[0].Values) != 1 {
		t.Fatalf("expected one value, got %d", len(payload.Streams[0].Values))
	}
	if got := payload.Streams[0].Values[0][1]; got != `{"event":"checkout.request"}` {
		t.Fatalf("unexpected pushed line: %q", got)
	}
	if got := payload.Streams[0].Values[0][0]; got != "1000000042" {
		t.Fatalf("unexpected timestamp value: %q", got)
	}
}

func TestFlushRestoreOnFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream down"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	rawSink, err := New(Config{
		URL:           srv.URL,
		BatchSize:     10,
		FlushInterval: time.Hour,
		Client:        srv.Client(),
	})
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	s := rawSink.(*sink)
	defer func() { _ = s.Close(context.Background()) }()

	if err := s.WriteEvent(context.Background(), []byte(`{"event":"retry-me"}`), nil); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("expected flush to retry and succeed, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 push attempts, got %d", calls.Load())
	}
}

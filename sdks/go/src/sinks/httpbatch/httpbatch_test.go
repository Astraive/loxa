package httpbatch

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNewRejectsMissingURL(t *testing.T) {
	if _, err := New(Config{}); err == nil || !strings.Contains(err.Error(), "URL is required") {
		t.Fatalf("New empty URL error = %v", err)
	}
}

func TestBatchSinkAppliesDefaultsAndFlushesOnTicker(t *testing.T) {
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		requests <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	defaultSink, err := New(Config{URL: server.URL})
	if err != nil {
		t.Fatalf("New defaults: %v", err)
	}
	if err := defaultSink.Close(context.Background()); err != nil {
		t.Fatalf("Close defaults: %v", err)
	}

	sink, err := New(Config{URL: server.URL, FlushInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("New ticker: %v", err)
	}
	if err := sink.WriteEvent(context.Background(), []byte("tick\n"), nil); err != nil {
		t.Fatalf("WriteEvent: %v", err)
	}
	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal("ticker did not flush buffered event")
	}
	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBatchSinkReturnsRequestErrors(t *testing.T) {
	malformed, err := New(Config{URL: "://invalid", BatchSize: 1, FlushInterval: time.Hour})
	if err != nil {
		t.Fatalf("New malformed URL: %v", err)
	}
	if err := malformed.WriteEvent(context.Background(), []byte("invalid\n"), nil); err == nil {
		t.Fatal("WriteEvent with malformed URL returned nil")
	}
	_ = malformed.Close(context.Background())

	transportError := errors.New("transport unavailable")
	failed, err := New(Config{
		URL:           "http://collector.invalid",
		BatchSize:     1,
		FlushInterval: time.Hour,
		Client:        &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, transportError })},
	})
	if err != nil {
		t.Fatalf("New transport error sink: %v", err)
	}
	if err := failed.WriteEvent(context.Background(), []byte("fails\n"), nil); !errors.Is(err, transportError) {
		t.Fatalf("transport error = %v, want %v", err, transportError)
	}
	_ = failed.Close(context.Background())
}

func TestBatchSinkFlushesBatchesAndHonorsHeaders(t *testing.T) {
	var mu sync.Mutex
	var bodies [][]byte
	var encodings []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if r.Header.Get("Content-Type") != "application/x-ndjson" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Test") != "yes" {
			t.Errorf("X-Test = %q", r.Header.Get("X-Test"))
		}
		mu.Lock()
		bodies = append(bodies, body)
		encodings = append(encodings, r.Header.Get("Content-Encoding"))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	got, err := New(Config{
		URL:           server.URL,
		Method:        http.MethodPut,
		Headers:       map[string]string{"X-Test": "yes"},
		BatchSize:     2,
		FlushInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if got.Name() != "httpbatch" {
		t.Fatalf("Name = %q", got.Name())
	}
	if err := got.WriteEvent(ctx, []byte("one\n"), nil); err != nil {
		t.Fatalf("first WriteEvent: %v", err)
	}
	if err := got.WriteEvent(ctx, []byte("two\n"), nil); err != nil {
		t.Fatalf("second WriteEvent: %v", err)
	}
	impl := got.(*sink)
	if err := impl.WriteBatch(ctx, nil, nil); err != nil {
		t.Fatalf("empty WriteBatch: %v", err)
	}
	if err := impl.WriteBatch(ctx, [][]byte{[]byte("three\n")}, nil); err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}
	if err := got.Flush(ctx); err != nil {
		t.Fatalf("empty Flush: %v", err)
	}
	if err := got.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(bodies))
	}
	if string(bodies[0]) != "one\ntwo\n" || string(bodies[1]) != "three\n" {
		t.Fatalf("request bodies = %q and %q", bodies[0], bodies[1])
	}
	if encodings[0] != "" || encodings[1] != "" {
		t.Fatalf("unexpected content encodings: %v", encodings)
	}
}

func TestBatchSinkGzipAndStatusErrors(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "bad gzip", http.StatusBadRequest)
			return
		}
		received, _ = io.ReadAll(reader)
		_ = reader.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink, err := New(Config{URL: server.URL, Gzip: true, BatchSize: 1, FlushInterval: time.Hour})
	if err != nil {
		t.Fatalf("New gzip: %v", err)
	}
	if err := sink.WriteEvent(context.Background(), []byte("compressed\n"), nil); err != nil {
		t.Fatalf("gzip WriteEvent: %v", err)
	}
	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	if string(received) != "compressed\n" {
		t.Fatalf("decompressed payload = %q", received)
	}

	failure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusBadGateway)
	}))
	defer failure.Close()
	bad, err := New(Config{URL: failure.URL, BatchSize: 1, FlushInterval: time.Hour})
	if err != nil {
		t.Fatalf("New failure sink: %v", err)
	}
	if err := bad.WriteEvent(context.Background(), []byte("fails\n"), nil); err == nil || !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("status error = %v", err)
	}
	if err := bad.Close(context.Background()); err != nil {
		t.Fatalf("Close after failed WriteEvent: %v", err)
	}
}

package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRotatingFileSinkRotateIncludesReopenError(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.log")
	f, err := os.OpenFile(existingPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open test file: %v", err)
	}

	s := &rotatingFileSink{
		cfg: RotatingFileConfig{
			Path: filepath.Join(dir, "missing", "current.log"),
		},
		f: f,
	}

	err = s.rotate()
	if err == nil {
		t.Fatalf("expected rotate error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "rotate") {
		t.Fatalf("expected rotate error context, got %q", msg)
	}
	if !strings.Contains(msg, "reopen") {
		t.Fatalf("expected reopen error context, got %q", msg)
	}
}

func TestCollectorSink_RetriesRetryableResponse(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errors":[{"code":"rate_limited","message":"retry","retryable":true}]}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer srv.Close()

	sink, err := CollectorSink(CollectorSinkConfig{
		Endpoint:          srv.URL,
		MaxRetries:        2,
		MaxBackoff:        100 * time.Millisecond,
		Timeout:           2 * time.Second,
		ConnectionTimeout: 2 * time.Second,
		Service:           "svc",
	})
	if err != nil {
		t.Fatalf("CollectorSink() error = %v", err)
	}

	ev := &Event{Service: "svc", Event: "test.event"}
	encoded := []byte(`{"event_id":"evt_1","event":"test.event","service":"svc"}`)
	if err := sink.WriteEvent(context.Background(), encoded, ev); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}
	if attempts.Load() < 2 {
		t.Fatalf("expected retry to happen, attempts=%d", attempts.Load())
	}
}

func TestCollectorSink_UsesCustomClientWithRetries(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			n := attempts.Add(1)
			if n == 1 {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Body:       io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
					Header:     http.Header{"Retry-After": []string{"0"}},
					Request:    req,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(strings.NewReader(`{"status":"accepted"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	sink, err := CollectorSink(CollectorSinkConfig{
		Endpoint:          "https://collector.example",
		Client:            client,
		MaxRetries:        1,
		MaxBackoff:        10 * time.Millisecond,
		Timeout:           2 * time.Second,
		ConnectionTimeout: 2 * time.Second,
		Service:           "svc",
	})
	if err != nil {
		t.Fatalf("CollectorSink() error = %v", err)
	}

	ev := &Event{Service: "svc", Event: "test.event"}
	encoded := []byte(`{"event_id":"evt_1","event":"test.event","service":"svc"}`)
	if err := sink.WriteEvent(context.Background(), encoded, ev); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected custom client retries, attempts=%d", attempts.Load())
	}
}

func TestCollectorSink_FailsOnPartialInvalidBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"status":"partial","accepted":1,"invalid":1,"acks":[{"status":"accepted","reason":"accepted"},{"status":"invalid","reason":"schema_invalid","message":"schema validation failed"}]}`))
	}))
	defer srv.Close()

	sink, err := CollectorSink(CollectorSinkConfig{
		Endpoint:          srv.URL,
		MaxRetries:        0,
		MaxBackoff:        10 * time.Millisecond,
		Timeout:           2 * time.Second,
		ConnectionTimeout: 2 * time.Second,
		Service:           "svc",
	})
	if err != nil {
		t.Fatalf("CollectorSink() error = %v", err)
	}

	ev := &Event{Service: "svc", Event: "test.event"}
	encoded := []byte(`{"event_id":"evt_1","event":"test.event","service":"svc"}`)
	err = sink.WriteEvent(context.Background(), encoded, ev)
	if err == nil {
		t.Fatalf("expected partial invalid batch to fail")
	}
	if !strings.Contains(err.Error(), "schema validation failed") {
		t.Fatalf("expected schema failure in error, got %v", err)
	}
}

func TestCollectorSink_CompressesRequestBodyWhenEnabled(t *testing.T) {
	var contentEncoding string
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding = r.Header.Get("Content-Encoding")
		var err error
		rawBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer srv.Close()

	sink, err := CollectorSink(CollectorSinkConfig{
		Endpoint:          srv.URL,
		MaxRetries:        0,
		MaxBackoff:        10 * time.Millisecond,
		Timeout:           2 * time.Second,
		ConnectionTimeout: 2 * time.Second,
		Service:           "svc",
		EnableCompression: true,
	})
	if err != nil {
		t.Fatalf("CollectorSink() error = %v", err)
	}

	ev := &Event{Service: "svc", Event: "test.event"}
	encoded := []byte(`{"event_id":"evt_1","event":"test.event","service":"svc"}`)
	if err := sink.WriteEvent(context.Background(), encoded, ev); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	if contentEncoding != "gzip" {
		t.Fatalf("expected gzip content-encoding, got %q", contentEncoding)
	}
	reader, err := gzip.NewReader(bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Contains(decompressed, []byte(`"events"`)) {
		t.Fatalf("expected compressed collector envelope, got %s", string(decompressed))
	}
}

package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPostIngestValidatesEnvelopeBeforeSending(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	payload := []byte(`{"api_version":"v1","source":{"sdk":"loxa-cli","version":"1.0.0","service":"checkout"}}`)
	if err := PostIngest(srv.URL, "application/json", payload); err == nil {
		t.Fatal("expected invalid envelope to fail validation")
	}
	if hits.Load() != 0 {
		t.Fatalf("expected no request to be sent, got %d hits", hits.Load())
	}
}

func TestPostIngestRetriesRetryableCollectorResponse(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Method, http.MethodPost) {
			t.Fatalf("unexpected method %s", r.Method)
		}
		n := hits.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status":"partial","accepted":0,"rejected":0,"invalid":0,"errors":[{"code":"rate_limited","message":"retry","retryable":true}]}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted","accepted":1,"rejected":0,"invalid":0}`))
	}))
	defer srv.Close()

	payload := []byte(`{"api_version":"v1","source":{"sdk":"loxa-cli","version":"1.0.0","service":"checkout"},"events":[{"schema_version":"v1","event_version":"v1","timestamp":"2026-05-12T00:00:00Z","event_id":"evt_1","service":"checkout","event":"checkout.request","kind":"http"}]}`)
	if err := PostIngest(srv.URL, "application/json", payload); err != nil {
		t.Fatalf("expected retryable ingest to succeed, got %v", err)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected retry to happen, hits=%d", hits.Load())
	}
}

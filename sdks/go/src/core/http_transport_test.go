package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPTransport_DefaultConfig(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{})

	if transport.maxRetries != 3 {
		t.Errorf("expected default maxRetries=3, got %d", transport.maxRetries)
	}
	if transport.maxBackoff != 30*time.Second {
		t.Errorf("expected default maxBackoff=30s, got %s", transport.maxBackoff)
	}
	if transport.timeout != 10*time.Second {
		t.Errorf("expected default timeout=10s, got %s", transport.timeout)
	}
	if transport.connectionTimeout != 5*time.Second {
		t.Errorf("expected default connectionTimeout=5s, got %s", transport.connectionTimeout)
	}
}

func TestNewHTTPTransport_CustomConfig(t *testing.T) {
	cfg := HTTPTransportConfig{
		MaxRetries:        5,
		MaxBackoff:        60 * time.Second,
		Timeout:           20 * time.Second,
		ConnectionTimeout: 10 * time.Second,
	}
	transport := NewHTTPTransport(cfg)

	if transport.maxRetries != 5 {
		t.Errorf("expected maxRetries=5, got %d", transport.maxRetries)
	}
	if transport.maxBackoff != 60*time.Second {
		t.Errorf("expected maxBackoff=60s, got %s", transport.maxBackoff)
	}
	if transport.timeout != 20*time.Second {
		t.Errorf("expected timeout=20s, got %s", transport.timeout)
	}
	if transport.connectionTimeout != 10*time.Second {
		t.Errorf("expected connectionTimeout=10s, got %s", transport.connectionTimeout)
	}
}

func TestHTTPTransport_SuccessfulRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{})
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"status":"success"}` {
		t.Errorf("unexpected response body: %s", string(resp.Body))
	}
}

func TestHTTPTransport_RetryOn429(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
		MaxBackoff: 5 * time.Second,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := transport.Do(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
	// Should have waited at least 2 seconds (2 retries with 1s Retry-After each)
	if elapsed < 2*time.Second {
		t.Errorf("expected at least 2s elapsed for retries, got %s", elapsed)
	}
}

func TestHTTPTransport_RetryOn503(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
		MaxBackoff: 5 * time.Second,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if attempts.Load() != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestHTTPTransport_ExhaustedRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 2,
		MaxBackoff: 5 * time.Second,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "HTTP 429") {
		t.Errorf("expected HTTP 429 error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response even on error")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", resp.StatusCode)
	}
	// Should attempt: initial + 2 retries = 3 total
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestHTTPTransport_NoRetryOn400(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 400 status")
	}
	if resp == nil {
		t.Fatal("expected response even on error")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
	// Should not retry on 400
	if attempts.Load() != 1 {
		t.Errorf("expected 1 attempt (no retries), got %d", attempts.Load())
	}
}

func TestHTTPTransport_RetryAfterHeader_Seconds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := transport.Do(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	// Should wait at least 2 seconds as specified in Retry-After
	if elapsed < 2*time.Second {
		t.Errorf("expected at least 2s wait, got %s", elapsed)
	}
}

func TestHTTPTransport_RetryAfterHeader_HTTPDate(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			// Set Retry-After to 2 seconds in the future
			retryTime := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", retryTime)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := transport.Do(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	// Should wait approximately 2 seconds
	if elapsed < 1900*time.Millisecond || elapsed > 2500*time.Millisecond {
		t.Errorf("expected ~2s wait, got %s", elapsed)
	}
}

func TestHTTPTransport_ExponentialBackoff(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 5,
		MaxBackoff: 30 * time.Second,
	})

	// Test backoff calculation for different attempts
	testCases := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 750 * time.Millisecond, 1250 * time.Millisecond},  // ~1s ± 25%
		{1, 1500 * time.Millisecond, 2500 * time.Millisecond}, // ~2s ± 25%
		{2, 3000 * time.Millisecond, 5000 * time.Millisecond}, // ~4s ± 25%
		{3, 6000 * time.Millisecond, 10 * time.Second},        // ~8s ± 25%
	}

	for _, tc := range testCases {
		backoff := transport.calculateBackoff(tc.attempt, 0)
		if backoff < tc.minExpected || backoff > tc.maxExpected {
			t.Errorf("attempt %d: expected backoff between %s and %s, got %s",
				tc.attempt, tc.minExpected, tc.maxExpected, backoff)
		}
	}
}

func TestHTTPTransport_MaxBackoffCap(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 10,
		MaxBackoff: 5 * time.Second,
	})

	// High attempt number should be capped at maxBackoff
	backoff := transport.calculateBackoff(10, 0)
	if backoff > 5*time.Second {
		t.Errorf("expected backoff capped at 5s, got %s", backoff)
	}
}

func TestHTTPTransport_RetryAfterOverridesExponentialBackoff(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
		MaxBackoff: 30 * time.Second,
	})

	// Retry-After should override exponential backoff
	retryAfter := 3 * time.Second
	backoff := transport.calculateBackoff(5, retryAfter)
	if backoff != retryAfter {
		t.Errorf("expected backoff=%s (from Retry-After), got %s", retryAfter, backoff)
	}
}

func TestHTTPTransport_RetryAfterCappedAtMaxBackoff(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
		MaxBackoff: 10 * time.Second,
	})

	// Retry-After exceeding maxBackoff should be capped
	retryAfter := 60 * time.Second
	backoff := transport.calculateBackoff(0, retryAfter)
	if backoff != 10*time.Second {
		t.Errorf("expected backoff capped at 10s, got %s", backoff)
	}
}

func TestHTTPTransport_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 5,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = transport.Do(ctx, req)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context cancellation error, got %v", err)
	}
}

func TestHTTPTransport_NetworkError_Retryable(t *testing.T) {
	// Create a server that closes immediately to simulate network error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries:        2,
		ConnectionTimeout: 100 * time.Millisecond,
		Timeout:           200 * time.Millisecond,
	})
	req, err := http.NewRequest(http.MethodPost, "http://"+addr, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = transport.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "all retries exhausted") {
		t.Errorf("expected retries exhausted error, got %v", err)
	}
}

func TestHTTPTransport_RequestBodyPreservedOnRetry(t *testing.T) {
	var attempts atomic.Int32
	var receivedBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 3,
	})
	
	expectedBody := "test request body"
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(expectedBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = transport.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(receivedBodies) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(receivedBodies))
	}
	for i, body := range receivedBodies {
		if body != expectedBody {
			t.Errorf("request %d: expected body %q, got %q", i+1, expectedBody, body)
		}
	}
}

func TestHTTPTransport_CustomClientPreservesRetriesAndMetrics(t *testing.T) {
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
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	metrics := NewMetricsCollector("loxa_http", 4)
	transport := NewHTTPTransport(HTTPTransportConfig{
		MaxRetries: 1,
		MaxBackoff: 10 * time.Millisecond,
		Client:     client,
		Metrics:    metrics,
	})

	req, err := http.NewRequest(http.MethodPost, "https://example.com", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retry, got %d", resp.StatusCode)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `loxa_http_retry_total{attempt="1"} 1`) {
		t.Fatalf("expected retry metric in output, got %s", body)
	}
	if !strings.Contains(body, `loxa_http_backpressure_total 1`) {
		t.Fatalf("expected backpressure metric in output, got %s", body)
	}
}

func TestHTTPTransport_IsRetryableError(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{})

	testCases := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "timeout error",
			err:       &timeoutError{},
			retryable: true,
		},
		{
			name:      "temporary error",
			err:       &temporaryError{},
			retryable: true,
		},
		{
			name:      "generic error",
			err:       errors.New("generic error"),
			retryable: false,
		},
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := transport.isRetryableError(tc.err)
			if result != tc.retryable {
				t.Errorf("expected retryable=%v, got %v", tc.retryable, result)
			}
		})
	}
}

func TestHTTPTransport_IsRetryableStatus(t *testing.T) {
	transport := NewHTTPTransport(HTTPTransportConfig{})

	testCases := []struct {
		status    int
		retryable bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			result := transport.isRetryableStatus(tc.status)
			if result != tc.retryable {
				t.Errorf("status %d: expected retryable=%v, got %v", tc.status, tc.retryable, result)
			}
		})
	}
}

// Mock error types for testing
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

type temporaryError struct{}

func (e *temporaryError) Error() string   { return "temporary" }
func (e *temporaryError) Timeout() bool   { return false }
func (e *temporaryError) Temporary() bool { return true }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

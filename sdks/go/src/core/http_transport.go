package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

// HTTPTransport provides HTTP client with retry logic for event delivery.
type HTTPTransport struct {
	client            *http.Client
	maxRetries        int
	maxBackoff        time.Duration
	timeout           time.Duration
	connectionTimeout time.Duration
	metrics           *MetricsCollector
}

// HTTPTransportConfig configures the HTTP transport.
type HTTPTransportConfig struct {
	MaxRetries        int
	MaxBackoff        time.Duration
	Timeout           time.Duration
	ConnectionTimeout time.Duration
	Client            *http.Client
	Metrics           *MetricsCollector
}

// NewHTTPTransport creates a new HTTP transport with retry logic.
func NewHTTPTransport(cfg HTTPTransportConfig) *HTTPTransport {
	// Apply defaults
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.ConnectionTimeout == 0 {
		cfg.ConnectionTimeout = 5 * time.Second
	}

	client := cfg.Client
	if client == nil {
		// Create HTTP client with custom transport for connection timeout.
		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: cfg.ConnectionTimeout,
			}).DialContext,
			TLSHandshakeTimeout:   cfg.ConnectionTimeout,
			ResponseHeaderTimeout: cfg.Timeout,
			ExpectContinueTimeout: 1 * time.Second,
		}

		client = &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		}
	}

	return &HTTPTransport{
		client:            client,
		maxRetries:        cfg.MaxRetries,
		maxBackoff:        cfg.MaxBackoff,
		timeout:           cfg.Timeout,
		connectionTimeout: cfg.ConnectionTimeout,
		metrics:           cfg.Metrics,
	}
}

// HTTPResponse represents the response from an HTTP request.
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Do executes an HTTP request with retry logic.
// It implements exponential backoff with jitter and honors Retry-After headers.
func (t *HTTPTransport) Do(ctx context.Context, req *http.Request) (*HTTPResponse, error) {
	var lastErr error
	var bodyBytes []byte

	// Read and store the request body if present (for retries)
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
	}

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		// Restore request body for retry
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Execute request
		resp, err := t.client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, t.maxRetries+1, err)

			// Check if we should retry
			if attempt < t.maxRetries && t.isRetryableError(err) {
				if t.metrics != nil {
					t.metrics.OnRetry(attempt + 1)
				}
				backoff := t.calculateBackoff(attempt, 0)
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return nil, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
				}
			}
			continue
		}

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Check if response indicates success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &HTTPResponse{
				StatusCode: resp.StatusCode,
				Body:       respBody,
				Headers:    resp.Header,
			}, nil
		}

		// Handle retryable status codes (429, 503)
		if t.isRetryableStatus(resp.StatusCode) && attempt < t.maxRetries {
			retryAfter := t.parseRetryAfter(resp.Header)
			backoff := t.calculateBackoff(attempt, retryAfter)
			if t.metrics != nil {
				t.metrics.OnBackpressure()
				t.metrics.OnRetry(attempt + 1)
			}

			lastErr = fmt.Errorf("retryable status %d (attempt %d/%d)", resp.StatusCode, attempt+1, t.maxRetries+1)

			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			}
		}

		// Non-retryable error or last attempt
		return &HTTPResponse{
			StatusCode: resp.StatusCode,
			Body:       respBody,
			Headers:    resp.Header,
		}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
	}
	return nil, fmt.Errorf("all retries exhausted")
}

// isRetryableError determines if an error is retryable.
func (t *HTTPTransport) isRetryableError(err error) bool {
	// Network errors are generally retryable
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

// isRetryableStatus determines if an HTTP status code is retryable.
func (t *HTTPTransport) isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || // 429
		statusCode == http.StatusServiceUnavailable // 503
}

// parseRetryAfter extracts the retry delay from Retry-After header.
// Returns the delay in milliseconds, or 0 if not present or invalid.
func (t *HTTPTransport) parseRetryAfter(headers http.Header) time.Duration {
	retryAfter := headers.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if t, err := http.ParseTime(retryAfter); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			// HTTP-date Retry-After only has second precision. Round up so we do
			// not retry earlier than the server intended due to sub-second loss.
			return ((delay + time.Second - 1) / time.Second) * time.Second
		}
	}

	return 0
}

// calculateBackoff calculates the backoff duration with exponential backoff and jitter.
// If retryAfter is provided (from Retry-After header), it takes precedence.
func (t *HTTPTransport) calculateBackoff(attempt int, retryAfter time.Duration) time.Duration {
	// Honor Retry-After header if present
	if retryAfter > 0 {
		if retryAfter > t.maxBackoff {
			return t.maxBackoff
		}
		return retryAfter
	}

	// Exponential backoff: base_delay * 2^attempt
	baseDelay := 1 * time.Second
	exponentialDelay := float64(baseDelay) * math.Pow(2, float64(attempt))

	// Add jitter (±25% of the delay)
	jitter := exponentialDelay * 0.25 * (rand.Float64()*2 - 1)
	delay := time.Duration(exponentialDelay + jitter)

	// Cap at max backoff
	if delay > t.maxBackoff {
		delay = t.maxBackoff
	}

	return delay
}

// Client returns the underlying HTTP client.
func (t *HTTPTransport) Client() *http.Client {
	return t.client
}

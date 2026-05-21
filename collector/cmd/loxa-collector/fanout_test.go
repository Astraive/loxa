package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"golang.org/x/time/rate"
)

type sequenceSink struct {
	mu     sync.Mutex
	errs   []error
	writes int
}

func (s *sequenceSink) Name() string { return "sequence" }

func (s *sequenceSink) WriteEvent(context.Context, []byte, *collectorevent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.writes
	s.writes++
	if idx < len(s.errs) {
		return s.errs[idx]
	}
	return nil
}

func (s *sequenceSink) Flush(context.Context) error { return nil }

func (s *sequenceSink) Close(context.Context) error { return nil }

func (s *sequenceSink) Writes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

func TestFanoutDeliveryPoliciesInDirectMode(t *testing.T) {
	tests := []struct {
		name           string
		policy         string
		primaryErr     error
		secondaryErr   error
		expectedStatus int
	}{
		{
			name:           "require_primary fails when primary fails",
			policy:         deliveryPolicyRequirePrimary,
			primaryErr:     errors.New("primary failed"),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "require_primary allows secondary failure",
			policy:         deliveryPolicyRequirePrimary,
			secondaryErr:   errors.New("secondary failed"),
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "require_all fails on secondary failure",
			policy:         deliveryPolicyRequireAll,
			secondaryErr:   errors.New("secondary failed"),
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "best_effort succeeds with secondary when primary fails",
			policy:         deliveryPolicyBestEffort,
			primaryErr:     errors.New("primary failed"),
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "best_effort fails when all outputs fail",
			policy:         deliveryPolicyBestEffort,
			primaryErr:     errors.New("primary failed"),
			secondaryErr:   errors.New("secondary failed"),
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testCollectorConfig()
			cfg.deliveryPolicy = tt.policy
			cfg.reliabilityMode = "direct"
			cfg.retryEnabled = false

			primary := &sequenceSink{errs: []error{tt.primaryErr}}
			secondary := &sequenceSink{errs: []error{tt.secondaryErr}}
			state := &collectorState{
				cfg:            cfg,
				ingestSink:     primary,
				secondarySinks: []namedSink{{Name: "secondary", Sink: secondary}},
				rateLimiter:    rate.NewLimiter(rate.Limit(1000), 1000),
				rng:            randSourceForTests(),
			}
			state.ready.Store(true)
			state.sinkHealthy.Store(true)
			state.spoolHealthy.Store(true)
			state.diskHealthy.Store(true)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"fanout.policy"}`))
			state.handleIngest(rec, req)
			if rec.Code != tt.expectedStatus {
				t.Fatalf("expected %d, got %d body=%s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFanoutFallbackTriggeredOnPolicyFailure(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.deliveryPolicy = deliveryPolicyRequirePrimary
	cfg.reliabilityMode = "direct"
	cfg.retryEnabled = false
	cfg.fallbackEnabled = true
	cfg.fallbackEnabled = true
	cfg.fallbackOnPolicyFail = true

	primary := &sequenceSink{errs: []error{errors.New("primary failed")}}
	fallback := &sequenceSink{}
	state := &collectorState{
		cfg:          cfg,
		ingestSink:   primary,
		fallbackSink: &namedSink{Name: "fallback", Sink: fallback},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	rec := httptest.NewRecorder()
	state.handleIngest(rec, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"fanout.fallback"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if fallback.Writes() != 1 {
		t.Fatalf("expected fallback write, got %d", fallback.Writes())
	}
}

func TestFanoutDLQOnSecondaryFailureInSpoolMode(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.deliveryPolicy = deliveryPolicyRequirePrimary
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true
	cfg.retryEnabled = false
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")
	cfg.dlqOnPrimaryFail = false
	cfg.dlqOnSecondaryFail = true
	cfg.dlqOnPolicyFail = false

	primary := &sequenceSink{}
	secondary := &sequenceSink{errs: []error{errors.New("secondary failed")}}
	state := &collectorState{
		cfg:            cfg,
		ingestSink:     primary,
		secondarySinks: []namedSink{{Name: "secondary", Sink: secondary}},
		rateLimiter:    rate.NewLimiter(rate.Limit(1000), 1000),
		rng:            randSourceForTests(),
		dedupeSeenAt:   make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	rec := httptest.NewRecorder()
	state.handleIngest(rec, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"fanout.secondary.dlq"}`)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	dlqLine := waitForDLQLine(t, cfg.dlqPath)
	var entry map[string]any
	if err := json.Unmarshal([]byte(dlqLine), &entry); err != nil {
		t.Fatalf("decode dlq: %v", err)
	}
	if !strings.Contains(entry["error"].(string), "secondary failed") {
		t.Fatalf("expected secondary error in dlq, got %#v", entry["error"])
	}
}

func TestFanoutDLQOnFallbackFailure(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "direct"
	cfg.retryEnabled = false
	cfg.deliveryPolicy = deliveryPolicyRequirePrimary
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")
	cfg.dlqOnPrimaryFail = false
	cfg.dlqOnPolicyFail = false
	cfg.dlqOnFallbackFail = true
	cfg.fallbackEnabled = true
	cfg.fallbackOnPrimaryFail = true

	primary := &sequenceSink{errs: []error{errors.New("primary failed")}}
	fallback := &sequenceSink{errs: []error{errors.New("fallback failed")}}
	state := &collectorState{
		cfg:          cfg,
		ingestSink:   primary,
		fallbackSink: &namedSink{Name: "fallback", Sink: fallback},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	rec := httptest.NewRecorder()
	state.handleIngest(rec, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"fanout.fallback.dlq"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	dlqLine := waitForDLQLine(t, cfg.dlqPath)
	var entry map[string]any
	if err := json.Unmarshal([]byte(dlqLine), &entry); err != nil {
		t.Fatalf("decode dlq: %v", err)
	}
	if !strings.Contains(entry["error"].(string), "fallback failed") {
		t.Fatalf("expected fallback error in dlq, got %#v", entry["error"])
	}
}

func waitForDLQLine(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				return lines[len(lines)-1]
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for dlq entry in %s", path)
	return ""
}

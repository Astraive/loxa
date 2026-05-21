//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"github.com/astraive/loxa-collector/internal/ingest"
	"golang.org/x/time/rate"
)

type integrationQueueSink struct {
	mu        sync.Mutex
	events    [][]byte
	attempts  int
	failUntil int
}

func (s *integrationQueueSink) Name() string { return "integration-queue" }

func (s *integrationQueueSink) WriteEvent(_ context.Context, encoded []byte, _ *collectorevent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	if s.attempts <= s.failUntil {
		return context.DeadlineExceeded
	}
	cp := append([]byte(nil), encoded...)
	s.events = append(s.events, cp)
	return nil
}

func (s *integrationQueueSink) Flush(context.Context) error { return nil }
func (s *integrationQueueSink) Close(context.Context) error { return nil }

func (s *integrationQueueSink) eventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *integrationQueueSink) writeAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts
}

func newQueueIntegrationState(cfg collectorConfig, sink *integrationQueueSink) *collectorState {
	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	return state
}

func postIngest(t *testing.T, state *collectorState, body string) (int, ingest.Response) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	state.handleIngest(rec, req)
	var resp ingest.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	return rec.Code, resp
}

func baseQueueIntegrationConfig() collectorConfig {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "queue"
	cfg.retryEnabled = true
	cfg.retryMaxAttempts = 2
	cfg.retryInitialBackoff = time.Millisecond
	cfg.retryMaxBackoff = time.Millisecond
	cfg.retryJitter = false
	return cfg
}

func TestCollectorIntegration_RedisDedupeSkipsDuplicateQueuePublish(t *testing.T) {
	mr := miniredis.RunT(t)
	sink := &integrationQueueSink{}
	cfg := baseQueueIntegrationConfig()
	cfg.dedupeEnabled = true
	cfg.dedupeBackend = "redis"
	cfg.dedupeRedisAddr = mr.Addr()
	cfg.dedupeRedisPrefix = "collector:dedupe:"
	state := newQueueIntegrationState(cfg, sink)

	payload := `[{"event_id":"evt-dedupe-1","schema_version":"v1","event_version":"v1","timestamp":"2026-05-18T12:30:45Z","service":"checkout","event":"checkout.request","kind":"http"},{"event_id":"evt-dedupe-1","schema_version":"v1","event_version":"v1","timestamp":"2026-05-18T12:30:45Z","service":"checkout","event":"checkout.request","kind":"http"}]`
	status, response := postIngest(t, state, payload)
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%+v", status, response)
	}
	if sink.eventCount() != 1 {
		t.Fatalf("expected 1 queued event, got %d", sink.eventCount())
	}
	if response.Duplicates != 1 || response.Accepted != 2 {
		t.Fatalf("expected one duplicate ack and two accepted events, got %+v", response)
	}
}

func TestCollectorIntegration_QueuesAcceptedEvents(t *testing.T) {
	sink := &integrationQueueSink{}
	cfg := baseQueueIntegrationConfig()
	cfg.dedupeEnabled = false
	state := newQueueIntegrationState(cfg, sink)

	payload := `[{"event_id":"evt-queue-1","schema_version":"v1","event_version":"v1","timestamp":"2026-05-18T12:30:45Z","service":"checkout","event":"checkout.request","kind":"http"},{"event_id":"evt-queue-2","schema_version":"v1","event_version":"v1","timestamp":"2026-05-18T12:30:45Z","service":"checkout","event":"checkout.completed","kind":"http"}]`
	status, response := postIngest(t, state, payload)
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%+v", status, response)
	}
	if sink.eventCount() != 2 {
		t.Fatalf("expected 2 queued events, got %d", sink.eventCount())
	}
}

func TestCollectorIntegration_RetriesQueuePublishOnTransientFailure(t *testing.T) {
	sink := &integrationQueueSink{failUntil: 1}
	cfg := baseQueueIntegrationConfig()
	state := newQueueIntegrationState(cfg, sink)

	status, response := postIngest(t, state, `{"event_id":"evt-retry-1","schema_version":"v1","event_version":"v1","timestamp":"2026-05-18T12:30:45Z","service":"checkout","event":"checkout.request","kind":"http"}`)
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%+v", status, response)
	}
	if sink.eventCount() != 1 {
		t.Fatalf("expected event queued after retry, got %d", sink.eventCount())
	}
	if sink.writeAttempts() < 2 {
		t.Fatalf("expected at least 2 queue write attempts, got %d", sink.writeAttempts())
	}
}

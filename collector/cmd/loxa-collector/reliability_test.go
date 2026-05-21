package main

import (
	"context"
	"encoding/json"
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

type capturedSink struct {
	mu     sync.Mutex
	events [][]byte
}

func (s *capturedSink) Name() string { return "captured" }
func (s *capturedSink) WriteEvent(_ context.Context, encoded []byte, _ *collectorevent.Event) error {
	cp := make([]byte, len(encoded))
	copy(cp, encoded)
	s.mu.Lock()
	s.events = append(s.events, cp)
	s.mu.Unlock()
	return nil
}
func (s *capturedSink) Flush(_ context.Context) error { return nil }
func (s *capturedSink) Close(_ context.Context) error { return nil }

func (s *capturedSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func TestHandleIngestDedupeWindowExpiry(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.dedupeEnabled = true
	cfg.dedupeKey = "event_id"
	cfg.dedupeWindow = 100 * time.Millisecond

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)

	// first ingest
	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-dead","event":"a"}`)))
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}

	// duplicate immediately should be deduped
	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-dead","event":"a"}`)))
	if state.metrics.eventsDeduped.Load() != 1 {
		t.Fatalf("expected 1 deduped, got %d", state.metrics.eventsDeduped.Load())
	}

	// wait for window to expire
	time.Sleep(200 * time.Millisecond)
	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-dead","event":"a"}`)))
	if len(sink.events) != 2 {
		t.Fatalf("expected 2 events after expiry, got %d", len(sink.events))
	}
}

func TestReadyFlipsAfterSinkFailure(t *testing.T) {
	cfg := testCollectorConfig()
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  errSink{err: context.DeadlineExceeded},
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	// initially ready
	rec := httptest.NewRecorder()
	state.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// cause sink failure
	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`)))
	// readiness should flip
	rec = httptest.NewRecorder()
	state.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after sink failure, got %d", rec.Code)
	}
}

func TestSpoolCrashReplay(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true

	var state1 collectorState
	state1.cfg = cfg
	state1.ingestSink = errSink{err: context.DeadlineExceeded}
	state1.rateLimiter = rate.NewLimiter(rate.Limit(1000), 1000)
	state1.rng = randSourceForTests()
	state1.dedupeSeenAt = make(map[string]time.Time)
	state1.ready.Store(true)
	state1.sinkHealthy.Store(true)
	state1.spoolHealthy.Store(true)
	state1.diskHealthy.Store(true)
	if err := state1.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}

	// ingest event with failing sink (spools to disk)
	state1.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"replay-1","event":"a"}`)))
	state1.closeReliability()

	// simulate restart with working sink
	sink := &capturedSink{}
	var state2 collectorState
	state2.cfg = cfg
	state2.ingestSink = sink
	state2.rateLimiter = rate.NewLimiter(rate.Limit(1000), 1000)
	state2.rng = randSourceForTests()
	state2.dedupeSeenAt = make(map[string]time.Time)
	state2.ready.Store(true)
	state2.sinkHealthy.Store(true)
	state2.spoolHealthy.Store(true)
	state2.diskHealthy.Store(true)
	if err := state2.initReliability(); err != nil {
		t.Fatalf("init reliability on restart: %v", err)
	}
	defer state2.closeReliability()

	// wait for replay delivery
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sink.Len() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := sink.Len(); got != 1 {
		t.Fatalf("expected replayed event, got %d", got)
	}
}

func TestDLQContainsRawAndReason(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true
	cfg.retryEnabled = true
	cfg.retryMaxAttempts = 2
	cfg.retryInitialBackoff = time.Millisecond
	cfg.retryMaxBackoff = time.Millisecond
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   errSink{err: context.DeadlineExceeded},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"dlq-1","event":"a"}`)))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if state.metrics.sinkWriteErrors.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if state.metrics.sinkWriteErrors.Load() == 0 {
		t.Fatalf("sinkWriteErrors never incremented (retry loop may be infinite)")
	}

	// read DLQ file
	rawDLQ, _ := os.ReadFile(cfg.dlqPath)
	lines := strings.Split(strings.TrimSpace(string(rawDLQ)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected DLQ entries")
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("decode dlq: %v", err)
	}
	if last["raw"] == nil {
		t.Fatalf("expected raw event in DLQ")
	}
	if last["error"] == nil {
		t.Fatalf("expected error reason in DLQ")
	}
}

func TestP0CollectorReadinessFailsWhenSpoolOverLimit(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 32
	cfg.spoolFsync = true

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   &fakeSink{},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
		dedupeSeenAt: make(map[string]time.Time),
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
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"this-payload-is-longer-than-the-spool-limit"}`))
	state.handleIngest(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if got := state.metrics.spoolBytes.Load(); got <= cfg.maxSpoolBytes {
		t.Fatalf("expected spool bytes > %d, got %d", cfg.maxSpoolBytes, got)
	}
	if state.effectiveSpoolHealthy() {
		t.Fatalf("expected spool health to fail when spool exceeds max bytes")
	}

	ready := httptest.NewRecorder()
	state.handleReady(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when spool is unhealthy, got %d", ready.Code)
	}
}

func TestSpoolQueueBytesLimitDropsToDLQ(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.maxQueueBytes = 8
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   &capturedSink{},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	state.enqueueDelivery([]byte(`{"event":"payload-too-large-for-queue-budget"}`))

	rawDLQ, err := os.ReadFile(cfg.dlqPath)
	if err != nil {
		t.Fatalf("read dlq: %v", err)
	}
	if !strings.Contains(string(rawDLQ), "delivery queue bytes exceeded") {
		t.Fatalf("expected queue bytes DLQ reason, got %s", string(rawDLQ))
	}
}

func TestSpoolReplaySkipsInvalidLinesAndCompacts(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true

	spoolPath := filepath.Join(cfg.spoolDir, cfg.spoolFile)
	if err := os.WriteFile(spoolPath, []byte("not-json\n{\"event_id\":\"replay-valid\",\"event\":\"ok\"}\n"), 0o600); err != nil {
		t.Fatalf("seed spool: %v", err)
	}

	sink := &capturedSink{}
	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sink.Len() == 1 && state.metrics.spoolBytes.Load() == 0 {
			quarantinePath := spoolPath + ".bad.ndjson"
			raw, err := os.ReadFile(quarantinePath)
			if err != nil {
				t.Fatalf("expected quarantine file: %v", err)
			}
			if !strings.Contains(string(raw), "invalid_spool_record") || !strings.Contains(string(raw), "not-json") {
				t.Fatalf("expected quarantined invalid spool line, got %s", string(raw))
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected one replayed event and compacted spool, got replayed=%d spool_bytes=%d", sink.Len(), state.metrics.spoolBytes.Load())
}

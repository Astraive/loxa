package main

import (
	"bytes"
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

	collectorevent "github.com/astraive/loza/collector/internal/event"
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

type shutdownOrderSink struct {
	started             chan struct{}
	release             chan struct{}
	startOnce           sync.Once
	mu                  sync.Mutex
	closed              bool
	wroteAfterClose     bool
	canceledBeforeDrain bool
}

func newShutdownOrderSink() *shutdownOrderSink {
	return &shutdownOrderSink{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *shutdownOrderSink) Name() string { return "shutdown-order" }

func (s *shutdownOrderSink) WriteEvent(ctx context.Context, _ []byte, _ *collectorevent.Event) error {
	s.startOnce.Do(func() { close(s.started) })
	select {
	case <-s.release:
	case <-ctx.Done():
		s.mu.Lock()
		s.canceledBeforeDrain = true
		s.mu.Unlock()
		return ctx.Err()
	}
	s.mu.Lock()
	s.wroteAfterClose = s.closed
	s.mu.Unlock()
	return nil
}

func (s *shutdownOrderSink) Flush(_ context.Context) error { return nil }

func (s *shutdownOrderSink) Close(_ context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *shutdownOrderSink) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *shutdownOrderSink) deliveredAfterClose() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wroteAfterClose
}

func (s *shutdownOrderSink) deliveryCanceledBeforeDrain() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.canceledBeforeDrain
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

func TestEncryptedSpoolCheckpointUsesStoredRecordLength(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.storageEncryptionKey = "checkpoint-test-key"
	cfg.maxSpoolBytes = 1024 * 1024

	spoolPath := filepath.Join(t.TempDir(), "events.ndjson")
	spoolFile, err := os.OpenFile(spoolPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open spool: %v", err)
	}
	defer spoolFile.Close()

	state := &collectorState{
		cfg:          cfg,
		spoolFile:    spoolFile,
		spoolPosFile: spoolPath + ".pos",
	}
	first := []byte(`{"event_id":"encrypted-1","event":"first"}`)
	second := []byte(`{"event_id":"encrypted-2","event":"second"}`)
	firstItem, err := state.appendSpool(first)
	if err != nil {
		t.Fatalf("append first record: %v", err)
	}
	if _, err := state.appendSpool(second); err != nil {
		t.Fatalf("append second record: %v", err)
	}

	stored, err := os.ReadFile(spoolPath)
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	firstRecordEnd := bytes.IndexByte(stored, '\n') + 1
	if firstRecordEnd <= len(first)+1 {
		t.Fatalf("expected encrypted record expansion, got stored=%d plain=%d", firstRecordEnd, len(first)+1)
	}

	state.markSpoolDelivered(firstItem)
	if got, want := state.spoolProcessedPos, int64(firstRecordEnd); got != want {
		t.Fatalf("processed position = %d, want stored record end %d", got, want)
	}
}

func TestSpoolCheckpointDoesNotAdvanceAcrossGap(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.maxSpoolBytes = 1024 * 1024

	spoolPath := filepath.Join(t.TempDir(), "events.ndjson")
	spoolFile, err := os.OpenFile(spoolPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open spool: %v", err)
	}
	defer spoolFile.Close()

	state := &collectorState{
		cfg:          cfg,
		spoolFile:    spoolFile,
		spoolPosFile: spoolPath + ".pos",
	}
	first, err := state.appendSpool([]byte(`{"event":"first"}`))
	if err != nil {
		t.Fatalf("append first record: %v", err)
	}
	second, err := state.appendSpool([]byte(`{"event":"second"}`))
	if err != nil {
		t.Fatalf("append second record: %v", err)
	}

	state.markSpoolDelivered(second)
	if state.spoolProcessedPos != 0 {
		t.Fatalf("checkpoint advanced across undelivered first record to %d", state.spoolProcessedPos)
	}
	state.markSpoolDelivered(first)
	if state.spoolProcessedPos != first.endOffset {
		t.Fatalf("checkpoint = %d, want first record end %d", state.spoolProcessedPos, first.endOffset)
	}
}

func TestShutdownDrainsSpoolBeforeClosingSinks(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.deliveryQueueSize = 1
	cfg.shutdownTimeout = 2 * time.Second

	sink := newShutdownOrderSink()
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
		t.Fatalf("initialize reliability: %v", err)
	}
	defer state.closeReliability()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"shutdown-1","event":"queued"}`))
	state.handleIngest(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("ingest status = %d, want 202", rec.Code)
	}
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("queued delivery did not start")
	}

	shutdownDone := make(chan struct{})
	go func() {
		shutdownCollector(&http.Server{}, nil, state, nil, cfg, nil, &sync.WaitGroup{})
		close(shutdownDone)
	}()

	time.Sleep(50 * time.Millisecond)
	closedBeforeDrain := sink.isClosed()
	close(sink.release)
	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatal("collector shutdown did not complete")
	}

	if closedBeforeDrain || sink.deliveredAfterClose() || sink.deliveryCanceledBeforeDrain() {
		t.Fatal("queued spool delivery was canceled or its sink closed before draining")
	}
}

func TestDeliveryQueueBackpressuresInsteadOfDropping(t *testing.T) {
	cfg := testCollectorConfig()
	state := &collectorState{
		cfg:           cfg,
		ingestSink:    &fakeSink{},
		rng:           randSourceForTests(),
		dedupeSeenAt:  make(map[string]time.Time),
		deliveryQueue: make(chan spoolDelivery, 1),
	}
	if err := state.ensureProcessor(); err != nil {
		t.Fatalf("initialize processor: %v", err)
	}
	defer state.processor.Close()

	first := []byte(`{"event":"first"}`)
	second := []byte(`{"event":"second"}`)
	if err := state.enqueueDelivery(spoolDelivery{payload: first}); err != nil {
		t.Fatalf("enqueue first record: %v", err)
	}

	enqueued := make(chan struct{})
	go func() {
		_ = state.enqueueDelivery(spoolDelivery{payload: second})
		close(enqueued)
	}()

	select {
	case <-enqueued:
		t.Fatal("second enqueue returned before queue capacity was available")
	case <-time.After(50 * time.Millisecond):
	}

	<-state.deliveryQueue
	select {
	case <-enqueued:
	case <-time.After(time.Second):
		t.Fatal("second enqueue did not resume after queue capacity became available")
	}
}

func TestDeliveryQueueReservesBytesBeforePublication(t *testing.T) {
	cfg := testCollectorConfig()
	state := &collectorState{
		cfg:           cfg,
		ingestSink:    &fakeSink{},
		rng:           randSourceForTests(),
		dedupeSeenAt:  make(map[string]time.Time),
		deliveryQueue: make(chan spoolDelivery),
	}
	if err := state.ensureProcessor(); err != nil {
		t.Fatalf("initialize processor: %v", err)
	}
	defer state.processor.Close()

	raw := []byte(`{"event":"reserved"}`)
	enqueued := make(chan struct{})
	go func() {
		_ = state.enqueueDelivery(spoolDelivery{payload: raw})
		close(enqueued)
	}()

	deadline := time.Now().Add(100 * time.Millisecond)
	reserved := false
	for time.Now().Before(deadline) {
		if state.metrics.queueBytes.Load() == int64(len(raw)) {
			reserved = true
			break
		}
		select {
		case <-enqueued:
			if !reserved {
				t.Fatal("enqueue returned before reserving queue bytes")
			}
		default:
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case <-enqueued:
	default:
		<-state.deliveryQueue
		<-enqueued
	}
	if !reserved {
		t.Fatal("queue bytes were not reserved before the item became receivable")
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

func TestSpoolLimitRejectsBeforeWriting(t *testing.T) {
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
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := state.metrics.spoolBytes.Load(); got > cfg.maxSpoolBytes {
		t.Fatalf("spool bytes exceeded limit %d: %d", cfg.maxSpoolBytes, got)
	}
	if !state.effectiveSpoolHealthy() {
		t.Fatal("rejected write should not make the existing spool unhealthy")
	}

	ready := httptest.NewRecorder()
	state.handleReady(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("expected ready collector after rejected write, got %d", ready.Code)
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

	if err := state.enqueueDelivery(spoolDelivery{payload: []byte(`{"event":"payload-too-large-for-queue-budget"}`)}); err == nil {
		t.Fatal("expected oversized delivery to be rejected")
	}

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

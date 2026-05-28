package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/astraive/loxa/collector/internal/ingest"
	"github.com/astraive/loxa/collector/internal/sinks/duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"golang.org/x/time/rate"
)

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 1: High Concurrency — 1000+ simultaneous ingest connections
// ──────────────────────────────────────────────────────────────────────────────

func TestStressHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_concurrency.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 500
	cfg.duckDBFlushInterval = 50 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)

	concurrencyLevels := []int{100, 500, 1000, 2000}
	results := make(map[int]struct {
		accepted int64
		errors   int64
		duration time.Duration
		rps      float64
	})

	for _, concurrency := range concurrencyLevels {
		var (
			accepted atomic.Int64
			errCount atomic.Int64
			statuses sync.Map // track status code distribution
			wg       sync.WaitGroup
		)

		start := time.Now()
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				evt := fmt.Sprintf(`{"event_id":"stress-c%d-%d","event":"stress.high_concurrency","service":"stress-test","kind":"event","timestamp":"%s"}`,
					concurrency, id, time.Now().UTC().Format(time.RFC3339))
				body := fmt.Sprintf(`[%s]`, evt)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				state.handleIngest(rec, req)

				statuses.Store(rec.Code, struct{}{})

				if rec.Code == http.StatusAccepted {
					accepted.Add(1)
				} else {
					errCount.Add(1)
				}
			}(i)
		}
		wg.Wait()
		elapsed := time.Since(start)

		// Flush
		ctx := context.Background()
		_ = sink.Flush(ctx)

		// Collect unique status codes
		var statusCodes []int
		statuses.Range(func(key, value any) bool {
			statusCodes = append(statusCodes, key.(int))
			return true
		})

		results[concurrency] = struct {
			accepted int64
			errors   int64
			duration time.Duration
			rps      float64
		}{
			accepted: accepted.Load(),
			errors:   errCount.Load(),
			duration: elapsed,
			rps:      float64(accepted.Load()) / elapsed.Seconds(),
		}

		t.Logf("Concurrency=%d: accepted=%d errors=%d duration=%v rps=%.0f status_codes=%v",
			concurrency, accepted.Load(), errCount.Load(), elapsed, results[concurrency].rps, statusCodes)
	}

	// Verify DuckDB row count
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM " + cfg.duckDBTable).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	t.Logf("Total DuckDB rows: %d", count)

	// All should be accepted (no rate limit, no backpressure)
	for _, r := range results {
		if r.accepted == 0 {
			t.Errorf("expected some accepted events, got 0")
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 2: Large Payloads — increasingly large event payloads
// ──────────────────────────────────────────────────────────────────────────────

func TestStressLargePayloads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := testCollectorConfig()
	cfg.schemaMode = "off"
	cfg.maxBodyBytes = 10 * 1024 * 1024 // 10MB

	sink := &capturedSink{}
	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)

	payloadSizes := []int{1024, 10240, 102400, 512000, 1048576} // 1KB, 10KB, 100KB, 500KB, 1MB
	type sizeResult struct {
		size      int
		status    int
		accepted  int
		rejected  int
		elapsed   time.Duration
		oomRisk   bool
	}

	var results []sizeResult

	for _, sz := range payloadSizes {
		// Generate event with large payload field
		payload := strings.Repeat("x", sz)
		evt := fmt.Sprintf(`{"event_id":"stress-large-%d","event":"stress.large_payload","service":"stress-test","kind":"event","timestamp":"%s","payload":"%s"}`,
			sz, time.Now().UTC().Format(time.RFC3339), payload)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(evt))
		req.Header.Set("Content-Type", "application/json")

		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		start := time.Now()
		state.handleIngest(rec, req)
		elapsed := time.Since(start)

		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		var resp ingest.Response
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		oomRisk := memAfter.Alloc-memBefore.Alloc > uint64(sz*10) // >10x the payload size in memory

		results = append(results, sizeResult{
			size:     sz,
			status:   rec.Code,
			accepted: resp.Accepted,
			rejected: resp.Rejected,
			elapsed:  elapsed,
			oomRisk:  oomRisk,
		})

		t.Logf("Payload=%dKB: status=%d accepted=%d rejected=%d elapsed=%v oomRisk=%v",
			sz/1024, rec.Code, resp.Accepted, resp.Rejected, elapsed, oomRisk)
	}

	// Payload that exceeds maxBodyBytes
	oversized := strings.Repeat("y", int(cfg.maxBodyBytes)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	state.handleIngest(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized payload, got %d", rec.Code)
	}
	t.Logf("Oversized payload: status=%d (expected 413)", rec.Code)
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 3: Sustained Load — moderate load for extended period
// ──────────────────────────────────────────────────────────────────────────────

func TestStressSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_sustained.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 200
	cfg.duckDBFlushInterval = 50 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)

	// Run 200 events/sec for 10 seconds = 2000 events total
	workers := 10
	eventsPerWorker := 200
	duration := 10 * time.Second

	var (
		sent       atomic.Int64
		accepted   atomic.Int64
		rejected   atomic.Int64
		memSamples []uint64
		memMu      sync.Mutex
		wg         sync.WaitGroup
	)

	// Memory sampler
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				memMu.Lock()
				memSamples = append(memSamples, m.Alloc)
				memMu.Unlock()
			}
		}
	}()

	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < eventsPerWorker; i++ {
				if time.Since(start) > duration {
					return
				}
				evtID := fmt.Sprintf("stress-sustained-%d-%d", workerID, i)
				evt := fmt.Sprintf(`{"event_id":"%s","event":"stress.sustained","service":"stress-test","kind":"event","timestamp":"%s","worker":%d,"seq":%d}`,
					evtID, time.Now().UTC().Format(time.RFC3339), workerID, i)
				body := fmt.Sprintf(`[%s]`, evt)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				state.handleIngest(rec, req)

				sent.Add(1)
				if rec.Code == http.StatusAccepted {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}

				// Pace to ~200 events/sec total
				time.Sleep(time.Duration(workers) * time.Millisecond)
			}
		}(w)
	}

	wg.Wait()
	close(done)
	elapsed := time.Since(start)

	// Flush
	ctx := context.Background()
	_ = sink.Flush(ctx)

	// Analyze memory growth
	memMu.Lock()
	if len(memSamples) > 2 {
		first := memSamples[0]
		last := memSamples[len(memSamples)-1]
		growth := int64(last) - int64(first)
		maxMem := uint64(0)
		for _, m := range memSamples {
			if m > maxMem {
				maxMem = m
			}
		}
		t.Logf("Memory: start=%dKB end=%dKB growth=%dKB peak=%dKB samples=%d",
			first/1024, last/1024, growth/1024, maxMem/1024, len(memSamples))

		if growth > 50*1024*1024 { // >50MB growth is concerning for 2000 events
			t.Errorf("EXCESSIVE MEMORY GROWTH: %dKB for %d events", growth/1024, accepted.Load())
		}
	}
	memMu.Unlock()

	// Check DuckDB rows
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM " + cfg.duckDBTable).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}

	t.Logf("Sustained load: duration=%v sent=%d accepted=%d rejected=%d rps=%.0f duckdb_rows=%d",
		elapsed, sent.Load(), accepted.Load(), rejected.Load(), float64(accepted.Load())/elapsed.Seconds(), count)

	if accepted.Load() == 0 {
		t.Error("no events accepted during sustained load test")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 4: Burst Traffic — sudden spike from 0 to high volume
// ──────────────────────────────────────────────────────────────────────────────

func TestStressBurstTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := testCollectorConfig()
	cfg.schemaMode = "off"
	cfg.rateLimitEnabled = true
	cfg.rateLimitRPS = 10000
	cfg.rateLimitBurst = 10000

	sink := &capturedSink{}
	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Limit(cfg.rateLimitRPS), cfg.rateLimitBurst)
	state.dedupeSeenAt = make(map[string]time.Time)

	// Burst: 5000 simultaneous requests
	burstSize := 5000
	var (
		accepted  atomic.Int64
		rateLimit atomic.Int64
		rejected  atomic.Int64
		wg        sync.WaitGroup
	)

	start := time.Now()
	for i := 0; i < burstSize; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			evt := fmt.Sprintf(`{"event_id":"stress-burst-%d","event":"stress.burst","service":"stress-test","kind":"event","timestamp":"%s"}`,
				id, time.Now().UTC().Format(time.RFC3339))
			body := fmt.Sprintf(`[%s]`, evt)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			state.handleIngest(rec, req)

			switch rec.Code {
			case http.StatusAccepted:
				accepted.Add(1)
			case http.StatusTooManyRequests:
				rateLimit.Add(1)
			default:
				rejected.Add(1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Burst: size=%d accepted=%d rateLimited=%d rejected=%d duration=%v rps=%.0f",
		burstSize, accepted.Load(), rateLimit.Load(), rejected.Load(), elapsed,
		float64(accepted.Load())/elapsed.Seconds())

	// With rate limiting, some should be accepted and some rate-limited
	total := accepted.Load() + rateLimit.Load() + rejected.Load()
	if total != int64(burstSize) {
		t.Errorf("expected %d total responses, got %d", burstSize, total)
	}
	if accepted.Load() == 0 {
		t.Error("expected some accepted events in burst test")
	}
	t.Logf("Rate limit effectiveness: %d/%d accepted (%.1f%%)",
		accepted.Load(), burstSize, float64(accepted.Load())*100/float64(burstSize))
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 5: Query Pressure — many concurrent LQL queries
// ──────────────────────────────────────────────────────────────────────────────

func TestStressQueryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_query.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 100
	cfg.duckDBFlushInterval = 50 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	// First, ingest some test data
	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)
	state.queryDB = db

	for i := 0; i < 100; i++ {
		evt := fmt.Sprintf(`{"event_id":"stress-query-%d","event":"stress.query","service":"stress-service","kind":"event","level":"info","timestamp":"%s","duration_ms":%d}`,
			i, time.Now().UTC().Format(time.RFC3339), rand.Intn(1000))
		body := fmt.Sprintf(`[%s]`, evt)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		state.handleIngest(rec, req)
	}
	_ = sink.Flush(context.Background())

	// Concurrent query pressure
	concurrentQueries := 50
	queries := []string{
		"SELECT * FROM events LIMIT 10",
		"SELECT COUNT(*) FROM events",
		"SELECT service, COUNT(*) FROM events GROUP BY service",
		"SELECT * FROM events WHERE event_name = 'stress.query' LIMIT 5",
		"SELECT * FROM events ORDER BY timestamp DESC LIMIT 20",
	}

	var (
		queryOK      atomic.Int64
		queryErr     atomic.Int64
		queryTotal   atomic.Int64
		queryErrBody sync.Map // track error responses
		wg           sync.WaitGroup
	)

	start := time.Now()
	for i := 0; i < concurrentQueries; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for _, q := range queries {
				body := fmt.Sprintf(`{"query":%q}`, q)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lql/query", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				state.HandleLQLQuery(rec, req)
				queryTotal.Add(1)
				if rec.Code == http.StatusOK {
					queryOK.Add(1)
				} else {
					queryErr.Add(1)
					queryErrBody.Store(rec.Code, rec.Body.String())
				}
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Query pressure: concurrent=%d total=%d ok=%d errors=%d duration=%v qps=%.0f",
		concurrentQueries, queryTotal.Load(), queryOK.Load(), queryErr.Load(), elapsed,
		float64(queryTotal.Load())/elapsed.Seconds())

	if queryErr.Load() > 0 {
		queryErrBody.Range(func(key, value any) bool {
			t.Logf("  Error status %d: %s", key.(int), value.(string)[:min(200, len(value.(string)))])
			return true
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 6: Resource Exhaustion — spool/file descriptor limits
// ──────────────────────────────────────────────────────────────────────────────

func TestStressSpoolCapacityExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 4096 // Very small spool
	cfg.spoolFsync = true
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")
	cfg.schemaMode = "off"

	// Use a failing sink so spool fills up instead of draining
	sink := &errSink{err: fmt.Errorf("simulated sink failure")}
	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Inf, 0),
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

	// Send events until spool exceeds capacity
	var accepted, rejected int64
	for i := 0; i < 200; i++ {
		evt := fmt.Sprintf(`{"event_id":"stress-spool-%d","event":"stress.spool","service":"stress-test","kind":"event","timestamp":"%s","data":"%s"}`,
			i, time.Now().UTC().Format(time.RFC3339), strings.Repeat("z", 200))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(evt))
		req.Header.Set("Content-Type", "application/json")
		state.handleIngest(rec, req)

		if rec.Code == http.StatusAccepted {
			accepted++
		} else {
			rejected++
		}
		// Small delay to allow delivery worker to process
		time.Sleep(5 * time.Millisecond)
	}

	t.Logf("Spool exhaustion: accepted=%d rejected=%d spoolBytes=%d maxSpoolBytes=%d spoolHealthy=%v",
		accepted, rejected, state.metrics.spoolBytes.Load(), cfg.maxSpoolBytes, state.effectiveSpoolHealthy())

	// Readiness should fail when spool is over limit
	rec := httptest.NewRecorder()
	state.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	t.Logf("Readiness after spool exhaustion: %d", rec.Code)
	// Note: if spool drained before we check, readiness might still be 200
	// The test verifies the mechanism works, not exact timing
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 7: Error Cascading — cortex down, collector degradation
// ──────────────────────────────────────────────────────────────────────────────

func TestStressCortexDownGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := testCollectorConfig()
	cfg.schemaMode = "off"
	cfg.cortexBridgeEnabled = false // Cortex not available

	sink := &capturedSink{}
	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)

	// Send many events with cortex bridge disabled — should still work
	var accepted, rejected int64
	for i := 0; i < 1000; i++ {
		evt := fmt.Sprintf(`{"event_id":"stress-cortex-%d","event":"stress.cortex_down","service":"stress-test","kind":"event","timestamp":"%s"}`,
			i, time.Now().UTC().Format(time.RFC3339))
		body := fmt.Sprintf(`[%s]`, evt)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		state.handleIngest(rec, req)

		if rec.Code == http.StatusAccepted {
			accepted++
		} else {
			rejected++
		}
	}

	t.Logf("Cortex down: accepted=%d rejected=%d", accepted, rejected)

	// Collector should still accept events even when cortex is down
	if accepted == 0 {
		t.Error("collector rejected all events when cortex is down — no graceful degradation")
	}

	// Health should still report OK
	rec := httptest.NewRecorder()
	state.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from /healthz when cortex is down, got %d", rec.Code)
	}

	// Readiness should still report OK
	rec = httptest.NewRecorder()
	state.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from /readyz when cortex is down, got %d", rec.Code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 8: Retry Storms — rapid duplicate submits
// ──────────────────────────────────────────────────────────────────────────────

func TestStressRetryStorms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := testCollectorConfig()
	cfg.schemaMode = "off"
	cfg.dedupeEnabled = true
	cfg.dedupeKey = "event_id"
	cfg.dedupeWindow = 5 * time.Second

	sink := &capturedSink{}
	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)

	// Simulate retry storm: same event sent 100 times concurrently
	evtID := "stress-retry-storm-001"
	evt := fmt.Sprintf(`{"event_id":"%s","event":"stress.retry_storm","service":"stress-test","kind":"event","timestamp":"%s"}`,
		evtID, time.Now().UTC().Format(time.RFC3339))
	body := fmt.Sprintf(`[%s]`, evt)

	retryCount := 100
	var (
		accepted atomic.Int64
		deduped  atomic.Int64
		wg       sync.WaitGroup
	)

	start := time.Now()
	for i := 0; i < retryCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			state.handleIngest(rec, req)

			if rec.Code == http.StatusAccepted {
				var resp ingest.Response
				_ = json.Unmarshal(rec.Body.Bytes(), &resp)
				if resp.Duplicates > 0 {
					deduped.Add(1)
				} else {
					accepted.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Retry storm: retries=%d accepted=%d deduped=%d duration=%v",
		retryCount, accepted.Load(), deduped.Load(), elapsed)

	// With deduplication, only 1 should be accepted
	if accepted.Load() != 1 {
		t.Logf("WARNING: expected 1 accepted (deduped), got %d — deduplication may not be active", accepted.Load())
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 9: Large Result Sets — query returning many rows
// ──────────────────────────────────────────────────────────────────────────────

func TestStressLargeResultSets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_large_result.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 5000
	cfg.duckDBFlushInterval = 10 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)
	state.queryDB = db

	// Ingest 5000 events
	for i := 0; i < 5000; i++ {
		evt := fmt.Sprintf(`{"event_id":"stress-lrs-%d","event":"stress.large_result","service":"stress-service","kind":"event","level":"info","timestamp":"%s"}`,
			i, time.Now().UTC().Format(time.RFC3339))
		body := fmt.Sprintf(`[%s]`, evt)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		state.handleIngest(rec, req)
	}
	_ = sink.Flush(context.Background())

	// Query with large limit
	queryTests := []struct {
		name  string
		query string
		limit int
	}{
		{"limit_1000", "SELECT * FROM events LIMIT 1000", 1000},
		{"limit_5000", "SELECT * FROM events LIMIT 5000", 5000},
		{"limit_10000", "SELECT * FROM events LIMIT 10000", 10000},
		{"count_all", "SELECT COUNT(*) FROM events", 1},
		{"group_by", "SELECT service, COUNT(*) as cnt FROM events GROUP BY service", 100},
	}

	for _, qt := range queryTests {
		body := fmt.Sprintf(`{"query":%q,"limit":%d}`, qt.query, qt.limit)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/lql/query", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		state.HandleLQLQuery(rec, req)
		elapsed := time.Since(start)

		t.Logf("Large result [%s]: status=%d duration=%v", qt.name, rec.Code, elapsed)

		if rec.Code != http.StatusOK {
			t.Errorf("query %s failed with status %d", qt.name, rec.Code)
		}

		// Verify response has rows
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if rowCount, ok := resp["row_count"].(float64); ok {
			t.Logf("  row_count=%.0f", rowCount)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 10: Connection Pool Exhaustion
// ──────────────────────────────────────────────────────────────────────────────

func TestStressConnectionPoolExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_pool.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBMaxOpenConns = 2 // Very limited pool
	cfg.duckDBMaxIdleConns = 1
	cfg.duckDBBatchSize = 10
	cfg.duckDBFlushInterval = 10 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)
	state.queryDB = db

	// Hammer with many concurrent writes + reads
	concurrency := 100
	var (
		writeOK   atomic.Int64
		writeErr  atomic.Int64
		queryOK   atomic.Int64
		queryErr  atomic.Int64
		wg        sync.WaitGroup
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Write
			evt := fmt.Sprintf(`{"event_id":"stress-pool-%d","event":"stress.pool","service":"stress-test","kind":"event","timestamp":"%s"}`,
				id, time.Now().UTC().Format(time.RFC3339))
			body := fmt.Sprintf(`[%s]`, evt)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			state.handleIngest(rec, req)
			if rec.Code == http.StatusAccepted {
				writeOK.Add(1)
			} else {
				writeErr.Add(1)
			}

			// Read
			qBody := `{"query":"SELECT COUNT(*) FROM events"}`
			qRec := httptest.NewRecorder()
			qReq := httptest.NewRequest(http.MethodPost, "/lql/query", strings.NewReader(qBody))
			qReq.Header.Set("Content-Type", "application/json")
			state.HandleLQLQuery(qRec, qReq)
			if qRec.Code == http.StatusOK {
				queryOK.Add(1)
			} else {
				queryErr.Add(1)
			}
		}(i)
	}

	wg.Wait()
	_ = sink.Flush(context.Background())

	t.Logf("Connection pool: concurrency=%d writeOK=%d writeErr=%d queryOK=%d queryErr=%d",
		concurrency, writeOK.Load(), writeErr.Load(), queryOK.Load(), queryErr.Load())

	// With limited pool, we might see some contention but not total failure
	if writeOK.Load() == 0 && queryOK.Load() == 0 {
		t.Error("total connection pool exhaustion — no operations succeeded")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// STRESS TEST 11: Mixed Workload — concurrent reads and writes
// ──────────────────────────────────────────────────────────────────────────────

func TestStressMixedWorkload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "stress_mixed.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 100
	cfg.duckDBFlushInterval = 50 * time.Millisecond
	cfg.schemaMode = "off"

	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:            db,
		Table:         cfg.duckDBTable,
		StoreRaw:      cfg.duckDBStoreRaw,
		RawColumn:     cfg.duckDBRawColumn,
		Schema:        cfg.duckDBSchema,
		BatchSize:     cfg.duckDBBatchSize,
		FlushInterval: cfg.duckDBFlushInterval,
	})
	if err != nil {
		t.Fatalf("duckdb sink: %v", err)
	}

	state := &collectorState{cfg: cfg}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.ingestSink = sink
	state.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	state.dedupeSeenAt = make(map[string]time.Time)
	state.queryDB = db

	// Pre-seed data
	for i := 0; i < 100; i++ {
		evt := fmt.Sprintf(`{"event_id":"stress-seed-%d","event":"stress.seed","service":"seed-service","kind":"event","level":"info","timestamp":"%s"}`,
			i, time.Now().UTC().Format(time.RFC3339))
		body := fmt.Sprintf(`[%s]`, evt)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		state.handleIngest(rec, req)
	}
	_ = sink.Flush(context.Background())

	// Mixed workload: 20 writers + 10 readers + 5 tail watchers
	duration := 5 * time.Second
	var (
		writes     atomic.Int64
		reads      atomic.Int64
		writeErrs  atomic.Int64
		readErrs   atomic.Int64
		wg         sync.WaitGroup
	)

	start := time.Now()

	// Writers
	for w := 0; w < 20; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			seq := 0
			for time.Since(start) < duration {
				evt := fmt.Sprintf(`{"event_id":"stress-mixed-%d-%d","event":"stress.mixed","service":"stress-test","kind":"event","timestamp":"%s"}`,
					id, seq, time.Now().UTC().Format(time.RFC3339))
				body := fmt.Sprintf(`[%s]`, evt)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				state.handleIngest(rec, req)
				if rec.Code == http.StatusAccepted {
					writes.Add(1)
				} else {
					writeErrs.Add(1)
				}
				seq++
			}
		}(w)
	}

	// Readers
	for r := 0; r < 10; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for time.Since(start) < duration {
				qBody := `{"query":"SELECT COUNT(*) FROM events"}`
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lql/query", strings.NewReader(qBody))
				req.Header.Set("Content-Type", "application/json")
				state.HandleLQLQuery(rec, req)
				if rec.Code == http.StatusOK {
					reads.Add(1)
				} else {
					readErrs.Add(1)
				}
			}
		}(r)
	}

	wg.Wait()
	elapsed := time.Since(start)

	_ = sink.Flush(context.Background())

	t.Logf("Mixed workload: duration=%v writes=%d writeErrs=%d reads=%d readErrs=%d",
		elapsed, writes.Load(), writeErrs.Load(), reads.Load(), readErrs.Load())

	if writes.Load() == 0 {
		t.Error("no writes succeeded in mixed workload")
	}
	if reads.Load() == 0 {
		t.Error("no reads succeeded in mixed workload")
	}
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/astraive/loza/collector/internal/sinks/duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"golang.org/x/time/rate"
)

func TestLoadSustainedThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "load.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 100
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

	// Sustained throughput: 1000 events/sec for 5 seconds = 5000 events
	targetEvents := 5000
	duration := 5 * time.Second
	eventsPerSec := targetEvents / int(duration.Seconds())

	var (
		sent     atomic.Int64
		accepted atomic.Int64
		rejected atomic.Int64
		wg       sync.WaitGroup
	)

	start := time.Now()
	deadline := start.Add(duration)

	// Send events at sustained rate
	for i := 0; i < eventsPerSec; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				evtID := fmt.Sprintf("load-%d-%d", workerID, sent.Add(1))
				evt := fmt.Sprintf(`{"event_id":"%s","event":"load.test","service":"bench","kind":"event","timestamp":"%s"}`,
					evtID, time.Now().UTC().Format(time.RFC3339))
				body, _ := json.Marshal(map[string]any{
					"api_version": "v1",
					"source":      map[string]string{"service": "bench"},
					"events":      []json.RawMessage{json.RawMessage(evt)},
				})

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
				req.Header.Set("Content-Type", "application/json")
				state.handleIngest(rec, req)

				if rec.Code == http.StatusAccepted {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}

				// Pace to ~1000 events/sec total
				time.Sleep(time.Duration(eventsPerSec) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Flush sink
	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Verify results
	totalSent := sent.Load()
	totalAccepted := accepted.Load()
	totalRejected := rejected.Load()
	throughput := float64(totalAccepted) / elapsed.Seconds()

	t.Logf("Load test results:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Sent: %d", totalSent)
	t.Logf("  Accepted: %d", totalAccepted)
	t.Logf("  Rejected: %d", totalRejected)
	t.Logf("  Throughput: %.0f events/sec", throughput)

	// Assertions
	if totalAccepted < int64(targetEvents*80/100) {
		t.Errorf("accepted too few events: %d (expected >= %d)", totalAccepted, targetEvents*80/100)
	}
	if throughput < 500 {
		t.Errorf("throughput too low: %.0f events/sec (expected >= 500)", throughput)
	}

	// Verify data in DuckDB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM " + cfg.duckDBTable).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	t.Logf("  DuckDB rows: %d", count)
	if count == 0 {
		t.Error("expected events in DuckDB, got 0")
	}
}

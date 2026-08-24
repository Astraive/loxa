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

func TestConcurrentIngestPersistsAllEvents(t *testing.T) {
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

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Inf, 0),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.diskHealthy.Store(true)
	state.spoolHealthy.Store(true)

	const (
		workerCount     = 8
		eventsPerWorker = 50
	)
	var (
		accepted atomic.Int64
		rejected atomic.Int64
		wg       sync.WaitGroup
	)

	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for eventIndex := 0; eventIndex < eventsPerWorker; eventIndex++ {
				evtID := fmt.Sprintf("load-%d-%d", workerID, eventIndex)
				evt := fmt.Sprintf(`{"event_id":"%s","event":"load.test","service":"bench","kind":"event","timestamp":"2026-01-01T00:00:00Z"}`, evtID)
				body, err := json.Marshal(map[string]any{
					"api_version": "v1",
					"source":      map[string]string{"service": "bench"},
					"events":      []json.RawMessage{json.RawMessage(evt)},
				})
				if err != nil {
					t.Errorf("marshal event: %v", err)
					return
				}

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
				req.Header.Set("Content-Type", "application/json")
				state.handleIngest(rec, req)
				if rec.Code == http.StatusAccepted {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}
			}
		}(workerID)
	}
	wg.Wait()

	if err := sink.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := int64(workerCount * eventsPerWorker)
	if got := accepted.Load(); got != want {
		t.Fatalf("accepted = %d, want %d; rejected=%d", got, want, rejected.Load())
	}
	if got := rejected.Load(); got != 0 {
		t.Fatalf("rejected = %d, want 0", got)
	}

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM " + cfg.duckDBTable).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != want {
		t.Fatalf("DuckDB rows = %d, want %d", count, want)
	}
}

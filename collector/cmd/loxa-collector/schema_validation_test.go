package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/sinks/duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"golang.org/x/time/rate"
)

func TestSchemaValidationModeOff(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema-off.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 10
	cfg.duckDBFlushInterval = 5 * time.Millisecond
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

	// Send event missing required fields (no schema_version, event_version, timestamp)
	invalidEvent := `{"event":"test","service":"checkout","event_id":"evt_1"}`
	body, _ := json.Marshal(map[string]any{
		"api_version": "v1",
		"source":      map[string]string{"service": "checkout"},
		"events":      []json.RawMessage{json.RawMessage(invalidEvent)},
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("mode=off: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSchemaValidationModeEnforce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema-enforce.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 10
	cfg.duckDBFlushInterval = 5 * time.Millisecond
	cfg.schemaMode = "enforce"

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

	// Send event with wrong schema_version
	invalidEvent := `{"schema_version":"v99","event_version":"v1","event":"test","service":"checkout","event_id":"evt_1","timestamp":"2026-01-01T00:00:00Z"}`
	body, _ := json.Marshal(map[string]any{
		"api_version": "v1",
		"source":      map[string]string{"service": "checkout"},
		"events":      []json.RawMessage{json.RawMessage(invalidEvent)},
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleIngest(rec, req)

	// Enforce mode should reject with 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("mode=enforce: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSchemaValidationModeWarn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema-warn.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 10
	cfg.duckDBFlushInterval = 5 * time.Millisecond
	cfg.schemaMode = "warn"

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

	// Send a valid event — warn mode should accept it
	validEvent := `{"schema_version":"v1","event_version":"v1","event_id":"evt-warn","event":"test","service":"checkout","kind":"event","timestamp":"2026-01-01T00:00:00Z"}`
	body, _ := json.Marshal(map[string]any{
		"api_version": "v1",
		"source":      map[string]string{"service": "checkout"},
		"events":      []json.RawMessage{json.RawMessage(validEvent)},
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleIngest(rec, req)

	// Warn mode should accept valid events (202)
	if rec.Code != http.StatusAccepted {
		t.Errorf("mode=warn: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFullPipelineIngestToQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pipeline.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 1
	cfg.duckDBFlushInterval = 5 * time.Millisecond
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

	// Send a valid event through the full pipeline
	validEvent := `{"schema_version":"v1","event_version":"v1","event_id":"pipeline-1","event":"payment.completed","service":"checkout","kind":"event","timestamp":"2026-01-01T00:00:00Z","level":"info","outcome":"success"}`
	body, _ := json.Marshal(map[string]any{
		"api_version": "v1",
		"source":      map[string]string{"service": "checkout"},
		"events":      []json.RawMessage{json.RawMessage(validEvent)},
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	// Flush sink to ensure data is written
	ctx := context.Background()
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Query back from DuckDB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM " + cfg.duckDBTable + " WHERE event_id = 'pipeline-1'").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	// Verify the event data
	var raw string
	err = db.QueryRow("SELECT "+cfg.duckDBRawColumn+" FROM "+cfg.duckDBTable+" WHERE event_id = 'pipeline-1'").Scan(&raw)
	if err != nil {
		t.Fatalf("query raw: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if parsed["event"] != "payment.completed" {
		t.Errorf("expected event=payment.completed, got %v", parsed["event"])
	}
	if parsed["service"] != "checkout" {
		t.Errorf("expected service=checkout, got %v", parsed["service"])
	}
}

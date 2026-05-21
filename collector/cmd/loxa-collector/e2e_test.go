package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/ingest"
	"github.com/astraive/loxa-collector/internal/sinks/duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"golang.org/x/time/rate"
)

func TestCollectorE2ERawPreservation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "collector-e2e.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()

	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.apiKey = "k1"
	cfg.duckDBPath = dbPath
	cfg.duckDBBatchSize = 10
	cfg.duckDBFlushInterval = 5 * time.Millisecond

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
		t.Fatalf("new sink: %v", err)
	}
	defer func() { _ = sink.Close(context.Background()) }()

	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	srv := httptest.NewServer(buildMux(state))
	defer srv.Close()

	body := `[{"event_id":"evt-1","event":"checkout.request","service":"checkout","http":{"status":200},"duration_ms":12.5,"timestamp":"2026-05-11T00:00:00Z"},123]`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest", strings.NewReader(body))
	req.Header.Set(state.cfg.apiKeyHeader, "k1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ingest request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMultiStatus {
		t.Fatalf("expected 207, got %d", res.StatusCode)
	}

	var out ingest.Response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Accepted != 1 || out.Invalid != 1 || out.Rejected != 0 {
		t.Fatalf("unexpected response: %+v", out)
	}

	_ = out

	if err := sink.Flush(context.Background()); err != nil {
		t.Fatalf("sink flush: %v", err)
	}

	var eventID, eventType, service, raw string
	var statusCode int
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := db.QueryRow(`SELECT event_id, event_name, service, status_code, raw FROM events LIMIT 1`).
			Scan(&eventID, &eventType, &service, &statusCode, &raw)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "no rows") || time.Now().After(deadline) {
			t.Fatalf("query row: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if eventID != "evt-1" || eventType != "checkout.request" || service != "checkout" || statusCode != 200 {
		t.Fatalf("unexpected projected row: event_id=%q event_type=%q service=%q status=%d", eventID, eventType, service, statusCode)
	}

	wantRaw := `{"event_id":"evt-1","event":"checkout.request","service":"checkout","http":{"status":200},"duration_ms":12.5,"timestamp":"2026-05-11T00:00:00Z"}`
	if !equivalentJSON(raw, wantRaw) {
		t.Fatalf("raw mismatch:\nwant: %s\ngot:  %s", wantRaw, raw)
	}
}

func TestCollectorE2EEndpointsAndGzip(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	srv := httptest.NewServer(buildMux(state))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz status: %d", res.StatusCode)
	}

	res, err = http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz request: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("readyz status: %d", res.StatusCode)
	}

	gzPayload := mustGzip(t, []byte(`{"event":"gzip.event"}`))
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest", bytes.NewReader(gzPayload))
	req.Header.Set("Content-Encoding", "gzip")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("gzip ingest request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.StatusCode)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one event written, got %d", len(sink.events))
	}

	res, err = http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("metrics status: %d", res.StatusCode)
	}
}

func mustGzip(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func equivalentJSON(left, right string) bool {
	var lhs any
	if err := json.Unmarshal([]byte(left), &lhs); err != nil {
		return false
	}
	var rhs any
	if err := json.Unmarshal([]byte(right), &rhs); err != nil {
		return false
	}
	return strings.TrimSpace(left) == strings.TrimSpace(right) || deepEqualJSON(lhs, rhs)
}

func deepEqualJSON(left, right any) bool {
	switch lhs := left.(type) {
	case map[string]any:
		rhs, ok := right.(map[string]any)
		if !ok || len(lhs) != len(rhs) {
			return false
		}
		for key, value := range lhs {
			if !deepEqualJSON(value, rhs[key]) {
				return false
			}
		}
		return true
	case []any:
		rhs, ok := right.([]any)
		if !ok || len(lhs) != len(rhs) {
			return false
		}
		for i := range lhs {
			if !deepEqualJSON(lhs[i], rhs[i]) {
				return false
			}
		}
		return true
	default:
		return left == right
	}
}

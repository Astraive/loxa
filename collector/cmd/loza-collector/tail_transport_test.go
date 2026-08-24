package main

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	serverruntime "github.com/astraive/loza/collector/internal/server"
	duckdbsink "github.com/astraive/loza/collector/internal/sinks/duckdb"
	publichttp "github.com/astraive/loza/collector/server/http"
)

func TestParseTailFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/tail?since=2026-01-01T00:00:00Z&after_event_id=evt-1&service=checkout&kind=log&trace_id=tr-1&incident_id=inc-1&limit=25", nil)
	filters, err := serverruntime.ParseTailFilters(req)
	if err != nil {
		t.Fatalf("parse filters: %v", err)
	}
	if !filters.Since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected since filter: %+v", filters)
	}
	if filters.AfterEventID != "evt-1" || filters.Service != "checkout" || filters.Kind != "log" || filters.TraceID != "tr-1" || filters.IncidentID != "inc-1" || filters.Limit != 25 {
		t.Fatalf("unexpected filters: %+v", filters)
	}
}

func TestRawMatchesTailFilters(t *testing.T) {
	raw := []byte(`{"id":"evt-1","service":"checkout","kind":"log","trace_id":"tr-1","incident_id":"inc-1"}`)
	if !rawMatchesTailFilters(raw, serverruntime.TailFilters{Service: "checkout", Kind: "log", TraceID: "tr-1", IncidentID: "inc-1"}) {
		t.Fatal("expected raw payload to match filters")
	}
	if rawMatchesTailFilters(raw, serverruntime.TailFilters{Service: "billing"}) {
		t.Fatal("expected service mismatch to fail")
	}
}

func TestTailMatchesAuthorizedCollectorScope(t *testing.T) {
	state := &collectorState{}
	ctx := publichttp.WithAuthorizedCollector(context.Background(), "orders", "production")
	matching := []byte(`{"collector":"orders","environment":"production","service":"checkout"}`)
	crossScope := []byte(`{"collector":"payments","environment":"production","service":"checkout"}`)

	if !state.TailMatches(ctx, matching, serverruntime.TailFilters{Service: "checkout"}) {
		t.Fatal("expected event in the authorized scope to match")
	}
	if state.TailMatches(ctx, crossScope, serverruntime.TailFilters{Service: "checkout"}) {
		t.Fatal("cross-collector event must not match an authorized stream")
	}
}

func TestQueryTailHistoryDecryptsBeforeFiltering(t *testing.T) {
	db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "tail.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (raw VARCHAR, timestamp TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}

	key := "tail-history-encryption-key"
	local := []byte(`{"event_id":"evt-local","timestamp":"2026-08-24T04:00:00Z","collector":"local","service":"checkout"}`)
	other := []byte(`{"event_id":"evt-other","timestamp":"2026-08-24T04:00:01Z","collector":"other","service":"checkout"}`)
	sink, err := duckdbsink.New(duckdbsink.Config{
		DB:         db,
		Table:      "events",
		Schema:     map[string]string{"timestamp": "timestamp"},
		StoreRaw:   true,
		RawColumn:  "raw",
		EncryptRaw: true,
		EncryptKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close(context.Background())
	for _, raw := range [][]byte{local, other} {
		if err := sink.WriteEvent(context.Background(), raw, nil); err != nil {
			t.Fatal(err)
		}
	}

	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable:          "events",
			duckDBRawColumn:      "raw",
			duckDBSchema:         map[string]string{"timestamp": "timestamp"},
			storageEncryptionKey: key,
		},
		queryDB: db,
	}
	rows, err := state.queryTailHistory(context.Background(), serverruntime.TailFilters{
		Collector: "local",
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || string(rows[0]) != string(local) {
		t.Fatalf("encrypted tail history = %q, want %q", rows, local)
	}
}

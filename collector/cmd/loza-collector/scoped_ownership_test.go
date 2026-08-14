package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	publichttp "github.com/astraive/loza/collector/server/http"
	_ "github.com/marcboeker/go-duckdb"
)

func scopedRequest(method, path, collector, environment, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(publichttp.WithAuthorizedCollector(req.Context(), collector, environment))
	req.SetPathValue("collector", collector)
	return req
}

func TestScopedIngestStampsAuthorizedOwnershipAndRejectsForgery(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.rateLimitEnabled = false
	sink := &fakeSink{}
	state := &collectorState{
		cfg:        cfg,
		ingestSink: sink,
	}

	accepted := scopedRequest(http.MethodPost, "/collectors/collector-a/events", "collector-a", "production", `{"event":"accepted"}`)
	accepted.Header.Set("Content-Type", "application/json")
	acceptedRecorder := httptest.NewRecorder()
	state.handleIngest(acceptedRecorder, accepted)
	if acceptedRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected scoped event to be accepted, got %d: %s", acceptedRecorder.Code, acceptedRecorder.Body.String())
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one stored event, got %d", len(sink.events))
	}
	var stored map[string]any
	if err := json.Unmarshal(sink.events[0], &stored); err != nil {
		t.Fatalf("decode stored event: %v", err)
	}
	if stored[collectorOwnershipColumn] != "collector-a" || stored[environmentOwnershipColumn] != "production" {
		t.Fatalf("stored event ownership = %#v, want collector-a/production", stored)
	}

	forged := scopedRequest(http.MethodPost, "/collectors/collector-a/events", "collector-a", "production", `{"event":"forged","collector":"collector-b","environment":"production"}`)
	forged.Header.Set("Content-Type", "application/json")
	forgedRecorder := httptest.NewRecorder()
	state.handleIngest(forgedRecorder, forged)
	if forgedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected forged ownership to be rejected, got %d: %s", forgedRecorder.Code, forgedRecorder.Body.String())
	}
	if len(sink.events) != 1 {
		t.Fatalf("forged event was stored; got %d stored events", len(sink.events))
	}
}

func TestScopedDeletionDoesNotRemoveAnotherCollectorsEvent(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (
		event_id VARCHAR,
		tenant_id VARCHAR,
		user_id VARCHAR,
		collector VARCHAR,
		environment VARCHAR,
		raw VARCHAR
	)`); err != nil {
		t.Fatalf("create events table: %v", err)
	}
	for _, collector := range []string{"collector-a", "collector-b"} {
		if _, err := db.Exec(`INSERT INTO events (event_id, collector, environment) VALUES (?, ?, ?)`, "shared-event", collector, "production"); err != nil {
			t.Fatalf("insert %s event: %v", collector, err)
		}
	}
	state := &collectorState{
		cfg: collectorConfig{
			duckDBTable: "events",
		},
		queryDB: db,
	}

	req := scopedRequest(http.MethodDelete, "/collectors/collector-a/events/shared-event", "collector-a", "production", "")
	rec := httptest.NewRecorder()
	state.handleDeleteEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected scoped deletion to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
	var remainingA, remainingB int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE collector = 'collector-a'`).Scan(&remainingA); err != nil {
		t.Fatalf("count collector-a rows: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE collector = 'collector-b'`).Scan(&remainingB); err != nil {
		t.Fatalf("count collector-b rows: %v", err)
	}
	if remainingA != 0 || remainingB != 1 {
		t.Fatalf("remaining rows collector-a=%d collector-b=%d, want 0 and 1", remainingA, remainingB)
	}
}

func TestScopedRawSQLQueryIsRejected(t *testing.T) {
	state := &collectorState{cfg: testCollectorConfig()}
	req := scopedRequest(http.MethodPost, "/collectors/collector-a/query", "collector-a", "production", `{"sql":"SELECT * FROM events"}`)
	rec := httptest.NewRecorder()

	state.handleQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected scoped raw SQL to be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["error"] != "scoped_raw_sql_unsupported" {
		t.Fatalf("error = %#v, want scoped_raw_sql_unsupported", response["error"])
	}
}

func TestScopedQueryReadsOnlyAuthorizedOwnership(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (event_id VARCHAR, collector VARCHAR, environment VARCHAR)`); err != nil {
		t.Fatalf("create events table: %v", err)
	}
	for _, row := range [][3]string{{"a-1", "collector-a", "production"}, {"b-1", "collector-b", "production"}} {
		if _, err := db.Exec(`INSERT INTO events VALUES (?, ?, ?)`, row[0], row[1], row[2]); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}
	state := &collectorState{cfg: collectorConfig{duckDBTable: "events"}, queryDB: db}
	rec := httptest.NewRecorder()
	state.handleQuery(rec, scopedRequest(http.MethodPost, "/collectors/collector-a/query", "collector-a", "production", `{"limit":5}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected scoped read to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "b-1") || !strings.Contains(rec.Body.String(), "a-1") {
		t.Fatalf("scoped query leaked or omitted rows: %s", rec.Body.String())
	}
}

func TestEnsureSchemaMigratesOwnershipProjectionColumns(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (raw VARCHAR)`); err != nil {
		t.Fatalf("create legacy events table: %v", err)
	}

	if err := ensureSchema(db, testCollectorConfig()); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	for _, column := range []string{collectorOwnershipColumn, environmentOwnershipColumn} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'events' AND column_name = ?`, column).Scan(&count); err != nil {
			t.Fatalf("check %s column: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("ownership column %q missing after migration", column)
		}
	}
}

func TestScopedTailAndReplayAreRejected(t *testing.T) {
	state := &collectorState{cfg: testCollectorConfig()}
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		handle func(http.ResponseWriter, *http.Request)
	}{
		{name: "tail", method: http.MethodGet, path: "/collectors/collector-a/tail", handle: state.handleTail},
		{name: "replay", method: http.MethodPost, path: "/collectors/collector-a/replay", body: `{"events":[]}`, handle: state.handleReplay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handle(rec, scopedRequest(tc.method, tc.path, "collector-a", "production", tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected scoped operation to be rejected, got %d: %s", rec.Code, rec.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response["error"] != "scoped_operation_unsupported" {
				t.Fatalf("error = %#v, want scoped_operation_unsupported", response["error"])
			}
		})
	}
}

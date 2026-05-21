package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestNewRequiresDSNWhenDBMissing(t *testing.T) {
	_, err := New(Config{Table: "events"})
	if err == nil || !strings.Contains(err.Error(), "DSN is required") {
		t.Fatalf("expected DSN required error, got: %v", err)
	}
}

func TestQuoteTableNameRejectsInvalidIdentifier(t *testing.T) {
	_, err := quoteTableName("events;drop")
	if err == nil {
		t.Fatalf("expected identifier error")
	}
}

func TestBuildSchemaInsertQuery(t *testing.T) {
	q := buildSchemaInsertQuery(`"events"`, []string{"event_id", "status_code"}, true, `"raw"`)
	want := `INSERT INTO "events" ("event_id", "status_code", "raw") VALUES (?, ?, ?)`
	if q != want {
		t.Fatalf("unexpected query:\nwant: %s\ngot:  %s", want, q)
	}
}

func TestNewRejectsInvalidSchemaPath(t *testing.T) {
	_, err := New(Config{
		Table: "events",
		Schema: map[string]string{
			"status_code": "http..status",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid schema path") {
		t.Fatalf("expected invalid schema path error, got: %v", err)
	}
}

func TestNewRejectsRawColumnConflict(t *testing.T) {
	_, err := New(Config{
		Table:     "events",
		RawColumn: "raw",
		StoreRaw:  true,
		Schema: map[string]string{
			"raw": "event_id",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts with schema column") {
		t.Fatalf("expected raw-column conflict error, got: %v", err)
	}
}

func TestNewRejectsInvalidRawColumnIdentifier(t *testing.T) {
	_, err := New(Config{
		Table:     "events",
		RawColumn: "raw-column",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid raw column") {
		t.Fatalf("expected invalid raw column error, got: %v", err)
	}
}

func TestWriteEventRawModeE2E(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:    db,
		Table: "events",
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	encoded := []byte(`{"event_id":"evt-raw","status":"ok"}`)
	if err := s.WriteEvent(ctx, encoded, nil); err != nil {
		t.Fatalf("write event failed: %v", err)
	}

	var got string
	if err := db.QueryRowContext(ctx, `SELECT raw FROM events`).Scan(&got); err != nil {
		t.Fatalf("query raw failed: %v", err)
	}
	if got != string(encoded) {
		t.Fatalf("unexpected raw payload:\nwant: %s\ngot:  %s", string(encoded), got)
	}
}

func TestWriteEventSchemaProjectionE2E(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (event_id TEXT, status_code TEXT)`)

	s, err := New(Config{
		DB:    db,
		Table: "events",
		Schema: map[string]string{
			"status_code": "http.status",
			"event_id":    "event_id",
		},
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	encoded := []byte(`{"event_id":"evt-schema","http":{"status":"200"}}`)
	if err := s.WriteEvent(ctx, encoded, nil); err != nil {
		t.Fatalf("write event failed: %v", err)
	}

	var eventID, statusCode string
	if err := db.QueryRowContext(ctx, `SELECT event_id, status_code FROM events`).Scan(&eventID, &statusCode); err != nil {
		t.Fatalf("query projected row failed: %v", err)
	}
	if eventID != "evt-schema" || statusCode != "200" {
		t.Fatalf("unexpected projected values: event_id=%q status_code=%q", eventID, statusCode)
	}
}

func TestWriteEventSchemaProjectionWithStoreRawE2E(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (event_id TEXT, status_code TEXT, raw TEXT)`)

	s, err := New(Config{
		DB:        db,
		Table:     "events",
		StoreRaw:  true,
		RawColumn: "raw",
		Schema: map[string]string{
			"status_code": "http.status",
			"event_id":    "event_id",
		},
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	encoded := []byte(`{"event_id":"evt-schema-raw","http":{"status":"201"}}`)
	if err := s.WriteEvent(ctx, encoded, nil); err != nil {
		t.Fatalf("write event failed: %v", err)
	}

	var eventID, statusCode, raw string
	if err := db.QueryRowContext(ctx, `SELECT event_id, status_code, raw FROM events`).Scan(&eventID, &statusCode, &raw); err != nil {
		t.Fatalf("query projected row failed: %v", err)
	}
	if eventID != "evt-schema-raw" || statusCode != "201" {
		t.Fatalf("unexpected projected values: event_id=%q status_code=%q", eventID, statusCode)
	}
	if raw != string(encoded) {
		t.Fatalf("unexpected raw payload:\nwant: %s\ngot:  %s", string(encoded), raw)
	}
}

func TestWriteEventBatchingFlushOnBatchSize(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:        db,
		Table:     "events",
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	if err := s.WriteEvent(ctx, []byte(`{"event_id":"a"}`), nil); err != nil {
		t.Fatalf("write event a failed: %v", err)
	}
	assertRowCount(t, db, 0)

	if err := s.WriteEvent(ctx, []byte(`{"event_id":"b"}`), nil); err != nil {
		t.Fatalf("write event b failed: %v", err)
	}
	assertRowCount(t, db, 2)
}

func TestWriteEventBatchingFlush(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:        db,
		Table:     "events",
		BatchSize: 10,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	if err := s.WriteEvent(ctx, []byte(`{"event_id":"a"}`), nil); err != nil {
		t.Fatalf("write event a failed: %v", err)
	}
	assertRowCount(t, db, 0)

	if err := s.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	assertRowCount(t, db, 1)
}

func TestWriteEventBatchingFlushInterval(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:            db,
		Table:         "events",
		BatchSize:     100,
		FlushInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	if err := s.WriteEvent(ctx, []byte(`{"event_id":"a"}`), nil); err != nil {
		t.Fatalf("write event a failed: %v", err)
	}
	assertRowCount(t, db, 0)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rowCount(t, db) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected periodic flush to persist row")
}

func TestWriteEventWriterLoopBatching(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:              db,
		Table:           "events",
		BatchSize:       3,
		FlushInterval:   50 * time.Millisecond,
		WriterLoop:      true,
		WriterQueueSize: 16,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	if err := s.WriteEvent(ctx, []byte(`{"event":"a"}`), nil); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := s.WriteEvent(ctx, []byte(`{"event":"b"}`), nil); err != nil {
		t.Fatalf("write b: %v", err)
	}
	assertRowCount(t, db, 0)

	if err := s.WriteEvent(ctx, []byte(`{"event":"c"}`), nil); err != nil {
		t.Fatalf("write c: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rowCount(t, db) == 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected writer-loop batch flush")
}

func TestWriterLoopCloseDrainsQueuedEvents(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:              db,
		Table:           "events",
		BatchSize:       100,
		WriterLoop:      true,
		WriterQueueSize: 128,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if err := s.WriteEvent(ctx, []byte(fmt.Sprintf(`{"event":"%d"}`, i)), nil); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	if err := s.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	assertRowCount(t, db, 25)
}

func TestWriterLoopConcurrentStress(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	s, err := New(Config{
		DB:              db,
		Table:           "events",
		BatchSize:       256,
		FlushInterval:   20 * time.Millisecond,
		WriterLoop:      true,
		WriterQueueSize: 4096,
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}

	const workers = 16
	const perWorker = 500
	ctx := context.Background()
	errCh := make(chan error, workers*perWorker)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				payload := []byte(fmt.Sprintf(`{"worker":%d,"seq":%d}`, w, i))
				if err := s.WriteEvent(ctx, payload, nil); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("stress write failed: %v", err)
		}
	}

	if err := s.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	assertRowCount(t, db, workers*perWorker)
}

func TestP0DuckDBWriterLoopFailureSurfaced(t *testing.T) {
	db := openSQLiteDB(t)
	mustExec(t, db, `CREATE TABLE events (raw TEXT)`)

	rawSink, err := New(Config{
		DB:              db,
		Table:           "events",
		WriterLoop:      true,
		WriterQueueSize: 1,
		BatchSize:       2,
		Schema: map[string]string{
			"event_id": "event_id",
		},
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}
	s := rawSink.(*sink)
	defer func() { _ = s.Close(context.Background()) }()

	if err := s.WriteEvent(context.Background(), []byte(`{"event_id":"evt-1"}`), nil); err != nil {
		t.Fatalf("first write should enqueue before worker error, got %v", err)
	}
	if err := s.WriteEvent(context.Background(), []byte(`{"event_id":"evt-2"}`), nil); err != nil {
		t.Fatalf("second write should enqueue before worker error, got %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var workerErr error
	for time.Now().Before(deadline) {
		s.mu.Lock()
		workerErr = s.lastErr
		s.mu.Unlock()
		if workerErr != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if workerErr == nil {
		t.Fatalf("expected writer loop failure")
	}

	err = s.WriteEvent(context.Background(), []byte(`{"event_id":"evt-3"}`), nil)
	if err == nil {
		t.Fatalf("expected write to surface writer-loop failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "event_id") {
		t.Fatalf("expected event_id-related error, got %v", err)
	}
}

func openSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q failed: %v", query, err)
	}
}

func rowCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("query row count failed: %v", err)
	}
	return count
}

func assertRowCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	if got := rowCount(t, db); got != want {
		t.Fatalf("row count mismatch: want=%d got=%d", want, got)
	}
}

func TestIsRetryableError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: false},
		{err: errors.New("database is locked"), want: true},
		{err: errors.New("BUSY"), want: true},
		{err: errors.New("temporary I/O timeout"), want: true},
		{err: errors.New("syntax error near SELECT"), want: false},
	}
	for _, tc := range cases {
		got := isRetryableError(tc.err)
		if got != tc.want {
			t.Fatalf("isRetryableError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestContextWithWriteTimeout(t *testing.T) {
	s := &sink{writeTimeout: 50 * time.Millisecond}
	ctx, cancel := s.contextWithWriteTimeout(context.Background())
	defer cancel()

	if _, ok := ctx.Deadline(); !ok {
		t.Fatalf("expected deadline to be applied")
	}
}

func TestContextWithWriteTimeoutKeepsExistingDeadline(t *testing.T) {
	base, baseCancel := context.WithTimeout(context.Background(), time.Second)
	defer baseCancel()

	s := &sink{writeTimeout: 10 * time.Millisecond}
	ctx, cancel := s.contextWithWriteTimeout(base)
	defer cancel()

	d1, ok1 := base.Deadline()
	d2, ok2 := ctx.Deadline()
	if !ok1 || !ok2 {
		t.Fatalf("expected both contexts to have deadlines")
	}
	if !d1.Equal(d2) {
		t.Fatalf("expected existing deadline to be preserved")
	}
}

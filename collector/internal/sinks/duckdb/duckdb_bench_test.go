package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

var benchCounter int64

func generateEvents(n int) [][]byte {
	events := make([][]byte, n)
	for i := 0; i < n; i++ {
		benchCounter++
		ev := map[string]any{
			"event_id":   fmt.Sprintf("evt_bench_%d", benchCounter),
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
			"event_name": "benchmark.event",
			"level":      "info",
			"service":    "bench-service",
			"outcome":    "success",
			"raw":        fmt.Sprintf(`{"event":{"name":"bench_%d","timestamp":"%s"}}`, benchCounter, time.Now().UTC().Format(time.RFC3339Nano)),
		}
		events[i], _ = json.Marshal(ev)
	}
	return events
}

// BenchmarkDuckDBInsert measures single-event INSERT performance.
func BenchmarkDuckDBInsert(b *testing.B) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		event_id TEXT PRIMARY KEY,
		timestamp TIMESTAMP,
		event_name TEXT,
		level TEXT,
		service TEXT,
		outcome TEXT,
		raw TEXT
	)`)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchCounter++
		ev := map[string]any{
			"event_id":   fmt.Sprintf("evt_bench_%d", benchCounter),
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
			"event_name": "benchmark.event",
			"level":      "info",
			"service":    "bench-service",
			"outcome":    "success",
			"raw":        fmt.Sprintf(`{"event":{"name":"bench_%d"}}`, benchCounter),
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO events (event_id, timestamp, event_name, level, service, outcome, raw) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ev["event_id"], ev["timestamp"], ev["event_name"], ev["level"], ev["service"], ev["outcome"], ev["raw"])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDuckDBBatchInsert measures batch INSERT in a transaction.
func BenchmarkDuckDBBatchInsert(b *testing.B) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		event_id TEXT PRIMARY KEY,
		timestamp TIMESTAMP,
		event_name TEXT,
		level TEXT,
		service TEXT,
		outcome TEXT,
		raw TEXT
	)`)
	if err != nil {
		b.Fatal(err)
	}

	events := generateEvents(1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTx(ctx, nil)
		stmt, _ := tx.PrepareContext(ctx,
			`INSERT INTO events (event_id, timestamp, event_name, level, service, outcome, raw) VALUES (?, ?, ?, ?, ?, ?, ?)`)
		for _, ev := range events {
			var parsed map[string]any
			_ = json.Unmarshal(ev, &parsed)
			_, _ = stmt.ExecContext(ctx, parsed["event_id"], parsed["timestamp"], parsed["event_name"], parsed["level"], parsed["service"], parsed["outcome"], parsed["raw"])
		}
		_ = stmt.Close()
		_ = tx.Commit()
	}
}

// BenchmarkDuckDBPointLookup measures SELECT by event_id.
func BenchmarkDuckDBPointLookup(b *testing.B) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		event_id TEXT PRIMARY KEY,
		timestamp TIMESTAMP,
		event_name TEXT,
		level TEXT,
		service TEXT,
		outcome TEXT,
		raw TEXT
	)`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert test data
	events := generateEvents(10000)
	ctx := context.Background()
	tx, _ := db.BeginTx(ctx, nil)
	stmt, _ := tx.PrepareContext(ctx,
		`INSERT INTO events (event_id, timestamp, event_name, level, service, outcome, raw) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	eventIDs := make([]string, len(events))
	for i, ev := range events {
		var parsed map[string]any
		_ = json.Unmarshal(ev, &parsed)
		eventIDs[i] = parsed["event_id"].(string)
		_, _ = stmt.ExecContext(ctx, parsed["event_id"], parsed["timestamp"], parsed["event_name"], parsed["level"], parsed["service"], parsed["outcome"], parsed["raw"])
	}
	_ = stmt.Close()
	_ = tx.Commit()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var eventID, ts, name, level, svc, outcome, raw string
		err := db.QueryRowContext(ctx, "SELECT event_id, timestamp, event_name, level, service, outcome, raw FROM events WHERE event_id = ?",
			eventIDs[i%len(eventIDs)]).Scan(&eventID, &ts, &name, &level, &svc, &outcome, &raw)
		if err != nil {
			b.Fatal(err)
		}
	}
}

package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

func BenchmarkDuckDBSink(b *testing.B) {
	dbPath := "bench_duckdb.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	payloads := map[string][]byte{
		"small":  []byte(`{"event":"test","val":123}`),
		"medium": []byte(`{"event":"checkout.request","service":"checkout","user":{"id":"u-123","plan":"pro"},"http":{"method":"POST","path":"/checkout","status":200},"duration_ms":42,"outcome":"success"}`),
	}
	type scenario struct {
		name string
		cfg  Config
	}
	scenarios := []scenario{
		{name: "direct", cfg: Config{}},
		{name: "batch-32", cfg: Config{BatchSize: 32}},
		{name: "batch-128", cfg: Config{BatchSize: 128}},
		{name: "writer-loop-128", cfg: Config{BatchSize: 128, WriterLoop: true, FlushInterval: 25 * time.Millisecond}},
		{name: "appender-128", cfg: Config{BatchSize: 128, WriterLoop: true, UseAppender: true, FlushInterval: 25 * time.Millisecond}},
	}

	for payloadName, payload := range payloads {
		for _, sc := range scenarios {
			sc := sc
			b.Run(fmt.Sprintf("%s/%s", sc.name, payloadName), func(b *testing.B) {
				table := fmt.Sprintf("events_%s_%s", sc.name, payloadName)
				table = sanitizeTableName(table)
				setupTable(b, db, table)

				cfg := sc.cfg
				cfg.DB = db
				cfg.Table = table
				sink, err := New(cfg)
				if err != nil {
					b.Fatal(err)
				}
				ctx := context.Background()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := sink.WriteEvent(ctx, payload, nil); err != nil {
						b.Fatal(err)
					}
				}
				if err := sink.Flush(ctx); err != nil {
					b.Fatal(err)
				}
			})
		}
	}
}

func setupTable(b *testing.B, db *sql.DB, table string) {
	_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
	if err != nil {
		b.Fatal(err)
	}
	_, err = db.Exec(fmt.Sprintf("CREATE TABLE %s (raw TEXT)", table))
	if err != nil {
		b.Fatal(err)
	}
}

func sanitizeTableName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

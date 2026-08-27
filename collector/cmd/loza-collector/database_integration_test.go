//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/collector/internal/database"
	_ "github.com/marcboeker/go-duckdb"
)

type databaseExecer interface {
	Exec(context.Context, string, ...any) error
}

func TestNamedDatabaseConnectionsRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	duckPath := t.TempDir() + "/listener.duckdb"
	duck := databaseConnectionConfig{
		name: "duckdb", backend: "duckdb", enabled: true, path: duckPath,
		driver: "duckdb", table: "events", rawColumn: "raw",
		connectionTimeout: 5 * time.Second, queryTimeout: 10 * time.Second,
	}
	duckConn, err := openNamedDuckDB(ctx, collectorConfig{duckDBMaxOpenConns: 1, duckDBMaxIdleConns: 1}, duck)
	if err != nil {
		t.Fatalf("open DuckDB named connection: %v", err)
	}
	t.Cleanup(func() { _ = duckConn.Close(context.Background()) })
	runDatabaseRoundTrip(t, ctx, duckConn, "listener_duckdb", "?")

	pgUser, pgPassword := os.Getenv("LOZA_TEST_POSTGRES_USER"), os.Getenv("LOZA_TEST_POSTGRES_PASSWORD")
	if pgUser == "" || pgPassword == "" {
		t.Skip("set LOZA_TEST_POSTGRES_USER and LOZA_TEST_POSTGRES_PASSWORD to run PostgreSQL integration")
	}
	pg := databaseConnectionConfig{
		name: "postgres", backend: "postgres", enabled: true,
		host:     envOr("LOZA_TEST_POSTGRES_HOST", "127.0.0.1"),
		port:     envIntOr("LOZA_TEST_POSTGRES_PORT", 5432),
		database: envOr("LOZA_TEST_POSTGRES_DATABASE", "loza"),
		username: pgUser, password: pgPassword, sslMode: envOr("LOZA_TEST_POSTGRES_SSLMODE", "disable"),
		table: "events", rawColumn: "raw", connectionTimeout: 5 * time.Second,
		queryTimeout: 10 * time.Second,
	}
	pgConn, err := openNamedPostgres(ctx, collectorConfig{}, pg)
	if err != nil {
		t.Fatalf("open PostgreSQL named connection: %v", err)
	}
	t.Cleanup(func() { _ = pgConn.Close(context.Background()) })
	runDatabaseRoundTrip(t, ctx, pgConn, "listener_postgres", "$1")

	chUser, chPassword := os.Getenv("LOZA_TEST_CLICKHOUSE_USER"), os.Getenv("LOZA_TEST_CLICKHOUSE_PASSWORD")
	if chUser == "" {
		chUser = "default"
	}
	ch := databaseConnectionConfig{
		name: "clickhouse", backend: "clickhouse", enabled: true,
		hosts:    strings.Split(envOr("LOZA_TEST_CLICKHOUSE_HOSTS", "127.0.0.1:9000"), ","),
		database: envOr("LOZA_TEST_CLICKHOUSE_DATABASE", "loza"),
		username: chUser, password: chPassword, table: "events", rawColumn: "raw",
		connectionTimeout: 5 * time.Second, queryTimeout: 10 * time.Second,
	}
	chConn, err := openNamedClickHouse(ctx, collectorConfig{}, ch)
	if err != nil {
		t.Fatalf("open ClickHouse named connection: %v", err)
	}
	t.Cleanup(func() { _ = chConn.Close(context.Background()) })
	runDatabaseRoundTrip(t, ctx, chConn, "listener_clickhouse", "")
}

func runDatabaseRoundTrip(t *testing.T, ctx context.Context, conn database.Connection, table, placeholder string) {
	t.Helper()
	execer, ok := conn.(databaseExecer)
	if !ok {
		t.Fatalf("%s connection does not support writes", conn.Backend())
	}
	if err := execer.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		t.Fatalf("%s cleanup: %v", conn.Backend(), err)
	}
	if err := execer.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (value VARCHAR NOT NULL)%s", table, databaseEngine(conn.Backend()))); err != nil {
		t.Fatalf("%s schema: %v", conn.Backend(), err)
	}
	if err := execer.Exec(ctx, fmt.Sprintf("INSERT INTO %s (value) VALUES ('listener-event')", table)); err != nil {
		t.Fatalf("%s write: %v", conn.Backend(), err)
	}
	query := fmt.Sprintf("SELECT value FROM %s", table)
	if placeholder != "" {
		query = fmt.Sprintf("SELECT value FROM %s WHERE value = %s", table, placeholder)
	}
	result, err := conn.Query(ctx, query, "listener-event")
	if err != nil {
		t.Fatalf("%s read: %v", conn.Backend(), err)
	}
	if len(result.Rows) != 1 || len(result.Rows[0]) != 1 || result.Rows[0][0] != "listener-event" {
		t.Fatalf("%s normalized result = %#v", conn.Backend(), result)
	}
	if err := conn.Ping(ctx); err != nil {
		t.Fatalf("%s test: %v", conn.Backend(), err)
	}
	if err := execer.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		t.Fatalf("%s final cleanup: %v", conn.Backend(), err)
	}
}
func databaseEngine(backend string) string {
	if backend == "clickhouse" {
		return " ENGINE = Memory"
	}
	return ""
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envIntOr(name string, fallback int) int {
	var value int
	if _, err := fmt.Sscanf(os.Getenv(name), "%d", &value); err == nil && value > 0 {
		return value
	}
	return fallback
}
